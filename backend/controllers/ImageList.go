package controllers

import (
	"net/http"
	"strconv"

	"oneimg/backend/database"
	"oneimg/backend/models"
	"oneimg/backend/utils/result"

	"github.com/gin-gonic/gin"
)

// GetImageList 获取图片列表
func GetImageList(c *gin.Context) {
	// 获取分页参数
	pageStr := c.DefaultQuery("page", "1")
	limitStr := c.DefaultQuery("limit", "20")

	page, err := strconv.Atoi(pageStr)
	if err != nil || page < 1 {
		page = 1
	}

	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit < 1 || limit > 100 {
		limit = 20
	}

	// 获取排序参数
	sortBy := c.DefaultQuery("sort_by", "created_at")
	sortOrder := c.DefaultQuery("sort_order", "desc")

	// 获取搜索参数
	search := c.Query("search")

	// 计算偏移量
	offset := (page - 1) * limit

	db := database.GetDB().DB

	var images []models.Image
	var total int64

	// 构建查询
	query := db.Model(&models.Image{})

	// 获取角色参数
	role := c.Query("role")
	if role != "" {
		switch role {
		case "admin":
			query = query.Where("user_id == 1")
		case "guest":
			query = query.Where("user_id != 1")
		}
	}

	if c.GetInt("user_role") != 1 || role == "" {
		query = query.Where("uuid = ?", GetUUID(c))
	}

	// 添加搜索条件
	if search != "" {
		query = query.Where("file_name LIKE ?", "%"+search+"%")
	}

	// 获取总数
	if err := query.Count(&total).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code": 500,
			"msg":  "获取图片总数失败",
		})
		return
	}

	// 验证排序字段
	validSortFields := map[string]bool{
		"created_at": true,
		"file_size":  true,
		"filename":   true,
	}

	if !validSortFields[sortBy] {
		sortBy = "created_at"
	}

	if sortOrder != "asc" && sortOrder != "desc" {
		sortOrder = "desc"
	}

	// 映射前端字段名到数据库字段名
	fieldMapping := map[string]string{
		"filename":   "file_name",
		"created_at": "created_at",
		"file_size":  "file_size",
	}

	dbField := fieldMapping[sortBy]
	if dbField == "" {
		dbField = "created_at"
	}

	// 获取图片列表
	orderClause := dbField + " " + sortOrder
	if err := query.Order(orderClause).Offset(offset).Limit(limit).Find(&images).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code": 500,
			"msg":  "获取图片列表失败",
		})
		return
	}

	// 计算总页数
	totalPages := (total + int64(limit) - 1) / int64(limit)

	c.JSON(http.StatusOK, result.Success("获取图片列表成功", gin.H{
		"images":      images,
		"total":       total,
		"page":        page,
		"limit":       limit,
		"total_pages": totalPages,
	}))
}
