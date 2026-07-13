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
	"oneimg/backend/utils/md5"
	"oneimg/backend/utils/telegram"
	"oneimg/backend/utils/uploads"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// uploadImagesLegacy keeps the original request-time, single-bucket upload
// path used when multi-storage synchronization is disabled.
func uploadImagesLegacy(c *gin.Context, setting models.Settings, existingTags []models.Tags) {
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

	files, err := uc.ParseAndValidateFiles()
	if err != nil {
		uc.Fail(http.StatusBadRequest, "文件解析失败：%v", err)
		return
	}

	if bucket.Type != "default" && bucket.Type != "telegram" && bucket.Capacity > 0 {
		var uploadSize uint64
		for _, file := range files {
			uploadSize += uint64(file.Size)
		}
		if bucket.Usage+uploadSize >= bucket.Capacity {
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
			uc.Fail(http.StatusInternalServerError, "文件[%s]上传失败：%v", file.Filename, uploadErr)
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
			UserId:    c.GetInt("user_id"),
			MD5:       md5.Md5(c.GetString("username") + fileResult.FileName),
			UUID:      GetUUID(c),
		}

		now := time.Now()
		if err := db.DB.Transaction(func(tx *gorm.DB) error {
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
			return nil
		}); err != nil {
			uc.Fail(http.StatusInternalServerError, "保存文件记录失败：%v", err)
			return
		}

		if fileResult.Storage != "default" {
			totalSize := uint64(fileResult.FileSize + fileResult.ThumbnailSize)
			update := db.DB.Model(&models.Buckets{}).
				Where("id = ? AND (usage + ? <= capacity OR type IN ('telegram','default') OR capacity = 0)", bucketID, totalSize).
				UpdateColumn("usage", gorm.Expr("usage + ?", totalSize))
			if update.Error != nil {
				log.Printf("更新Usage失败：%v", update.Error)
			}
		}

		responseResult := *fileResult
		responseResult.ID = imageModel.Id
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

	uc.Success("上传成功", map[string]any{
		"files": results,
		"count": len(results),
	})
}

func uploadImageByURLLegacy(c *gin.Context, setting models.Settings, rawURL, tag, rawBucketID string) {
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

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Get(rawURL)
	if err != nil {
		uc.Fail(http.StatusInternalServerError, "图片下载失败：%v", err)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		uc.Fail(http.StatusBadRequest, "图片下载失败，远端状态码：%d", resp.StatusCode)
		return
	}

	contentType := resp.Header.Get("Content-Type")
	if !strings.HasPrefix(contentType, "image/") {
		uc.Fail(http.StatusBadRequest, "URL不是图片类型")
		return
	}
	fileName := filepath.Base(rawURL)
	if fileName == "/" || fileName == "." || fileName == "" {
		fileName = fmt.Sprintf("url_image_%d.jpg", time.Now().Unix())
	}
	fileBytes, err := io.ReadAll(io.LimitReader(resp.Body, int64(setting.MaxFileSize)+1))
	if err != nil {
		uc.Fail(http.StatusInternalServerError, "读取图片失败：%v", err)
		return
	}
	if len(fileBytes) > setting.MaxFileSize {
		uc.Fail(http.StatusBadRequest, "URL 图片超过文件大小限制")
		return
	}

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
		UserId:    c.GetInt("user_id"),
		MD5:       md5.Md5(c.GetString("username") + fileResult.FileName),
		UUID:      GetUUID(c),
	}
	now := time.Now()
	if err := db.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&imageModel).Error; err != nil {
			return err
		}
		storageStatus := models.ImageStorage{
			ImageID:       imageModel.Id,
			BucketID:      bucketID,
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
		if tag != "" && tag != "0" {
			tagID, err := strconv.Atoi(tag)
			if err != nil {
				return err
			}
			return tx.Create(&models.ImageToTags{ImageId: imageModel.Id, TagId: tagID}).Error
		}
		return nil
	}); err != nil {
		uc.Fail(http.StatusInternalServerError, "保存文件记录失败：%v", err)
		return
	}

	if fileResult.Storage != "default" {
		totalSize := uint64(fileResult.FileSize + fileResult.ThumbnailSize)
		db.DB.Model(&models.Buckets{}).
			Where("id = ? AND (usage + ? <= capacity OR type IN ('telegram','default') OR capacity = 0)", bucketID, totalSize).
			UpdateColumn("usage", gorm.Expr("usage + ?", totalSize))
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
	responseResult.URL = applyPublicImageURL(setting, bucket.Type, bucketID, fileResult.URL)
	responseResult.ThumbnailURL = applyPublicImageURL(setting, bucket.Type, bucketID, fileResult.ThumbnailURL)
	uc.Success("URL 图片上传成功", map[string]any{"file": responseResult})
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
		if bucket.Id == setting.DefaultStorage {
			return bucket.Id, nil
		}
	}
	if len(available) > 0 {
		return available[0].Id, nil
	}
	return 0, errors.New("当前没有可用的存储源")
}
