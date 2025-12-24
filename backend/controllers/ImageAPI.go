package controllers

import (
	"math/rand"
	"net/http"
	"oneimg/backend/database"
	"oneimg/backend/models"
	"oneimg/backend/utils/result"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// 初始化随机数种子
var randomGenerator = rand.New(rand.NewSource(time.Now().UnixNano()))

// 定义返回的图片结构体
type RandomImageResponse struct {
	Image string `json:"image"` // 图片文件名
	Url   string `json:"url"`   // 图片完整访问地址
}

func GetRandomImages(c *gin.Context) {
	// 解析并校验参数
	tag := c.Query("tag")
	model := c.DefaultQuery("model", "json")
	limitStr := c.DefaultQuery("limit", "1")
	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit < 1 || limit > 20 {
		c.JSON(http.StatusBadRequest, result.Error(400, "limit参数错误，必须是1-20之间的整数"))
		return
	}

	// 初始化数据库连接
	db := database.GetDB().DB
	var images []models.Image
	var total int64
	var respData []RandomImageResponse

	if tag == "" {
		// 统计总图片数
		if err := db.Model(&models.Image{}).Count(&total).Error; err != nil {
			c.JSON(http.StatusInternalServerError, result.Error(500, "获取图片总数失败"))
			return
		}

		// 无图片时返回空数据
		if total == 0 {
			c.JSON(http.StatusOK, result.Success("ok", []RandomImageResponse{}))
			return
		}

		// 计算随机偏移量
		var offset int
		if total > int64(limit) {
			offset = randomGenerator.Intn(int(total) - limit + 1)
		} else {
			offset = 0
		}

		// 查询随机图片
		if err := db.Model(&models.Image{}).
			Offset(offset).
			Limit(limit).
			Find(&images).Error; err != nil {
			c.JSON(http.StatusInternalServerError, result.Error(500, "获取图片失败"))
			return
		}
	} else {
		// 统计该标签下的图片数
		if err := db.Model(&models.Image{}).
			Joins("JOIN image_to_tags ON images.id = image_to_tags.image_id").
			Joins("JOIN tags ON image_to_tags.tag_id = tags.id").
			Where("tags.name = ?", tag).
			Count(&total).Error; err != nil {
			c.JSON(http.StatusInternalServerError, result.Error(500, "获取标签图片总数失败"))
			return
		}

		// 无该标签图片时返回空数据
		if total == 0 {
			c.JSON(http.StatusOK, result.Success("ok", []RandomImageResponse{}))
			return
		}

		// 计算随机偏移量
		var offset int
		if total > int64(limit) {
			offset = randomGenerator.Intn(int(total) - limit + 1)
		} else {
			offset = 0
		}

		// 查询该标签下的随机图片
		if err := db.Model(&models.Image{}).
			Joins("JOIN image_to_tags ON images.id = image_to_tags.image_id").
			Joins("JOIN tags ON image_to_tags.tag_id = tags.id").
			Where("tags.name = ?", tag).
			Offset(offset).
			Limit(limit).
			Find(&images).Error; err != nil {
			c.JSON(http.StatusInternalServerError, result.Error(500, "获取标签图片失败"))
			return
		}
	}

	if model == "image" {
		imageUrl := strings.ReplaceAll(images[0].Url, "/uploads/", "/")
		c.AddParam("path", imageUrl)
		ImageProxy(c)
		return
	}
	for _, img := range images {
		fullUrl := getFullImageUrl(c, img.Url)
		respData = append(respData, RandomImageResponse{
			Image: img.FileName,
			Url:   fullUrl,
		})
	}

	c.JSON(http.StatusOK, result.Success("ok", respData))
}

func getFullImageUrl(c *gin.Context, path string) string {
	var scheme string
	if proto := c.GetHeader("X-Forwarded-Proto"); proto != "" {
		scheme = proto
	} else if c.Request.TLS != nil {
		scheme = "https"
	} else {
		scheme = "http"
	}
	var host string
	if forwardedHost := c.GetHeader("X-Forwarded-Host"); forwardedHost != "" {
		host = forwardedHost
	} else {
		host = c.Request.Host
	}
	baseUrl := scheme + "://" + host
	baseUrl = strings.TrimSuffix(baseUrl, "/")
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	fullUrl := baseUrl + path
	return fullUrl
}
