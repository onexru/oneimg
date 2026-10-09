package controllers

import (
	"bytes"
	"fmt"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"oneimg/backend/database"
	"oneimg/backend/models"
	"oneimg/backend/services"
	"oneimg/backend/utils/images"
	"oneimg/backend/utils/md5"
	"oneimg/backend/utils/safefetch"
	"oneimg/backend/utils/settings"
	"oneimg/backend/utils/telegram"
	"oneimg/backend/utils/uploads"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// fetchUploadURL is injectable for isolated controller tests; production keeps
// safefetch's DNS, redirect, and private-address defenses.
var fetchUploadURL = safefetch.Get

// 通过URL上传图片
func UploadImagesByURL(c *gin.Context) {
	release, ok := acquireUploadRequest(c)
	if !ok {
		return
	}
	defer release()
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 32<<10)
	defer func() {
		if c.Request.MultipartForm != nil {
			_ = c.Request.MultipartForm.RemoveAll()
		}
	}()
	uc := uploads.NewUploadContext(c)
	if c.GetInt("user_role") == models.RoleGuest && (c.GetInt("user_id") >= 0 || !validGuestUUID(CurrentOwnerKey(c))) {
		uc.Fail(403, "游客身份无效")
		return
	}
	db := database.GetDB()

	type URLUploadRequest struct {
		Urls     string `json:"url" binding:"required"`
		Tag      string `json:"tag_id"`
		FolderID int    `json:"folder_id"`
		BucketID string `json:"bucket_id"` // 兼容旧客户端，目标存储源以用户配置为准。
	}

	var req URLUploadRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		uc.Fail(400, "参数格式错误：%v", err)
		return
	}

	if !validateUploadFolder(c, req.FolderID) {
		return
	}
	if req.Urls == "" {
		uc.Fail(400, "URL不能为空")
		return
	}
	if req.Tag != "" && req.Tag != "0" {
		var tags models.Tags
		if err := db.DB.Where("id = ?", req.Tag).First(&tags).Error; err != nil {
			uc.Fail(400, "标签不存在")
			return
		}
	}

	setting, err := settings.GetSettings()
	if err != nil {
		uc.Fail(500, "获取上传配置失败：%v", err)
		return
	}
	if !setting.MultiStorageSync {
		uploadImageByURLLegacy(c, setting, req.Urls, req.Tag, req.BucketID, req.FolderID)
		return
	}

	localBucket, syncBuckets, err := resolveUploadBuckets(c, setting)
	if err != nil {
		uc.Fail(500, "获取用户同步存储源失败：%v", err)
		return
	}

	// 下载图片（拒绝内网/metadata，防止 SSRF）
	if err := safefetch.ValidatePublicHTTPURL(req.Urls); err != nil {
		uc.Fail(400, "URL 不合法或禁止访问内网地址")
		return
	}
	resp, err := fetchUploadURL(c.Request.Context(), req.Urls, 60*time.Second)
	if err != nil {
		uc.Fail(400, "图片下载失败：%v", err)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		uc.Fail(400, "图片下载失败，远端状态码：%d", resp.StatusCode)
		return
	}

	fileName := filepath.Base(req.Urls)
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

	part, _ := writer.CreatePart(map[string][]string{
		"Content-Disposition": {`form-data; name="file"; filename="` + fileName + `"`},
		"Content-Type":        {contentType},
	})
	part.Write(fileBytes)
	writer.Close()

	// 伪装请求
	c.Request.Body = io.NopCloser(body)
	c.Request.Header.Set("Content-Type", writer.FormDataContentType())
	c.Request.ContentLength = int64(body.Len())

	file, header, err := c.Request.FormFile("file")
	if err != nil {
		uc.Fail(500, "构造文件失败：%v", err)
		return
	}
	defer file.Close()

	uploader, err := uc.GetStorageUploader(&setting, &localBucket)
	if err != nil {
		uc.Fail(500, "初始化本机存储失败：%s", err.Error())
		return
	}

	fileResult, err := uploader.Upload(c, &setting, &localBucket, header)
	if err != nil {
		uc.Fail(500, "保存到本机失败[%s]：%v", fileName, err)
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
		BucketId:  localBucket.Id,
		FolderId:  req.FolderID,
		UserId:    c.GetInt("user_id"),
		MD5:       md5.Md5(c.GetString("username") + fileResult.FileName),
		UUID:      CurrentOwnerKey(c),
	}

	now := time.Now()
	err = db.DB.Transaction(func(tx *gorm.DB) error {
		resolved, err := services.ResolvePublicationFolder(tx, imageModel.UserId, imageModel.FolderId)
		if err != nil {
			return err
		}
		imageModel.FolderId = resolved
		if err := tx.Create(&imageModel).Error; err != nil {
			return err
		}
		localStatus := models.ImageStorage{
			ImageID:       imageModel.Id,
			BucketID:      localBucket.Id,
			Storage:       localBucket.Type,
			Status:        models.ImageStorageStatusSuccess,
			URL:           fileResult.URL,
			Thumbnail:     fileResult.ThumbnailURL,
			FileSize:      fileResult.FileSize,
			ThumbnailSize: fileResult.ThumbnailSize,
			SyncedAt:      &now,
		}
		if err := tx.Create(&localStatus).Error; err != nil {
			return err
		}
		for _, bucket := range syncBuckets {
			storageStatus := models.ImageStorage{
				ImageID:       imageModel.Id,
				BucketID:      bucket.Id,
				Storage:       bucket.Type,
				Status:        models.ImageStorageStatusPending,
				URL:           fileResult.URL,
				Thumbnail:     fileResult.ThumbnailURL,
				FileSize:      fileResult.FileSize,
				ThumbnailSize: fileResult.ThumbnailSize,
			}
			if err := tx.Create(&storageStatus).Error; err != nil {
				return err
			}
		}
		if req.Tag != "" && req.Tag != "0" {
			tagID, conversionErr := strconv.Atoi(req.Tag)
			if conversionErr != nil {
				return conversionErr
			}
			if err := tx.Create(&models.ImageToTags{ImageId: imageModel.Id, TagId: tagID}).Error; err != nil {
				return err
			}
		}
		return chargeUploadQuota(tx, localBucket, fileResult)
	})
	if err != nil {
		cleanupLocalUpload(imageModel)
		uc.Fail(500, "保存文件记录失败：%v", err)
		return
	}

	// TG通知
	if setting.TGNotice {
		placeholderData := telegram.PlaceholderData{
			Username:    c.GetString("username"),
			Date:        time.Now().Format("2006-01-02 15:04:05"),
			Filename:    fileResult.FileName,
			StorageType: localBucket.Type,
			URL:         buildImageResponseURL(c, setting, localBucket.Type, localBucket.Id, fileResult.URL),
		}
		if err := telegram.SendSimpleMsg(setting.TGBotToken, setting.TGReceivers, setting.TGNoticeText, placeholderData); err != nil {
			log.Println(err)
		}
	}

	responseResult := *fileResult
	responseResult.ID = imageModel.Id
	responseResult.FolderID = imageModel.FolderId
	services.WakeStorageSyncWorker()

	uc.Success("URL 图片已保存到本机，正在后台同步", map[string]any{
		"file":         responseResult,
		"sync_targets": len(syncBuckets),
	})
}
