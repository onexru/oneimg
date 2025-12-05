package controllers

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"oneimg/backend/database"
	"oneimg/backend/models"
	"oneimg/backend/utils/result"
	"oneimg/backend/utils/s3"
	"oneimg/backend/utils/settings"
	"oneimg/backend/utils/webdav"

	"github.com/aws/aws-sdk-go-v2/aws"
	awss3 "github.com/aws/aws-sdk-go-v2/service/s3"
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
		c.JSON(http.StatusBadRequest, result.Error(
			400,
			"图片ID无效",
		))
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

	var deleteStatus bool
	// 检查存储
	switch image.Storage {
	case "default":
		deleteStatus = DeleteDefaultStorageImage(image)
	case "s3":
		deleteStatus = DeleteS3StorageImage(image)
	case "r2":
		deleteStatus = DeleteS3StorageImage(image)
	case "webdav":
		deleteStatus = DeleteWebDavStorageImage(image)
	}

	// 删除数据库记录
	if err := db.Delete(&image).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code": 500,
			"msg":  "删除图片记录失败",
		})
		return
	}

	if !deleteStatus {
		c.JSON(http.StatusOK, result.Success(
			"记录删除成功,物理删除失败",
			nil,
		))
		return
	}

	c.JSON(http.StatusOK, result.Success("删除成功", nil))
}

// 删除默认存储的图片
func DeleteDefaultStorageImage(image models.Image) (deleteStatus bool) {
	relativePath := image.Url
	if len(relativePath) > 9 && relativePath[:9] == "/uploads/" {
		relativePath = relativePath[9:] // 去掉 "/uploads/" 前缀
	}
	// 构建完整文件路径
	filePath := filepath.Join("./uploads", relativePath)
	// 删除物理文件
	if err := os.Remove(filePath); err != nil {
		// 文件可能已经不存在，记录日志但不阻止删除数据库记录
	}
	// 检查是否存在缩略图
	if image.Thumbnail != "" {
		relativePath = image.Thumbnail
		if len(relativePath) > 9 && relativePath[:9] == "/uploads/" {
			relativePath = relativePath[9:] // 去掉 "/uploads/" 前缀
		}
		filePath = filepath.Join("./uploads", relativePath)
		if err := os.Remove(filePath); err != nil {
			// 文件可能已经不存在，记录日志但不阻止删除数据库记录
		}
	}
	return true
}

// 删除S3存储的图片
func DeleteS3StorageImage(image models.Image) (deleteStatus bool) {
	// 获取系统配置
	setting, err := settings.GetSettings()
	if err != nil {
		return false
	}
	// 获取S3客户端
	s3Client, err := s3.NewS3Client(setting)
	if err != nil {
		return false
	}
	objectKey := strings.TrimPrefix(image.Url, "/")
	bucket := setting.S3Bucket
	if bucket == "" || objectKey == "" {
		return false
	}

	// 构建删除请求
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	_, err = s3Client.DeleteObject(ctx, &awss3.DeleteObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(objectKey),
	})

	// 检查是否存在缩略图
	if image.Thumbnail != "" {
		objectKey = strings.TrimPrefix(image.Thumbnail, "/")
		_, err = s3Client.DeleteObject(ctx, &awss3.DeleteObjectInput{
			Bucket: aws.String(bucket),
			Key:    aws.String(objectKey),
		})
	}

	if err != nil {
		return !true
	}

	return true
}

// 删除WebDAV存储的图片
func DeleteWebDavStorageImage(image models.Image) (deleteStatus bool) {
	// 获取系统配置
	setting, err := settings.GetSettings()
	if err != nil {
		return false
	}
	// 获取WebDav客户端
	client := webdav.Client(webdav.Config{
		BaseURL:  setting.WebdavURL,
		Username: setting.WebdavUser,
		Password: setting.WebdavPass,
		Timeout:  30 * time.Second,
	})

	var deleteFile = func(filePath string) bool {
		if filePath == "" {
			return false
		}
		err := client.WebDAVDelete(context.TODO(), filePath)
		if err != nil {
			return !true
		}
		return true
	}

	// 检查是否存在缩略图
	if image.Thumbnail != "" {
		deleteFile(image.Thumbnail)
	}
	return deleteFile(image.Url)
}
