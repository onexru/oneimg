package controllers

import (
	"net/http"
	"os"
	"path/filepath"
	"strconv"

	"oneimg/backend/config"
	"oneimg/backend/database"
	"oneimg/backend/models"

	"github.com/gin-gonic/gin"
)

// DeleteImage 删除图片
func DeleteImage(c *gin.Context) {
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

	// 查询图片信息
	if err := db.First(&image, uint(id)).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"code": 404,
			"msg":  "图片不存在",
		})
		return
	}

	// 获取配置
	cfg, exists := c.Get("config")
	if !exists {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code": 500,
			"msg":  "获取配置失败",
		})
		return
	}
	config := cfg.(*config.Config)

	// 从URL中提取文件路径
	// URL格式: /uploads/2025/09/filename.ext
	// 需要去掉前缀 "/uploads/" 得到相对路径
	relativePath := image.Url
	if len(relativePath) > 9 && relativePath[:9] == "/uploads/" {
		relativePath = relativePath[9:] // 去掉 "/uploads/" 前缀
	}

	// 构建完整文件路径
	filePath := filepath.Join(config.UploadPath, relativePath)

	// 删除物理文件
	if err := os.Remove(filePath); err != nil {
		// 文件可能已经不存在，记录日志但不阻止删除数据库记录
		// log.Printf("删除文件失败: %v", err)
	}

	// 删除数据库记录
	if err := db.Delete(&image).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code": 500,
			"msg":  "删除图片记录失败",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code": 200,
		"msg":  "删除图片成功",
	})
}
