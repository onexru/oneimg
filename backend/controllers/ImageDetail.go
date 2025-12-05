package controllers

import (
	"net/http"
	"strconv"

	"oneimg/backend/database"
	"oneimg/backend/models"

	"github.com/gin-gonic/gin"
)

// GetImageDetail 获取图片详情
func GetImageDetail(c *gin.Context) {
	// 获取图片ID参数
	idStr := c.Param("id")
	if idStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"code": 400,
			"msg":  "图片ID不能为空",
		})
		return
	}

	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code": 400,
			"msg":  "无效的图片ID",
		})
		return
	}

	db := database.GetDB().DB
	var image models.Image

	// 查询图片详情
	if err := db.First(&image, uint(id)).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"code": 404,
			"msg":  "图片不存在",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code": 200,
		"msg":  "获取图片详情成功",
		"data": image,
	})
}
