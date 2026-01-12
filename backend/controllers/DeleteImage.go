package controllers

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"oneimg/backend/database"
	"oneimg/backend/models"
	"oneimg/backend/utils/buckets"
	"oneimg/backend/utils/ftp"
	"oneimg/backend/utils/md5"
	"oneimg/backend/utils/result"
	"oneimg/backend/utils/s3"
	"oneimg/backend/utils/settings"
	"oneimg/backend/utils/telegram"
	"oneimg/backend/utils/webdav"

	"github.com/aws/aws-sdk-go-v2/aws"
	awss3 "github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
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

	// 校验权限
	if !CheckImageAccessPermission(c, image) {
		c.JSON(http.StatusForbidden, gin.H{
			"code": 403,
			"msg":  "无权访问",
		})
		return
	}

	// 获取存储配置
	var bucket models.Buckets
	if err := db.Where("id = ?", image.BucketId).First(&bucket).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusForbidden, result.Error(400, "存储配置不存在"))
			return
		}
		c.JSON(http.StatusForbidden, result.Error(400, "存储配置查询失败"))
		return
	}

	var deleteStatus bool
	// 检查存储
	switch image.Storage {
	case "default":
		deleteStatus = DeleteDefaultStorageImage(image)
	case "s3":
		deleteStatus = DeleteS3StorageImage(image, bucket)
	case "r2":
		deleteStatus = DeleteS3StorageImage(image, bucket)
	case "webdav":
		deleteStatus = DeleteWebDavStorageImage(image, bucket)
	case "ftp":
		deleteStatus = DeleteFtpStorageImage(image, bucket)
	case "telegram":
		deleteStatus = DeleteTelegramStorageImage(image, bucket)
	default:
		deleteStatus = false
	}

	// 删除数据库记录
	if err := db.Delete(&image).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code": 500,
			"msg":  "删除图片记录失败",
		})
		return
	}

	// 对应存储减去存储空间
	if image.BucketId != 1 {
		result := db.Model(&models.Buckets{}).
			Where("id = ? AND usage >= ?", image.BucketId, image.FileSize).
			UpdateColumn("usage", gorm.Expr("usage - ?", image.FileSize))
		if result.Error != nil {
			log.Printf("更新Usage失败：%v", result.Error)
		}
	}

	// 删除关联tag
	db.Where("image_id = ?", image.Id).Delete(models.ImageToTags{})

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
func DeleteS3StorageImage(image models.Image, bucket models.Buckets) (deleteStatus bool) {
	// 获取系统配置
	setting, err := settings.GetSettings()
	if err != nil {
		return false
	}
	// 获取S3客户端
	s3Client, err := s3.NewS3Client(setting, bucket)
	if err != nil {
		return false
	}
	objectKey := strings.TrimPrefix(image.Url, "/")

	// 获取存储配置
	storageConfig := buckets.ConvertToS3Bucket(bucket.Config)

	// 弃用
	// bucket := setting.S3Bucket
	if objectKey == "" {
		return false
	}

	// 构建删除请求
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	_, err = s3Client.DeleteObject(ctx, &awss3.DeleteObjectInput{
		Bucket: aws.String(storageConfig.S3Bucket),
		Key:    aws.String(objectKey),
	})

	// 检查是否存在缩略图
	if image.Thumbnail != "" {
		objectKey = strings.TrimPrefix(image.Thumbnail, "/")
		_, err = s3Client.DeleteObject(ctx, &awss3.DeleteObjectInput{
			Bucket: aws.String(storageConfig.S3Bucket),
			Key:    aws.String(objectKey),
		})
	}

	if err != nil {
		return !true
	}

	return true
}

// 删除WebDAV存储的图片
func DeleteWebDavStorageImage(image models.Image, bucket models.Buckets) (deleteStatus bool) {
	// 获取存储配置
	storageConfig := buckets.ConvertToWebDavBucket(bucket.Config)

	// 获取WebDav客户端
	client := webdav.Client(webdav.Config{
		BaseURL:  storageConfig.WebdavURL,
		Username: storageConfig.WebdavUser,
		Password: storageConfig.WebdavPass,
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

// 删除FTP存储的图片
func DeleteFtpStorageImage(image models.Image, bucket models.Buckets) (deleteStatus bool) {
	// 获取存储配置
	storageConfig := buckets.ConvertToFTPBucket(bucket.Config)
	// 初始化FTP客户端
	ftpUtil := ftp.NewFTPUtil(ftp.FTPConfig{
		Host:     storageConfig.FTPHost,
		Port:     storageConfig.FTPPort,
		User:     storageConfig.FTPUser,
		Password: storageConfig.FTPPass,
		Timeout:  60,
	})

	// 删除图片
	if err := ftpUtil.DeleteImage(image.Url); err != nil {
		return !true
	}

	// 检查是否存在缩略图
	if image.Thumbnail != "" {
		// 删除缩略图
		if err := ftpUtil.DeleteImage(image.Thumbnail); err != nil {
			return !true
		}
	}
	return true
}

// 删除TG存储的图片
func DeleteTelegramStorageImage(image models.Image, bucket models.Buckets) (deleteStatus bool) {
	// 获取存储配置
	storageConfig := buckets.ConvertToTelegramBucket(bucket.Config)

	// 查询图片ID
	db := database.GetDB()
	if db == nil {
		// 数据库连接失败忽略错误，防止阻塞线程
	}
	var telegramModel models.ImageTeleGram
	if err := db.DB.Where("file_name = ?", image.FileName).First(&telegramModel).Error; err != nil {
		// 查询失败忽略错误，防止阻塞线程
	}

	tgClient := telegram.NewClient(storageConfig.TGBotToken)
	tgClient.Timeout = 20 * time.Second
	tgClient.Retry = 3

	uploader := telegram.NewTelegramUploader(tgClient)

	// 直接删除，不检查是否成功
	uploader.DeletePhoto(storageConfig.TGReceivers, telegramModel.TGMessageId)

	// 检查是否存在缩略图
	if image.Thumbnail != "" {
		// 删除缩略图，不检查是否成功
		uploader.DeletePhoto(storageConfig.TGReceivers, telegramModel.TGThumbnailMessageId)
	}
	// 直接返回成功
	return true
}

// 辅助函数：权限校验
func CheckImageAccessPermission(c *gin.Context, image models.Image) bool {
	currentUserUUID := GetUUID(c)
	currentUsername := c.GetString("username")
	// 如果是管理员直接通过
	if c.GetInt("user_role") == 1 {
		return true
	}
	// 如果是游客则需要同时满足md5校验和UUID校验
	if image.UUID == currentUserUUID && md5.Md5(currentUsername+image.FileName) == image.MD5 {
		return true
	}
	return false
}
