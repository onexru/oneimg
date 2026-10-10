package controllers
import (
 "net/http"
 "strconv"
 "oneimg/backend/database"
 "oneimg/backend/models"
 "oneimg/backend/utils/result"
 "github.com/gin-gonic/gin"
 "gorm.io/gorm"
)
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
			internalFailure(c, "查询图片失败", err)
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
			internalFailure(c, "查询标签失败", err)
		}
		return
	}

	// 4. 查询图片是否已经添加过该标签
	var imageTag models.ImageToTags
	if err := db.Where("image_id = ? AND tag_id = ?", imageId, tagId).First(&imageTag).Error; err == nil {
		c.JSON(http.StatusBadRequest, result.Error(400, "图片已添加过该标签"))
		return
	} else if err != gorm.ErrRecordNotFound {
		internalFailure(c, "检查标签关联失败", err)
		return
	}

	// 5. 添加图片标签关联
	if err := db.Create(&models.ImageToTags{
		ImageId: imageId,
		TagId:   tagId,
	}).Error; err != nil {
		internalFailure(c, "添加标签失败", err)
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
			internalFailure(c, "查询图片失败", err)
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
		internalFailure(c, "删除标签失败", err)
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
		internalFailure(c, "批量删除标签失败", err)
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
			internalFailure(c, "查询标签失败", err)
		}
		return
	}

	// 查询并校验图片列表权限
	var images []models.Image
	if err := db.Where("id IN ?", req.Images).Find(&images).Error; err != nil {
		internalFailure(c, "查询图片列表失败", err)
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
		internalFailure(c, "检查标签关联失败", err)
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
			internalFailure(c, "批量添加标签失败", err)
			return
		}
	} else {
		c.JSON(http.StatusOK, result.Success("没有需要添加的标签(可能已全部存在)", nil))
		return
	}

	c.JSON(http.StatusOK, result.Success("批量添加标签成功", nil))
}
