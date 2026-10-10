package controllers

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"oneimg/backend/database"
	"oneimg/backend/interfaces"
	"oneimg/backend/models"
	"oneimg/backend/services"
	"oneimg/backend/utils/images"
	"oneimg/backend/utils/md5"
	"oneimg/backend/utils/safefetch"
	"oneimg/backend/utils/telegram"
	"oneimg/backend/utils/uploads"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// uploadImagesLegacy keeps the original request-time, single-bucket upload
// path used when multi-storage synchronization is disabled.
func uploadImagesLegacy(c *gin.Context, setting models.Settings, existingTags []models.Tags, folderID int) {
	uc := uploads.NewUploadContext(c)
	db := database.GetDB()

	bucketID, err := resolveLegacyRequestedBucketID(c, setting, c.PostForm("bucket_id"))
	if err != nil {
		uc.Fail(http.StatusBadRequest, "%v", err)
		return
	}

	allowed, err := canUseLegacyUploadBucket(c, setting, bucketID)
	if err != nil {
		uc.Fail(http.StatusInternalServerError, "校验存储权限失败：%v", err)
		return
	}
	if !allowed {
		uc.Fail(http.StatusForbidden, "无权使用该存储源")
		return
	}

	var bucket models.Buckets
	if err := db.DB.First(&bucket, bucketID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			uc.Fail(http.StatusBadRequest, "存储配置不存在")
			return
		}
		uc.Fail(http.StatusInternalServerError, "存储配置查询失败：%v", err)
		return
	}

	files, err := parseConfiguredUploadFiles(c, setting)
	if err != nil {
		uc.Fail(uploadParseErrorStatus(err), "文件解析失败：%v", err)
		return
	}

	if bucket.Type != "default" && bucket.Type != "telegram" && bucket.Capacity > 0 {
		var uploadSize uint64
		for _, file := range files {
			uploadSize += uint64(file.Size)
		}
		if !bucket.CanStore(uploadSize) {
			uc.Fail(http.StatusBadRequest, "存储空间已满, 请切换存储")
			return
		}
	}

	uploader, err := uc.GetStorageUploader(&setting, &bucket)
	if err != nil {
		uc.Fail(http.StatusBadRequest, "%s", err.Error())
		return
	}

	results := make([]interfaces.ImageUploadResult, 0, len(files))
	for _, file := range files {
		fileResult, uploadErr := uploader.Upload(c, &setting, &bucket, file)
		if uploadErr != nil {
			results = append(results, failedUploadResult(file, "文件上传失败"))
			continue
		}

		imageModel := models.Image{
			Url:       fileResult.URL,
			Thumbnail: fileResult.ThumbnailURL,
			FileName:  fileResult.FileName,
			FileSize:  fileResult.FileSize,
			MimeType:  fileResult.MimeType,
			Width:     fileResult.Width,
			Height:    fileResult.Height,
			Storage:   fileResult.Storage,
			BucketId:  bucketID,
			FolderId:  folderID,
			UserId:    c.GetInt("user_id"),
			MD5:       md5.Md5(c.GetString("username") + fileResult.FileName),
			UUID:      CurrentOwnerKey(c),
		}

		now := time.Now()
		if err := db.DB.Transaction(func(tx *gorm.DB) error {
			resolved, err := services.ResolvePublicationFolder(tx, imageModel.UserId, imageModel.FolderId)
			if err != nil {
				return err
			}
			imageModel.FolderId = resolved
			if err := tx.Create(&imageModel).Error; err != nil {
				return err
			}
			storageStatus := models.ImageStorage{
				ImageID:       imageModel.Id,
				BucketID:      bucket.Id,
				Storage:       bucket.Type,
				Status:        models.ImageStorageStatusSuccess,
				URL:           fileResult.URL,
				Thumbnail:     fileResult.ThumbnailURL,
				FileSize:      fileResult.FileSize,
				ThumbnailSize: fileResult.ThumbnailSize,
				SyncedAt:      &now,
			}
			if err := tx.Create(&storageStatus).Error; err != nil {
				return err
			}
			if len(existingTags) > 0 {
				relations := make([]models.ImageToTags, 0, len(existingTags))
				for _, tag := range existingTags {
					relations = append(relations, models.ImageToTags{ImageId: imageModel.Id, TagId: tag.Id})
				}
				if err := tx.Create(&relations).Error; err != nil {
					return err
				}
			}
			return chargeUploadQuota(tx, bucket, fileResult)
		}); err != nil {
			cleanupUnpublishedUpload(c, imageModel, fileResult)
			results = append(results, failedUploadResult(file, "保存记录失败或容量不足"))
			continue
		}

		responseResult := *fileResult
		responseResult.ID = imageModel.Id
		responseResult.FolderID = imageModel.FolderId
		responseResult.URL = applyPublicImageURL(setting, bucket.Type, bucketID, fileResult.URL)
		responseResult.ThumbnailURL = applyPublicImageURL(setting, bucket.Type, bucketID, fileResult.ThumbnailURL)
		results = append(results, responseResult)

		if setting.TGNotice {
			data := telegram.PlaceholderData{
				Username:    c.GetString("username"),
				Date:        time.Now().Format("2006-01-02 15:04:05"),
				Filename:    fileResult.FileName,
				StorageType: bucket.Type,
				URL:         buildImageResponseURL(c, setting, bucket.Type, bucketID, fileResult.URL),
			}
			if err := telegram.SendSimpleMsg(setting.TGBotToken, setting.TGReceivers, setting.TGNoticeText, data); err != nil {
				log.Println(err)
			}
		}
	}

	respondUploadBatch(uc, results, "上传成功", 0)
}

func uploadImageByURLLegacy(c *gin.Context, setting models.Settings, rawURL, tag, rawBucketID string, folderID int) {
	uc := uploads.NewUploadContext(c)
	db := database.GetDB()
	bucketID, err := resolveLegacyRequestedBucketID(c, setting, rawBucketID)
	if err != nil {
		uc.Fail(http.StatusBadRequest, "%v", err)
		return
	}

	allowed, err := canUseLegacyUploadBucket(c, setting, bucketID)
	if err != nil {
		uc.Fail(http.StatusInternalServerError, "校验存储权限失败：%v", err)
		return
	}
	if !allowed {
		uc.Fail(http.StatusForbidden, "无权使用该存储源")
		return
	}

	var bucket models.Buckets
	if err := db.DB.First(&bucket, bucketID).Error; err != nil {
		uc.Fail(http.StatusBadRequest, "存储配置不存在")
		return
	}

	if err := safefetch.ValidatePublicHTTPURL(rawURL); err != nil {
		uc.Fail(http.StatusBadRequest, "URL 不合法或禁止访问内网地址")
		return
	}
	resp, err := fetchUploadURL(c.Request.Context(), rawURL, 60*time.Second)
	if err != nil {
		uc.Fail(http.StatusBadRequest, "图片下载失败：%v", err)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		uc.Fail(http.StatusBadRequest, "图片下载失败，远端状态码：%d", resp.StatusCode)
		return
	}

	fileName := filepath.Base(rawURL)
	if fileName == "/" || fileName == "." || fileName == "" {
		fileName = fmt.Sprintf("url_image_%d.jpg", time.Now().Unix())
	}
	fileName = filepath.Base(strings.ReplaceAll(fileName, "\\", "/"))
	if strings.Contains(fileName, "..") || fileName == "." || fileName == "" {
		fileName = fmt.Sprintf("url_image_%d.jpg", time.Now().Unix())
	}
	maxBytes := int64(setting.MaxFileSize)
	if maxBytes <= 0 || maxBytes > images.MaxUploadBytes {
		maxBytes = images.MaxUploadBytes
	}
	fileBytes, err := safefetch.ReadLimited(resp.Body, maxBytes)
	if err != nil {
		uc.Fail(http.StatusBadRequest, "URL 图片超过限制或下载失败")
		return
	}
	info, err := inspectDownloadedImage(fileBytes)
	if err != nil || !images.AllowedMIME(info.MIME, strings.Split(setting.AllowedTypes, ",")) {
		uc.Fail(http.StatusBadRequest, "URL不是允许的安全图片")
		return
	}
	contentType := info.MIME

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreatePart(map[string][]string{
		"Content-Disposition": {`form-data; name="file"; filename="` + fileName + `"`},
		"Content-Type":        {contentType},
	})
	if err != nil {
		uc.Fail(http.StatusInternalServerError, "构造文件失败：%v", err)
		return
	}
	if _, err := part.Write(fileBytes); err != nil {
		uc.Fail(http.StatusInternalServerError, "构造文件失败：%v", err)
		return
	}
	if err := writer.Close(); err != nil {
		uc.Fail(http.StatusInternalServerError, "构造文件失败：%v", err)
		return
	}
	c.Request.Body = io.NopCloser(body)
	c.Request.Header.Set("Content-Type", writer.FormDataContentType())
	c.Request.ContentLength = int64(body.Len())

	file, header, err := c.Request.FormFile("file")
	if err != nil {
		uc.Fail(http.StatusInternalServerError, "构造文件失败：%v", err)
		return
	}
	defer file.Close()

	if bucket.Type != "default" && bucket.Type != "telegram" && bucket.Capacity > 0 && bucket.Usage+uint64(header.Size) > bucket.Capacity {
		uc.Fail(http.StatusBadRequest, "存储空间已满")
		return
	}
	uploader, err := uc.GetStorageUploader(&setting, &bucket)
	if err != nil {
		uc.Fail(http.StatusBadRequest, "获取上传器失败：%s", err.Error())
		return
	}
	fileResult, err := uploader.Upload(c, &setting, &bucket, header)
	if err != nil {
		uc.Fail(http.StatusInternalServerError, "上传失败[%s]：%v", fileName, err)
		return
	}

	imageModel := models.Image{
		Url:       fileResult.URL,
		Thumbnail: fileResult.ThumbnailURL,
		FileName:  fileResult.FileName,
		FileSize:  fileResult.FileSize,
		MimeType:  fileResult.MimeType,
		Width:     fileResult.Width,
		Height:    fileResult.Height,
		Storage:   fileResult.Storage,
		BucketId:  bucketID,
		FolderId:  folderID,
		UserId:    c.GetInt("user_id"),
		MD5:       md5.Md5(c.GetString("username") + fileResult.FileName),
		UUID:      CurrentOwnerKey(c),
	}
	if err := saveLegacyURLUpload(db.DB, bucket, &imageModel, fileResult, tag); err != nil {
		cleanupUnpublishedUpload(c, imageModel, fileResult)
		uc.Fail(http.StatusInternalServerError, "保存文件记录失败或容量不足")
		return
	}

	if setting.TGNotice {
		data := telegram.PlaceholderData{
			Username:    c.GetString("username"),
			Date:        time.Now().Format("2006-01-02 15:04:05"),
			Filename:    fileResult.FileName,
			StorageType: bucket.Type,
			URL:         buildImageResponseURL(c, setting, bucket.Type, bucketID, fileResult.URL),
		}
		if err := telegram.SendSimpleMsg(setting.TGBotToken, setting.TGReceivers, setting.TGNoticeText, data); err != nil {
			log.Println(err)
		}
	}

	responseResult := *fileResult
	responseResult.ID = imageModel.Id
	responseResult.FolderID = imageModel.FolderId
	responseResult.URL = applyPublicImageURL(setting, bucket.Type, bucketID, fileResult.URL)
	responseResult.ThumbnailURL = applyPublicImageURL(setting, bucket.Type, bucketID, fileResult.ThumbnailURL)
	uc.Success("URL 图片上传成功", map[string]any{"file": responseResult})
}

// Publish URL uploads and account the processed main/thumbnail bytes atomically,
// just like multipart uploads. Tags must not bypass the final quota check.
func saveLegacyURLUpload(db *gorm.DB, bucket models.Buckets, image *models.Image, result *interfaces.ImageUploadResult, tag string) error {
	return db.Transaction(func(tx *gorm.DB) error {
		resolved, err := services.ResolvePublicationFolder(tx, image.UserId, image.FolderId)
		if err != nil {
			return err
		}
		image.FolderId = resolved
		if err := tx.Create(image).Error; err != nil {
			return err
		}
		now := time.Now()
		storageStatus := models.ImageStorage{
			ImageID:       image.Id,
			BucketID:      bucket.Id,
			Storage:       bucket.Type,
			Status:        models.ImageStorageStatusSuccess,
			URL:           result.URL,
			Thumbnail:     result.ThumbnailURL,
			FileSize:      result.FileSize,
			ThumbnailSize: result.ThumbnailSize,
			SyncedAt:      &now,
		}
		if err := tx.Create(&storageStatus).Error; err != nil {
			return err
		}
		if tag != "" && tag != "0" {
			tagID, err := strconv.Atoi(tag)
			if err != nil {
				return err
			}
			if err := tx.Create(&models.ImageToTags{ImageId: image.Id, TagId: tagID}).Error; err != nil {
				return err
			}
		}
		return chargeUploadQuota(tx, bucket, result)
	})
}

func resolveLegacyRequestedBucketID(c *gin.Context, setting models.Settings, rawBucketID string) (int, error) {
	if rawBucketID != "" {
		bucketID, err := strconv.Atoi(rawBucketID)
		if err != nil || bucketID <= 0 {
			return 0, errors.New("存储ID无效")
		}
		return bucketID, nil
	}

	available, err := resolveLegacyUploadBuckets(c, setting)
	if err != nil {
		return 0, fmt.Errorf("获取可用存储源失败：%w", err)
	}
	for _, bucket := range available {
		if bucket.Id == defaultUploadStorageID(c.GetInt("user_role"), setting) {
			return bucket.Id, nil
		}
	}
	if len(available) > 0 {
		return available[0].Id, nil
	}
	return 0, errors.New("当前没有可用的存储源")
}
