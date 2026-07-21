package controllers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"oneimg/backend/database"
	"oneimg/backend/interfaces"
	"oneimg/backend/models"
	"oneimg/backend/services"
	"oneimg/backend/utils/md5"
	"oneimg/backend/utils/result"
	"oneimg/backend/utils/settings"
	"oneimg/backend/utils/telegram"
	"oneimg/backend/utils/uploads"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// UploadImages 图片上传主入口
func UploadImages(c *gin.Context) {
	uc := uploads.NewUploadContext(c)
	db := database.GetDB()

	var tags []string
	var existingTags []models.Tags
	tagsStr := c.PostForm("tags")
	if tagsStr != "" {
		err := json.Unmarshal([]byte(tagsStr), &tags)
		if err != nil {
			uc.Fail(400, "Tags参数格式错误：%v", err)
			return
		}

		err = db.DB.Where("name IN ?", tags).Find(&existingTags).Error
		if err != nil {
			uc.Fail(500, "Tag查询失败：%v", err)
			return
		}
	}

	// 获取系统配置
	setting, err := settings.GetSettings()
	if err != nil {
		uc.Fail(500, "获取上传配置失败：%v", err)
		return
	}
	if !setting.MultiStorageSync {
		uploadImagesLegacy(c, setting, existingTags)
		return
	}

	localBucket, syncBuckets, err := resolveUploadBuckets(c, setting)
	if err != nil {
		uc.Fail(500, "获取用户同步存储源失败：%v", err)
		return
	}

	// 解析并校验上传文件
	files, err := uc.ParseAndValidateFiles()
	if err != nil {
		uc.Fail(400, "文件解析失败")
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
			uc.Fail(500, "文件[%s]保存到本机失败：%v", file.Filename, err)
			return
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
			UserId:    c.GetInt("user_id"),
			MD5:       md5.Md5(c.GetString("username") + fileResult.FileName),
			UUID:      GetUUID(c),
		}

		now := time.Now()
		err = db.DB.Transaction(func(tx *gorm.DB) error {
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
			return nil
		})
		if err != nil {
			cleanupLocalUpload(imageModel)
			uc.Fail(500, "保存文件记录失败：%v", err)
			return
		}

		responseResult := *fileResult
		responseResult.ID = imageModel.Id
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

	if successCount == 0 {
		uc.Fail(500, "所有文件上传失败")
		return
	}
	services.WakeStorageSyncWorker()

	// 返回上传结果
	uc.Success("文件已保存到本机，正在后台同步", map[string]any{
		"files":        uploadResults,
		"count":        successCount,
		"sync_targets": len(syncBuckets),
	})
}

// UploadImage 单文件上传
func UploadImage(c *gin.Context) {
	UploadImages(c)
}

// AddImageTag 单个添加图片标签
func AddImageTag(c *gin.Context) {
	type TagRequest struct {
		Id  int    `json:"id"`  // 图片ID
		Tag string `json:"tag"` // 标签ID（前端传字符串，后端转换）
	}

	var req TagRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, result.Error(400, "参数解析失败："+err.Error()))
		return
	}

	if req.Id <= 0 || req.Tag == "" {
		c.JSON(http.StatusBadRequest, result.Error(400, "参数错误"))
		return
	}

	tagId, err := strconv.Atoi(req.Tag)
	if err != nil || tagId <= 0 {
		c.JSON(http.StatusBadRequest, result.Error(400, "标签ID无效"))
		return
	}
	imageId := req.Id

	db := database.GetDB().DB

	// 1. 查询图片是否存在
	var image models.Image
	if err := db.Where("id = ?", imageId).First(&image).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusBadRequest, result.Error(400, "图片不存在"))
		} else {
			c.JSON(http.StatusInternalServerError, result.Error(500, "查询图片失败："+err.Error()))
		}
		return
	}

	// 2. 校验图片操作权限
	if !CheckImageAccessPermission(c, image, "image:tag:add") {
		c.JSON(http.StatusForbidden, result.Error(403, "无权操作"))
		return
	}

	// 3. 查询标签是否存在
	var tag models.Tags
	if err := db.Where("id = ?", tagId).First(&tag).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusBadRequest, result.Error(400, "标签不存在"))
		} else {
			c.JSON(http.StatusInternalServerError, result.Error(500, "查询标签失败："+err.Error()))
		}
		return
	}

	// 4. 查询图片是否已经添加过该标签
	var imageTag models.ImageToTags
	if err := db.Where("image_id = ? AND tag_id = ?", imageId, tagId).First(&imageTag).Error; err == nil {
		c.JSON(http.StatusBadRequest, result.Error(400, "图片已添加过该标签"))
		return
	} else if err != gorm.ErrRecordNotFound {
		c.JSON(http.StatusInternalServerError, result.Error(500, "检查标签关联失败："+err.Error()))
		return
	}

	// 5. 添加图片标签关联
	if err := db.Create(&models.ImageToTags{
		ImageId: imageId,
		TagId:   tagId,
	}).Error; err != nil {
		c.JSON(http.StatusInternalServerError, result.Error(500, "添加标签失败："+err.Error()))
		return
	}

	c.JSON(http.StatusOK, result.Success("标签添加成功", nil))
}

// DeleteImageTag 单个删除图片标签
func DeleteImageTag(c *gin.Context) {
	type TagRequest struct {
		Id  int `json:"id"`  // 图片ID
		Tag int `json:"tag"` // 标签ID
	}

	var req TagRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, result.Error(400, "参数解析失败："+err.Error()))
		return
	}

	if req.Id <= 0 || req.Tag <= 0 {
		c.JSON(http.StatusBadRequest, result.Error(400, "参数错误"))
		return
	}

	tagId := req.Tag
	imageId := req.Id
	db := database.GetDB().DB

	// 1. 查询图片是否存在
	var image models.Image
	if err := db.Where("id = ?", imageId).First(&image).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusBadRequest, result.Error(400, "图片不存在"))
		} else {
			c.JSON(http.StatusInternalServerError, result.Error(500, "查询图片失败："+err.Error()))
		}
		return
	}

	// 2. 校验图片操作权限
	if !CheckImageAccessPermission(c, image, "image:tag:delete") {
		c.JSON(http.StatusForbidden, result.Error(403, "无权操作"))
		return
	}

	// 3. 检查标签是否已经添加过该图片
	var imageTag models.ImageToTags
	if err := db.Where("image_id = ? AND tag_id = ?", imageId, tagId).First(&imageTag).Error; err != nil {
		c.JSON(http.StatusBadRequest, result.Error(400, "关联不存在"))
		return
	}

	// 4. 删除图片标签关联
	if err := db.Delete(&imageTag).Error; err != nil {
		c.JSON(http.StatusInternalServerError, result.Error(500, "删除标签失败："+err.Error()))
		return
	}

	c.JSON(http.StatusOK, result.Success("标签删除成功", nil))
}

// DeleteImageTags 批量删除图片标签
func DeleteImageTags(c *gin.Context) {
	type Request struct {
		Images []int  `json:"image_ids"`
		Tag    string `json:"tag_id"`
	}

	var req Request
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, result.Error(400, "参数解析失败："+err.Error()))
		return
	}

	tagID, err := strconv.Atoi(req.Tag)
	if err != nil || len(req.Images) <= 0 || tagID <= 0 {
		c.JSON(http.StatusBadRequest, result.Error(400, "参数错误"))
		return
	}

	db := database.GetDB().DB

	// 查询所有目标图片记录并校验权限
	var images []models.Image
	if err := db.Where("id IN ?", req.Images).Find(&images).Error; err != nil {
		c.JSON(http.StatusInternalServerError, result.Error(500, "查询图片数据失败"))
		return
	}

	// 核对查询到的图片数量，防止前端传入不存在的图片ID
	if len(images) == 0 {
		c.JSON(http.StatusBadRequest, result.Error(400, "指定图片不存在"))
		return
	}

	var validImageIDs []int
	for _, img := range images {
		// 校验每一张图片的操作权限
		if !CheckImageAccessPermission(c, img, "image:tag:delete") {
			c.JSON(http.StatusForbidden, result.Error(403, "无权操作部分或全部图片"))
			return
		}
		validImageIDs = append(validImageIDs, int(img.Id)) // 收集有效ID
	}

	// 直接使用单条SQL执行批量删除，避免for循环中执行多条SQL
	if err := db.Where("image_id IN ? AND tag_id = ?", validImageIDs, tagID).Delete(&models.ImageToTags{}).Error; err != nil {
		c.JSON(http.StatusInternalServerError, result.Error(500, "批量删除标签失败："+err.Error()))
		return
	}

	c.JSON(http.StatusOK, result.Success("批量删除标签成功", nil))
}

// AddImageTags 批量添加图片标签
func AddImageTags(c *gin.Context) {
	type Request struct {
		Images []int  `json:"image_ids"`
		Tag    string `json:"tag_id"`
	}

	var req Request
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, result.Error(400, "参数解析失败："+err.Error()))
		return
	}

	tagID, err := strconv.Atoi(req.Tag)
	if err != nil || len(req.Images) <= 0 || tagID <= 0 {
		c.JSON(http.StatusBadRequest, result.Error(400, "参数错误"))
		return
	}

	db := database.GetDB().DB

	// 1. 检查标签是否存在
	var tag models.Tags
	if err := db.Where("id = ?", tagID).First(&tag).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusBadRequest, result.Error(400, "标签不存在"))
		} else {
			c.JSON(http.StatusInternalServerError, result.Error(500, "查询标签失败："+err.Error()))
		}
		return
	}

	// 查询并校验图片列表权限
	var images []models.Image
	if err := db.Where("id IN ?", req.Images).Find(&images).Error; err != nil {
		c.JSON(http.StatusInternalServerError, result.Error(500, "查询图片列表失败："+err.Error()))
		return
	}

	var validImageIDs []int
	for _, img := range images {
		// 校验每张图的权限
		if !CheckImageAccessPermission(c, img, "image:tag:add") {
			c.JSON(http.StatusForbidden, result.Error(403, "无权操作部分或全部图片"))
			return
		}
		validImageIDs = append(validImageIDs, int(img.Id))
	}

	if len(validImageIDs) == 0 {
		c.JSON(http.StatusBadRequest, result.Error(400, "无效的图片ID"))
		return
	}

	// 3. 筛选出已经存在关联的记录，防止重复添加
	var existRelations []int
	if err := db.Model(&models.ImageToTags{}).
		Where("image_id IN ? AND tag_id = ?", validImageIDs, tagID).
		Pluck("image_id", &existRelations).Error; err != nil {
		c.JSON(http.StatusInternalServerError, result.Error(500, "检查标签关联失败："+err.Error()))
		return
	}

	existRelationMap := make(map[int]bool)
	for _, id := range existRelations {
		existRelationMap[id] = true
	}

	// 4. 构建需要插入的新关联数据
	var insertData []models.ImageToTags
	for _, imageID := range validImageIDs {
		if existRelationMap[imageID] {
			continue // 如果已存在则跳过
		}
		insertData = append(insertData, models.ImageToTags{
			ImageId: imageID,
			TagId:   tagID,
		})
	}

	// 5. 批量插入
	if len(insertData) > 0 {
		err := db.Transaction(func(tx *gorm.DB) error {
			if err := tx.CreateInBatches(&insertData, 100).Error; err != nil {
				return err
			}
			return nil
		})
		if err != nil {
			c.JSON(http.StatusInternalServerError, result.Error(500, "批量添加标签失败："+err.Error()))
			return
		}
	} else {
		c.JSON(http.StatusOK, result.Success("没有需要添加的标签(可能已全部存在)", nil))
		return
	}

	c.JSON(http.StatusOK, result.Success("批量添加标签成功", nil))
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
	}
	if !setting.MultiStorageSync {
		buckets, err := resolveLegacyUploadBuckets(c, setting)
		if err != nil {
			c.JSON(http.StatusInternalServerError, result.Error(500, "获取存储桶列表失败："+err.Error()))
			return
		}
		bucketRes := make([]map[string]any, 0, len(buckets))
		effectiveDefaultBucket := setting.DefaultStorage
		defaultAvailable := false
		for _, bucket := range buckets {
			bucketRes = append(bucketRes, toResponse(bucket))
			if bucket.Id == setting.DefaultStorage {
				defaultAvailable = true
			}
		}
		if !defaultAvailable && len(buckets) > 0 {
			effectiveDefaultBucket = buckets[0].Id
		}
		config["buckets"] = bucketRes
		config["sync_buckets"] = []map[string]any{}
		config["default_bucket"] = effectiveDefaultBucket
	} else {
		localBucket, syncBuckets, err := resolveUploadBuckets(c, setting)
		if err != nil {
			c.JSON(http.StatusInternalServerError, result.Error(500, "获取同步存储源失败："+err.Error()))
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

// 通过URL上传图片
func UploadImagesByURL(c *gin.Context) {
	uc := uploads.NewUploadContext(c)
	db := database.GetDB()

	type URLUploadRequest struct {
		Urls     string `json:"url" binding:"required"`
		Tag      string `json:"tag_id"`
		BucketID string `json:"bucket_id"` // 兼容旧客户端，目标存储源以用户配置为准。
	}

	var req URLUploadRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		uc.Fail(400, "参数格式错误：%v", err)
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
		uploadImageByURLLegacy(c, setting, req.Urls, req.Tag, req.BucketID)
		return
	}

	localBucket, syncBuckets, err := resolveUploadBuckets(c, setting)
	if err != nil {
		uc.Fail(500, "获取用户同步存储源失败：%v", err)
		return
	}

	// 下载图片
	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Get(req.Urls)
	if err != nil {
		uc.Fail(500, "图片下载失败：%v", err)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		uc.Fail(400, "图片下载失败，远端状态码：%d", resp.StatusCode)
		return
	}

	contentType := resp.Header.Get("Content-Type")
	if !strings.HasPrefix(contentType, "image/") {
		uc.Fail(400, "URL不是图片类型")
		return
	}

	fileName := filepath.Base(req.Urls)
	if fileName == "/" || fileName == "." || fileName == "" {
		fileName = fmt.Sprintf("url_image_%d.jpg", time.Now().Unix())
	}

	maxRead := int64(setting.MaxFileSize) + 1
	fileBytes, err := io.ReadAll(io.LimitReader(resp.Body, maxRead))
	if err != nil {
		uc.Fail(500, "读取图片失败：%v", err)
		return
	}
	if int64(len(fileBytes)) > int64(setting.MaxFileSize) {
		uc.Fail(400, "URL 图片超过文件大小限制")
		return
	}
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
		UserId:    c.GetInt("user_id"),
		MD5:       md5.Md5(c.GetString("username") + fileResult.FileName),
		UUID:      GetUUID(c),
	}

	now := time.Now()
	err = db.DB.Transaction(func(tx *gorm.DB) error {
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
		return nil
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
	services.WakeStorageSyncWorker()

	uc.Success("URL 图片已保存到本机，正在后台同步", map[string]any{
		"file":         responseResult,
		"sync_targets": len(syncBuckets),
	})
}
