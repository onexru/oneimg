package controllers

import (
	"net/http"
	"strconv"

	"oneimg/backend/database"
	"oneimg/backend/models"
	"oneimg/backend/utils/result"
	"oneimg/backend/utils/settings"

	"github.com/gin-gonic/gin"
)

// GetImageDetail 获取单张图片详情（含存储同步状态）。
func GetImageDetail(c *gin.Context) {
	idStr := c.Param("id")
	if idStr == "" {
		c.JSON(http.StatusBadRequest, result.Error(400, "图片ID不能为空"))
		return
	}

	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, result.Error(400, "无效的图片ID"))
		return
	}

	db := database.GetDB().DB
	var image models.Image
	if err := db.First(&image, uint(id)).Error; err != nil {
		c.JSON(http.StatusNotFound, result.Error(404, "图片不存在"))
		return
	}

	if !CheckImageAccessPermission(c, image, "") {
		c.JSON(http.StatusForbidden, result.Error(403, "无权查看此图片"))
		return
	}

	setting, err := settings.GetSettings()
	if err != nil {
		c.JSON(http.StatusInternalServerError, result.Error(500, "获取系统配置失败"))
		return
	}
	rewriteImageURLs(setting, &image)

	statusMap, err := loadImageStorageStatuses([]int{image.Id}, setting)
	if err != nil {
		c.JSON(http.StatusInternalServerError, result.Error(500, "获取存储同步状态失败"))
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code": 200,
		"msg":  "获取图片详情成功",
		"data": struct {
			models.Image
			StorageStatuses []ImageStorageStatusResponse `json:"storage_statuses"`
		}{
			Image:           image,
			StorageStatuses: statusMap[image.Id],
		},
	})
}
