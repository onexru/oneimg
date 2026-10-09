package controllers

import (
	"encoding/json"
	"log"
	"net/http"
	"oneimg/backend/database"
	"oneimg/backend/interfaces"
	"oneimg/backend/models"
	"oneimg/backend/services"
	"oneimg/backend/utils/images"
	"oneimg/backend/utils/md5"
	"oneimg/backend/utils/result"
	"oneimg/backend/utils/settings"
	"oneimg/backend/utils/storagepolicy"
	"oneimg/backend/utils/telegram"
	"oneimg/backend/utils/uploadpolicy"
	"oneimg/backend/utils/uploads"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// UploadImages 图片上传主入口
func UploadImages(c *gin.Context) {
	release, ok := acquireUploadRequest(c)
	if !ok {
		return
	}
	defer release()
	setting, err := settings.GetSettings()
	if err != nil {
		c.JSON(500, result.Error(500, "获取上传配置失败"))
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, uploadpolicy.RequestBytes(setting))
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

	if _, err := parseConfiguredUploadFiles(c, setting); err != nil {
		uc.Fail(uploadParseErrorStatus(err), "文件解析失败：%v", err)
		return
	}
	folderID, validFolder := multipartUploadFolder(c)
	if !validFolder {
		return
	}
	var tags []string
	var existingTags []models.Tags
	tagsStr := c.PostForm("tags")
	if tagsStr != "" {
		err := json.Unmarshal([]byte(tagsStr), &tags)
		if err != nil {
			uc.Fail(400, "Tags参数格式错误：%v", err)
			return
		}

		if err := validateUploadTags(setting, tags); err != nil {
			uc.Fail(400, "%v", err)
			return
		}
		err = db.DB.Where("name IN ?", tags).Find(&existingTags).Error
		if err != nil {
			uc.Fail(500, "Tag查询失败：%v", err)
			return
		}
	}

	if !setting.MultiStorageSync {
		uploadImagesLegacy(c, setting, existingTags, folderID)
		return
	}

	localBucket, syncBuckets, err := resolveUploadBuckets(c, setting)
	if err != nil {
		uc.Fail(500, "获取用户同步存储源失败：%v", err)
		return
	}

	// 解析并校验上传文件
	files, err := parseConfiguredUploadFiles(c, setting)
	if err != nil {
		uc.Fail(uploadParseErrorStatus(err), "文件解析失败：%v", err)
		return
	}

	// 请求内只处理一次并持久化到本机；远端副本由持久化后台任务上传。
	uploader, err := uc.GetStorageUploader(&setting, &localBucket)
	if err != nil {
		uc.Fail(500, "初始化本机存储失败：%s", err.Error())
		return
	}

	// 批量处理文件上传（参数匹配接口定义）
	uploadResults := make([]interfaces.ImageUploadResult, 0, len(files))
	successCount := 0

	for _, file := range files {
		fileResult, err := uploader.Upload(c, &setting, &localBucket, file)
		if err != nil {
			uploadResults = append(uploadResults, failedUploadResult(file, "文件保存失败"))
			continue
		}

		// 保存图片信息到数据库
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
			FolderId:  folderID,
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

			if len(existingTags) > 0 {
				imageTagRelations := make([]models.ImageToTags, 0, len(existingTags))
				for _, tag := range existingTags {
					imageTagRelations = append(imageTagRelations, models.ImageToTags{
						ImageId: imageModel.Id,
						TagId:   tag.Id,
					})
				}
				if err := tx.Create(&imageTagRelations).Error; err != nil {
					return err
				}
			}
			return chargeUploadQuota(tx, localBucket, fileResult)
		})
		if err != nil {
			cleanupLocalUpload(imageModel)
			uploadResults = append(uploadResults, failedUploadResult(file, "保存文件记录失败"))
			continue
		}

		responseResult := *fileResult
		responseResult.ID = imageModel.Id
		responseResult.FolderID = imageModel.FolderId
		uploadResults = append(uploadResults, responseResult)

		if setting.TGNotice {
			placeholderData := telegram.PlaceholderData{
				Username:    c.GetString("username"),
				Date:        time.Now().Format("2006-01-02 15:04:05"),
				Filename:    fileResult.FileName,
				StorageType: localBucket.Type,
				URL:         buildImageResponseURL(c, setting, localBucket.Type, localBucket.Id, fileResult.URL),
			}

			err := telegram.SendSimpleMsg(
				setting.TGBotToken,   // 机器人Token
				setting.TGReceivers,  // 接收者ChatID
				setting.TGNoticeText, // 模板文本
				placeholderData,      // 占位符数据
			)
			if err != nil {
				log.Println(err)
				// 忽略错误
			}
		}

		successCount++
	}

	if successCount > 0 {
		services.WakeStorageSyncWorker()
	}
	respondUploadBatch(uc, uploadResults, "文件已保存到本机，正在后台同步", len(syncBuckets))
}

// UploadImage 单文件上传
func UploadImage(c *gin.Context) {
	UploadImages(c)
}

// 获取上传配置
func GetUploadConfig(c *gin.Context) {
	var tags []models.Tags

	db := database.GetDB().DB
	if err := db.Model(&models.Tags{}).Find(&tags).Error; err != nil {
		c.JSON(http.StatusInternalServerError, result.Error(500, "获取标签列表失败"))
		return
	}

	setting, err := settings.GetSettings()
	if err != nil {
		c.JSON(http.StatusInternalServerError, result.Error(500, "获取上传设置失败"))
		return
	}

	toResponse := func(bucket models.Buckets) map[string]any {
		return map[string]any{
			"id":   bucket.Id,
			"name": bucket.Name,
			"type": bucket.Type,
		}
	}

	config := map[string]any{
		"tags":               tags,
		"multi_storage_sync": setting.MultiStorageSync,
		"max_upload_files":   uploadpolicy.MaxFiles(setting),
		"max_file_size":      images.UploadByteLimit(int64(setting.MaxFileSize)),
		"allowed_types":      effectiveUploadMIMEs(setting),
		"tag_max_length":     uploadpolicy.TagLength(setting),
		"random_image_limit": uploadpolicy.RandomLimit(setting),
	}
	if !setting.MultiStorageSync {
		buckets, err := resolveLegacyUploadBuckets(c, setting)
		if err != nil {
			internalFailure(c, "获取存储桶列表失败", err)
			return
		}
		bucketRes := make([]map[string]any, 0, len(buckets))
		effectiveDefaultBucket := defaultUploadStorageID(c.GetInt("user_role"), setting)
		defaultAvailable := false
		for _, bucket := range buckets {
			bucketRes = append(bucketRes, toResponse(bucket))
			if bucket.Id == effectiveDefaultBucket {
				defaultAvailable = true
			}
		}
		if !defaultAvailable && len(buckets) > 0 {
			effectiveDefaultBucket = buckets[0].Id
		}
		if len(buckets) == 0 {
			policy, err := storagepolicy.Load(database.GetDB().DB)
			if err != nil {
				internalFailure(c, "获取角色存储策略失败", err)
				return
			}
			if policy.Custom(c.GetInt("user_role")) {
				effectiveDefaultBucket = 0
			}
		}
		config["buckets"] = bucketRes
		config["sync_buckets"] = []map[string]any{}
		config["default_bucket"] = effectiveDefaultBucket
	} else {
		localBucket, syncBuckets, err := resolveUploadBuckets(c, setting)
		if err != nil {
			internalFailure(c, "获取同步存储源失败", err)
			return
		}
		bucketRes := make([]map[string]any, 0, len(syncBuckets)+1)
		bucketRes = append(bucketRes, toResponse(localBucket))
		syncBucketRes := make([]map[string]any, 0, len(syncBuckets))
		for _, bucket := range syncBuckets {
			item := toResponse(bucket)
			bucketRes = append(bucketRes, item)
			syncBucketRes = append(syncBucketRes, item)
		}
		config["buckets"] = bucketRes
		config["local_bucket"] = toResponse(localBucket)
		config["sync_buckets"] = syncBucketRes
		config["default_bucket"] = localBucket.Id
	}

	c.JSON(http.StatusOK, result.Success("ok", config))
}
