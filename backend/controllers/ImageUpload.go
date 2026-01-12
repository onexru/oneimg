package controllers

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"oneimg/backend/config"
	"oneimg/backend/database"
	"oneimg/backend/interfaces"
	"oneimg/backend/models"
	"oneimg/backend/utils/md5"
	"oneimg/backend/utils/result"
	"oneimg/backend/utils/settings"
	"oneimg/backend/utils/telegram"
	"oneimg/backend/utils/uploads"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// UploadImages 图片上传主入口
func UploadImages(c *gin.Context) {
	// 初始化上传上下文
	uc := uploads.NewUploadContext(c)

	// 获取数据库连接
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

	// 获取存储ID
	var bucketID int
	bucketIDStr := c.PostForm("bucket_id")
	if bucketIDStr != "" {
		// 转换为int
		bucketID, err = strconv.Atoi(bucketIDStr)
		if err != nil {
			uc.Fail(400, "存储ID无效")
			return
		}
	} else {
		bucketID = setting.DefaultStorage
	}

	// 检查游客上传
	if isTouristUsername(c.GetString("username")) {
		if setting.DefaultStorage != bucketID {
			uc.Fail(403, "游客不能上传到非默认存储")
			return
		}
	}

	// 查询存储配置
	var buckets models.Buckets
	if err := db.DB.Where("id = ?", bucketID).First(&buckets).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			uc.Fail(400, "存储配置不存在")
			return
		}
		uc.Fail(500, "存储配置查询失败：%v", err)
		return
	}

	// 解析并校验上传文件
	files, err := uc.ParseAndValidateFiles()
	if err != nil {
		uc.Fail(400, "文件解析失败")
		return
	}

	// 获取文件大小
	var filesize uint64
	if buckets.Id != 1 && buckets.Type != "telegram" {
		for _, file := range files {
			filesize += uint64(file.Size)
		}
		if (buckets.Usage + filesize) >= buckets.Capacity {
			uc.Fail(400, "存储空间已满, 请切换存储")
			return
		}
	}

	// 获取存储上传器
	uploader, err := uc.GetStorageUploader(&setting, &buckets)
	if err != nil {
		uc.Fail(400, "%s", err.Error())
		return
	}

	// 获取全局配置
	cfg, ok := c.MustGet("config").(*config.Config)
	if !ok {
		uc.Fail(500, "全局配置获取失败")
		return
	}

	// 批量处理文件上传（参数匹配接口定义）
	uploadResults := make([]interfaces.ImageUploadResult, 0, len(files))
	successCount := 0

	for _, file := range files {
		fileResult, err := uploader.Upload(c, cfg, &setting, &buckets, file)
		if err != nil {
			// 单个文件上传失败不影响其他文件
			uc.Fail(500, "文件[%s]上传失败：%v", file.Filename, err)
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
			BucketId:  bucketID,
			UserId:    c.GetInt("user_id"),
			MD5:       md5.Md5(c.GetString("username") + fileResult.FileName),
			UUID:      GetUUID(c),
		}

		if db != nil {
			db.DB.Create(&imageModel)
		}

		// 保存文件大小至存储
		if fileResult.Storage != "default" {
			fileSizeUint := uint64(fileResult.FileSize)
			result := db.DB.Model(&models.Buckets{}).
				Where("id = ? AND (usage + ? <= capacity OR type IN ('telegram','default') OR capacity = 0)", bucketID, fileSizeUint).
				UpdateColumn("usage", gorm.Expr("usage + ?", fileSizeUint))
			if result.Error != nil {
				log.Printf("更新Usage失败：%v", result.Error)
			}
			if result.RowsAffected == 0 {
				log.Printf("更新Usage无生效，原因：1.桶ID不存在 2.usage+文件大小>容量 3.数据无变更")
			}
		}

		// 上传时关联图片标签
		if len(existingTags) > 0 {
			var imageTagRelations []models.ImageToTags
			for _, tag := range existingTags {
				imageTagRelations = append(imageTagRelations, models.ImageToTags{
					ImageId: imageModel.Id,
					TagId:   tag.Id,
				})
			}

			db.DB.Create(&imageTagRelations)
		}

		uploadResults = append(uploadResults, *fileResult)

		if setting.TGNotice {
			placeholderData := telegram.PlaceholderData{
				Username:    c.GetString("username"),
				Date:        time.Now().Format("2006-01-02 15:04:05"),
				Filename:    fileResult.FileName,
				StorageType: buckets.Type,
				URL:         c.Request.Host + fileResult.URL,
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

	// 返回上传结果
	uc.Success("上传成功", map[string]any{
		"files": uploadResults,
		"count": successCount,
	})
}

// UploadImage 单文件上传
func UploadImage(c *gin.Context) {
	UploadImages(c)
}

func AddImageTag(c *gin.Context) {
	// 获取请求参数
	type TagRequest struct {
		Id  int    `json:"id"`  // 图片ID
		Tag string `json:"tag"` // 标签ID（前端传字符串，后端转换）
	}

	var req TagRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, result.Error(400, "参数解析失败："+err.Error()))
		return
	}

	// 参数非空校验
	if req.Id <= 0 || req.Tag == "" {
		c.JSON(http.StatusBadRequest, result.Error(400, "参数错误"))
		return
	}

	// 转换并校验图片ID
	tagId, err := strconv.Atoi(req.Tag)
	if err != nil || tagId <= 0 {
		c.JSON(http.StatusBadRequest, result.Error(400, "标签ID无效"))
		return
	}
	imageId := req.Id

	// 获取数据库连接
	db := database.GetDB().DB

	// 查询图片是否存在
	var image models.Image
	if err := db.Where("id = ?", imageId).First(&image).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusBadRequest, result.Error(400, "图片不存在"))
		} else {
			c.JSON(http.StatusInternalServerError, result.Error(500, "查询图片失败："+err.Error()))
		}
		return
	}

	// 查询标签是否存在
	var tag models.Tags
	if err := db.Where("id = ?", tagId).First(&tag).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusBadRequest, result.Error(400, "标签不存在"))
		} else {
			c.JSON(http.StatusInternalServerError, result.Error(500, "查询标签失败："+err.Error()))
		}
		return
	}

	// 查询图片是否已经添加过该标签
	var imageTag models.ImageToTags
	if err := db.Where("image_id = ? AND tag_id = ?", imageId, tagId).First(&imageTag).Error; err == nil {
		c.JSON(http.StatusBadRequest, result.Error(400, "图片已添加过该标签"))
		return
	} else if err != gorm.ErrRecordNotFound {
		c.JSON(http.StatusInternalServerError, result.Error(500, "检查标签关联失败："+err.Error()))
		return
	}

	// 添加图片标签关联
	if err := db.Create(&models.ImageToTags{
		ImageId: imageId,
		TagId:   tagId,
	}).Error; err != nil {
		c.JSON(http.StatusInternalServerError, result.Error(500, "添加标签失败："+err.Error()))
		return
	}

	c.JSON(http.StatusOK, result.Success("标签添加成功", nil))
}

func DeleteImageTag(c *gin.Context) {
	// 获取请求参数
	type TagRequest struct {
		Id  int `json:"id"`  // 图片ID
		Tag int `json:"tag"` // 标签ID
	}

	var req TagRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, result.Error(400, "参数解析失败："+err.Error()))
		return
	}

	// 参数非空校验
	if req.Id <= 0 || req.Tag <= 0 {
		c.JSON(http.StatusBadRequest, result.Error(400, "参数错误"))
		return
	}

	// 转换并校验图片ID
	tagId := req.Tag
	imageId := req.Id

	// 获取数据库连接
	db := database.GetDB().DB

	// 查询图片是否存在
	var image models.Image
	if err := db.Where("id = ?", imageId).First(&image).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusBadRequest, result.Error(400, "图片不存在"))
		} else {
			c.JSON(http.StatusInternalServerError, result.Error(500, "查询图片失败："+err.Error()))
		}
		return
	}

	// 检查标签是否已经添加过该图片
	var imageTag models.ImageToTags
	if err := db.Where("image_id = ? AND tag_id = ?", imageId, tagId).First(&imageTag).Error; err != nil {
		c.JSON(http.StatusBadRequest, result.Error(400, "关联不存在"))
		return
	}

	// 删除图片标签关联
	if err := db.Delete(&imageTag).Error; err != nil {
		c.JSON(http.StatusInternalServerError, result.Error(500, "删除标签失败："+err.Error()))
		return
	}

	c.JSON(http.StatusOK, result.Success("标签删除成功", nil))
}

// 批量删除tag
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
	if err != nil {
		c.JSON(http.StatusBadRequest, result.Error(400, "tag_id必须是有效数字"))
		return
	}

	if len(req.Images) <= 0 || tagID <= 0 {
		c.JSON(http.StatusBadRequest, result.Error(400, "参数错误"))
		return
	}

	// 直接执行删除操作，不返回结果
	db := database.GetDB().DB

	for _, imageId := range req.Images {
		db.Where("image_id = ? AND tag_id = ?", imageId, tagID).Delete(&models.ImageToTags{})
	}

	c.JSON(http.StatusOK, result.Success("批量删除标签成功", nil))
}

// 批量添加tag
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
	if err != nil {
		c.JSON(http.StatusBadRequest, result.Error(400, "tag_id必须是有效数字"))
		return
	}

	if len(req.Images) <= 0 || tagID <= 0 {
		c.JSON(http.StatusBadRequest, result.Error(400, "参数错误"))
		return
	}

	db := database.GetDB().DB

	// 检查标签是否存在
	var tag models.Tags
	if err := db.Where("id = ?", tagID).First(&tag).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusBadRequest, result.Error(400, "标签不存在"))
		} else {
			c.JSON(http.StatusInternalServerError, result.Error(500, "查询标签失败："+err.Error()))
		}
		return
	}

	var existImageIDs []int
	if err := db.Model(&models.Image{}).Where("id IN (?)", req.Images).Pluck("id", &existImageIDs).Error; err != nil {
		c.JSON(http.StatusInternalServerError, result.Error(500, "查询图片列表失败："+err.Error()))
		return
	}

	var existRelations []int
	if err := db.Model(&models.ImageToTags{}).
		Where("image_id IN (?) AND tag_id = ?", req.Images, tagID).
		Pluck("image_id", &existRelations).Error; err != nil {
		c.JSON(http.StatusInternalServerError, result.Error(500, "检查标签关联失败："+err.Error()))
		return
	}

	var insertData []models.ImageToTags
	existRelationMap := make(map[int]bool)
	for _, id := range existRelations {
		existRelationMap[id] = true
	}
	for _, imageID := range req.Images {
		if existRelationMap[imageID] {
			continue
		}
		insertData = append(insertData, models.ImageToTags{
			ImageId: imageID,
			TagId:   tagID,
		})
	}

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
		c.JSON(http.StatusOK, result.Success("没有需要添加的标签", nil))
		return
	}

	c.JSON(http.StatusOK, result.Success("批量添加标签成功", nil))
}

// 获取上传配置
func GetUploadConfig(c *gin.Context) {
	var tags []models.Tags
	var buckets []models.Buckets

	db := database.GetDB().DB
	query := db.Model(&models.Tags{})
	// 获取标签列表
	if err := query.Find(&tags).Error; err != nil {
		c.JSON(http.StatusInternalServerError, result.Error(500, "获取标签列表失败"))
		return
	}

	if err := db.Model(&models.Buckets{}).Find(&buckets).Error; err != nil {
		c.JSON(http.StatusInternalServerError, result.Error(500, "获取存储桶列表失败"))
		return
	}

	var bucketRes []map[string]any
	for _, bucket := range buckets {
		// 过滤已满的存储桶
		if bucket.Capacity > 0 && bucket.Usage >= bucket.Capacity {
			continue
		}
		res := map[string]any{
			"id":   bucket.Id,
			"name": bucket.Name,
			"type": bucket.Type,
		}
		bucketRes = append(bucketRes, res)
	}

	setting, _ := settings.GetSettings()

	// 构造返回参数
	config := map[string]any{
		"buckets":        bucketRes,
		"tags":           tags,
		"default_bucket": setting.DefaultStorage,
	}

	c.JSON(http.StatusOK, result.Success("ok", config))
}
