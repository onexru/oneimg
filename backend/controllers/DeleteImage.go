package controllers

import (
	"context"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"oneimg/backend/database"
	"oneimg/backend/middlewares"
	"oneimg/backend/models"
	"oneimg/backend/services"
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
	idStr := c.Param("id")
	if idStr == "" {
		c.JSON(http.StatusBadRequest, result.Error(400, "图片ID不能为空"))
		return
	}

	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, result.Error(400, "图片ID无效"))
		return
	}

	db := database.GetDB().DB
	var image models.Image

	// 查询图片信息
	if err := db.First(&image, uint(id)).Error; err != nil {
		c.JSON(http.StatusNotFound, result.Error(404, "图片不存在"))
		return
	}

	// 集中、统一的权限校验
	if !CheckImageAccessPermission(c, image, "image:delete") {
		c.JSON(http.StatusForbidden, result.Error(403, "无权访问或删除此图片"))
		return
	}

	deleteCtx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Minute)
	defer cancel()

	// 先删除所有云端/本地物理文件
	if err := services.DeleteImageReplicas(deleteCtx, image); err != nil {
		log.Printf("删除图片 %d 的存储副本失败：%v", image.Id, err)
		c.JSON(http.StatusBadGateway, result.Error(502, "部分存储源删除失败，文件记录已保留，可稍后重试"))
		return
	}

	fileSize := uint64(image.FileSize)
	err = db.Transaction(func(tx *gorm.DB) error {
		// 查出该图片全部存储副本记录
		var storageList []models.ImageStorage
		if err := tx.Where("image_id = ?", image.Id).Find(&storageList).Error; err != nil {
			return err
		}

		// 遍历所有副本，每个对应Bucket扣减容量（usage - size，最小0）
		for _, storage := range storageList {
			err := tx.Model(&models.Buckets{}).
				Where("id = ?", storage.BucketID).
				UpdateColumn("usage", gorm.Expr("GREATEST(usage - ?, 0)", fileSize)).Error
			if err != nil {
				log.Printf("Bucket %d 扣减容量失败 size=%d err=%v", storage.BucketID, fileSize, err)
				return err
			}
		}

		// 删除关联表与主记录
		if err := tx.Where("image_id = ?", image.Id).Delete(&models.ImageStorage{}).Error; err != nil {
			return err
		}
		if err := tx.Where("image_id = ?", image.Id).Delete(&models.ImageToTags{}).Error; err != nil {
			return err
		}
		if err := tx.Delete(&image).Error; err != nil {
			return err
		}
		return nil
	})

	if err != nil {
		log.Printf("图片 %d 事务处理失败 err=%v", image.Id, err)
		c.JSON(http.StatusInternalServerError, result.Error(500, "删除图片记录或更新存储容量失败"))
		return
	}

	c.JSON(http.StatusOK, result.Success("删除成功，对应存储容量已释放", nil))
}

// 辅助函数：统一权限校验
func CheckImageAccessPermission(c *gin.Context, image models.Image, requiredPerm string) bool {
	userId := c.GetInt("user_id")

	// 超级管理员的图片只能超级管理员自己操作，防止普通管理员越权
	if image.UserId == models.SuperAdminID && userId != models.SuperAdminID {
		return false
	}

	// 本人操作，或者本身是超级管理员，直接放行
	if (userId > 0 && userId == image.UserId) || userId == models.SuperAdminID {
		return true
	}

	// 检查是否有专门的越权操作权限
	user, exists := middlewares.GetCurrentUser(c)
	if exists && requiredPerm != "" && user.Permission.HasPermission(requiredPerm) {
		return true
	}

	// 游客基于 Token/UUID 校验
	currentUserUUID := GetUUID(c)
	currentUsername := c.GetString("username")
	if image.UUID != "" && image.UUID == currentUserUUID && md5.Md5(currentUsername+image.FileName) == image.MD5 {
		return true
	}

	return false
}

func deleteLocalFile(fileUrl string) {
	if fileUrl == "" {
		return
	}
	relPath := strings.TrimPrefix(strings.TrimPrefix(fileUrl, "/"), "uploads/")
	filePath := filepath.Join("./uploads", relPath)
	if err := os.Remove(filePath); err != nil && !os.IsNotExist(err) {
		log.Printf("[本地存储] 删除文件失败 path=%s err=%v", filePath, err)
	}
}

// DeleteDefaultStorageImage 删除默认存储的图片
func DeleteDefaultStorageImage(image models.Image) bool {
	deleteLocalFile(image.Url)
	deleteLocalFile(image.Thumbnail)
	return true
}

func deleteS3Object(ctx context.Context, client *awss3.Client, bucketName, fileUrl string) error {
	if fileUrl == "" {
		return nil
	}
	objectKey := strings.TrimPrefix(fileUrl, "/")
	_, err := client.DeleteObject(ctx, &awss3.DeleteObjectInput{
		Bucket: aws.String(bucketName),
		Key:    aws.String(objectKey),
	})
	return err
}

// DeleteR2StorageImage 删除R2存储的图片
func DeleteR2StorageImage(image models.Image, bucket models.Buckets) bool {
	setting, err := settings.GetSettings()
	if err != nil {
		return false
	}
	s3Client, err := s3.NewS3Client(setting, bucket)
	if err != nil {
		return false
	}

	storageConfig := buckets.ConvertToR2Bucket(bucket.Config)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := deleteS3Object(ctx, s3Client, storageConfig.R2Bucket, image.Url); err != nil {
		log.Printf("[R2] 删除原图失败 bucket=%s key=%s err=%v", storageConfig.R2Bucket, image.Url, err)
		return false
	}

	if err := deleteS3Object(ctx, s3Client, storageConfig.R2Bucket, image.Thumbnail); err != nil {
		log.Printf("[R2] 删除缩略图失败 bucket=%s key=%s err=%v", storageConfig.R2Bucket, image.Thumbnail, err)
		return false
	}
	return true
}

// DeleteS3StorageImage 删除S3存储的图片
func DeleteS3StorageImage(image models.Image, bucket models.Buckets) bool {
	setting, err := settings.GetSettings()
	if err != nil {
		log.Printf("[S3] 获取系统配置失败 bucketId=%d err=%v", bucket.Id, err)
		return false
	}
	s3Client, err := s3.NewS3Client(setting, bucket)
	if err != nil {
		log.Printf("[S3] 创建客户端失败 bucketId=%d err=%v", bucket.Id, err)
		return false
	}

	storageConfig := buckets.ConvertToS3Bucket(bucket.Config)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := deleteS3Object(ctx, s3Client, storageConfig.S3Bucket, image.Url); err != nil {
		log.Printf("[S3] 删除原图失败 bucket=%s key=%s err=%v", storageConfig.S3Bucket, image.Url, err)
		return false
	}

	if err := deleteS3Object(ctx, s3Client, storageConfig.S3Bucket, image.Thumbnail); err != nil {
		log.Printf("[S3] 删除缩略图失败 bucket=%s key=%s err=%v", storageConfig.S3Bucket, image.Thumbnail, err)
		return false
	}
	return true
}

// DeleteWebDavStorageImage 删除WebDAV存储的图片
func DeleteWebDavStorageImage(image models.Image, bucket models.Buckets) bool {
	storageConfig := buckets.ConvertToWebDavBucket(bucket.Config)
	client := webdav.Client(webdav.Config{
		BaseURL:  storageConfig.WebdavURL,
		Username: storageConfig.WebdavUser,
		Password: storageConfig.WebdavPass,
		Timeout:  30 * time.Second,
	})

	deleteFile := func(filePath string) bool {
		if filePath == "" {
			return true
		}
		if err := client.WebDAVDelete(context.TODO(), filePath); err != nil {
			log.Printf("[WebDAV] 删除文件失败 path=%s err=%v", filePath, err)
			return false
		}
		return true
	}

	// 两者都必须成功才算成功
	mainOk := deleteFile(image.Url)
	thumbOk := deleteFile(image.Thumbnail)
	return mainOk && thumbOk
}

// DeleteFtpStorageImage 删除FTP存储的图片
func DeleteFtpStorageImage(image models.Image, bucket models.Buckets) bool {
	storageConfig := buckets.ConvertToFTPBucket(bucket.Config)
	ftpUtil := ftp.NewFTPUtil(ftp.FTPConfig{
		Host:     storageConfig.FTPHost,
		Port:     storageConfig.FTPPort,
		User:     storageConfig.FTPUser,
		Password: storageConfig.FTPPass,
		Timeout:  60,
	})

	if err := ftpUtil.DeleteImage(image.Url); err != nil {
		log.Printf("[FTP] 删除原图失败 bucketId=%d path=%s err=%v", bucket.Id, image.Url, err)
		return false
	}

	if image.Thumbnail != "" {
		if err := ftpUtil.DeleteImage(image.Thumbnail); err != nil {
			log.Printf("[FTP] 删除缩略图失败 bucketId=%d path=%s err=%v", bucket.Id, image.Thumbnail, err)
			return false
		}
	}
	return true
}

// DeleteTelegramStorageImage 删除TG存储的图片
func DeleteTelegramStorageImage(image models.Image, bucket models.Buckets) bool {
	storageConfig := buckets.ConvertToTelegramBucket(bucket.Config)

	dbInstance := database.GetDB()
	if dbInstance == nil || dbInstance.DB == nil {
		log.Printf("[TG] 数据库实例为空，无法查询TG文件记录")
		return false
	}

	var telegramModel models.ImageTeleGram
	if err := dbInstance.DB.Where("file_name = ?", image.FileName).First(&telegramModel).Error; err != nil {
		log.Printf("[TG] 查询TG文件记录失败 fileName=%s err=%v", image.FileName, err)
		return false
	}

	tgClient := telegram.NewClient(storageConfig.TGBotToken)
	tgClient.Timeout = 20 * time.Second
	tgClient.Retry = 3

	uploader := telegram.NewTelegramUploader(tgClient)
	uploader.DeletePhoto(storageConfig.TGReceivers, telegramModel.TGMessageId)

	if image.Thumbnail != "" && telegramModel.TGThumbnailMessageId != 0 {
		uploader.DeletePhoto(storageConfig.TGReceivers, telegramModel.TGThumbnailMessageId)
	}

	return true
}
