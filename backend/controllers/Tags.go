package controllers

import (
	"net/http"
	"strconv"
	"unicode/utf8"

	"oneimg/backend/database"
	"oneimg/backend/models"
	"oneimg/backend/utils/result"

	"github.com/gin-gonic/gin"
)

func AddTag(c *gin.Context) {
	var tag models.Tags
	if err := c.ShouldBindJSON(&tag); err != nil {
		c.JSON(http.StatusBadRequest, result.Error(400, "参数错误"))
		return
	}

	if tag.Name == "" {
		c.JSON(http.StatusBadRequest, result.Error(400, "标签名称不能为空"))
		return
	}

	if utf8.RuneCountInString(tag.Name) > 10 {
		c.JSON(http.StatusBadRequest, result.Error(400, "标签名称过长"))
		return
	}

	db := database.GetDB().DB

	// 查询标签是否存在
	var count int64
	if err := db.Model(&models.Tags{}).Where("name = ?", tag.Name).Count(&count).Error; err != nil {
		c.JSON(http.StatusInternalServerError, result.Error(500, "查询标签失败"))
		return
	}

	// 标签已存在
	if count > 0 {
		c.JSON(http.StatusConflict, result.Error(409, "标签已存在"))
		return
	}

	// 插入标签
	if err := db.Create(&tag).Error; err != nil {
		c.JSON(http.StatusInternalServerError, result.Error(500, "插入标签失败"))
		return
	}

	c.JSON(http.StatusOK, result.Success("ok", tag))
}

func GetTags(c *gin.Context) {
	var tags []models.Tags
	var total int64

	db := database.GetDB().DB

	query := db.Model(&models.Tags{})

	// 获取总数
	if err := query.Count(&total).Error; err != nil {
		c.JSON(http.StatusInternalServerError, result.Error(500, "获取标签总数失败"))
		return
	}

	// 获取标签列表
	if err := query.Find(&tags).Error; err != nil {
		c.JSON(http.StatusInternalServerError, result.Error(500, "获取标签列表失败"))
		return
	}

	c.JSON(http.StatusOK, result.Success("ok", map[string]any{
		"total": total,
		"list":  tags,
	}))
}

func DeleteTag(c *gin.Context) {
	idStr := c.Param("id")
	if idStr == "" {
		c.JSON(http.StatusBadRequest, result.Error(400, "标签ID不能为空"))
		return
	}

	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, result.Error(400, "标签ID无效"))
		return
	}

	if id == 0 {
		c.JSON(http.StatusForbidden, result.Error(403, "默认标签不能删除"))
		return
	}

	db := database.GetDB().DB
	var tag models.Tags

	// 查询标签信息
	if err := db.First(&tag, uint(id)).Error; err != nil {
		c.JSON(http.StatusNotFound, result.Error(404, "标签不存在"))
		return
	}

	// 删除图片关联
	if err := db.Where("tag_id = ?", id).Delete(&models.ImageToTags{}).Error; err != nil {
		c.JSON(http.StatusInternalServerError, result.Error(500, "删除标签关联失败"))
		return
	}

	// 标签存在且用户有权限
	if err := db.Delete(&tag).Error; err != nil {
		c.JSON(http.StatusInternalServerError, result.Error(500, "删除标签失败"))
		return
	}

	c.JSON(http.StatusOK, result.Success("ok", nil))
}
