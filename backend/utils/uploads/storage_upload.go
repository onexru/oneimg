package uploads

import (
	"bytes"
	"fmt"
	"mime/multipart"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"oneimg/backend/database"
	"oneimg/backend/interfaces"
	"oneimg/backend/models"
	"oneimg/backend/storage"
	"oneimg/backend/utils/images"
	"oneimg/backend/utils/securestorage"
)

type driverUploader struct{}

func (u *driverUploader) Upload(c *gin.Context, setting *models.Settings, bucket *models.Buckets, header *multipart.FileHeader) (*interfaces.ImageUploadResult, error) {
	processed, err := processUpload(c, setting, bucket, header)
	if err != nil {
		return nil, err
	}
	driver, err := storage.New(*setting, *bucket)
	if err != nil {
		return nil, err
	}
	template := setting.DefaultPath
	if template == "" {
		template = "uploads/{year}/{month}"
	}
	subdir := images.ImageSvc.ReplaceMagicVariables(template, header.Filename, c.GetInt("user_role"))
	key, err := storage.Key(strings.TrimSuffix(subdir, "/") + "/" + processed.UniqueFileName)
	if err != nil {
		return nil, err
	}
	mainURL := "/" + key
	thumbnailURL := ""
	thumbKey, err := storage.Key(strings.TrimSuffix(subdir, "/") + "/thumbnails/" + processed.UniqueFileName)
	if err != nil {
		return nil, err
	}
	// Encode all artifacts before publishing the first; drivers never encrypt.
	main, err := securestorage.Encode(processed.CompressedBytes, setting.EncryptedStorage)
	if err != nil {
		return nil, err
	}
	var thumb []byte
	if setting.Thumbnail && len(processed.ThumbnailBytes) > 0 {
		thumb, err = securestorage.Encode(processed.ThumbnailBytes, setting.EncryptedStorage)
		if err != nil {
			return nil, err
		}
	}
	ctx := c.Request.Context()
	metadata := map[string]any{}
	info, err := driver.Upload(ctx, storage.Ref{Key: key}, bytes.NewReader(main), storage.UploadOptions{Size: int64(len(main)), ContentType: storageContentType(processed.MimeType, setting.EncryptedStorage), FileName: processed.UniqueFileName, Private: setting.EncryptedStorage})
	storage.MergeArtifactMetadata(metadata, info, false)
	if err != nil {
		if cleanupErr := compensateUpload(driver, storage.ArtifactRef(key, metadata, false)); cleanupErr != nil {
			retainCleanupManifest(c, bucket, mainURL, "", processed, metadata)
		}
		return nil, fmt.Errorf("上传主图失败: %w", err)
	}
	if len(thumb) > 0 {
		info, thumbErr := driver.Upload(ctx, storage.Ref{Key: thumbKey}, bytes.NewReader(thumb), storage.UploadOptions{Size: int64(len(thumb)), ContentType: storageContentType(http.DetectContentType(processed.ThumbnailBytes), setting.EncryptedStorage), FileName: "thumbnail_" + processed.UniqueFileName, Private: setting.EncryptedStorage})
		storage.MergeArtifactMetadata(metadata, info, true)
		if thumbErr == nil {
			thumbnailURL = "/" + thumbKey
		} else {
			if cleanupErr := compensateUpload(driver, storage.ArtifactRef(thumbKey, metadata, true)); cleanupErr != nil {
				_ = compensateUpload(driver, storage.ArtifactRef(key, metadata, false))
				retainCleanupManifest(c, bucket, mainURL, "/"+thumbKey, processed, metadata)
				return nil, fmt.Errorf("缩略图清理失败，请重试删除")
			}
		}
	}
	if bucket.Type == "telegram" {
		db := database.GetDB()
		if db == nil || db.DB == nil {
			err = fmt.Errorf("数据库未初始化")
		} else {
			row := models.ImageTeleGram{TGFileId: storage.MetadataString(metadata, "tg_file_id"), TGThumbnailFileId: storage.MetadataString(metadata, "tg_thumbnail_file_id"), TGMessageId: storage.MetadataInt(metadata, "tg_message_id"), TGThumbnailMessageId: storage.MetadataInt(metadata, "tg_thumbnail_message_id"), FileName: processed.UniqueFileName}
			err = db.DB.Create(&row).Error
		}
		if err != nil {
			mainCleanup := compensateUpload(driver, storage.ArtifactRef(key, metadata, false))
			var thumbCleanup error
			if thumbnailURL != "" {
				thumbCleanup = compensateUpload(driver, storage.ArtifactRef(thumbKey, metadata, true))
			}
			if mainCleanup != nil || thumbCleanup != nil {
				retainCleanupManifest(c, bucket, mainURL, thumbnailURL, processed, metadata)
			}
			return nil, fmt.Errorf("保存Telegram图片信息失败")
		}
	}
	return &interfaces.ImageUploadResult{Success: true, Message: "上传成功", FileName: processed.UniqueFileName, FileSize: int64(len(processed.CompressedBytes)), ThumbnailSize: publishedThumbnailSize(thumbnailURL, processed.ThumbnailBytes), MimeType: processed.MimeType, URL: mainURL, ThumbnailURL: thumbnailURL, Storage: bucket.Type, CreatedAt: time.Now().Format("2006-01-02 15:04:05"), Width: processed.Width, Height: processed.Height}, nil
}
