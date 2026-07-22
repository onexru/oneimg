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

// DeleteImage 删除图片：先删各存储物理副本，再事务释放容量并删库记录。
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
	if err := db.First(&image, uint(id)).Error; err != nil {
		c.JSON(http.StatusNotFound, result.Error(404, "图片不存在"))
		return
	}

	if !CheckImageAccessPermission(c, image, "image:delete") {
		c.JSON(http.StatusForbidden, result.Error(403, "无权访问或删除此图片"))
		return
	}

	deleteCtx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Minute)
	defer cancel()

	if err := services.DeleteImageReplicas(deleteCtx, image); err != nil {
		log.Printf("删除图片 %d 的存储副本失败：%v", image.Id, err)
		c.JSON(http.StatusBadGateway, result.Error(502, "部分存储源删除失败，文件记录已保留，可稍后重试"))
		return
	}

	fileSize := uint64(image.FileSize)
	err = db.Transaction(func(tx *gorm.DB) error {
		var storageList []models.ImageStorage
		if err := tx.Where("image_id = ?", image.Id).Find(&storageList).Error; err != nil {
			return err
		}

		for _, storage := range storageList {
			err := tx.Model(&models.Buckets{}).
				Where("id = ?", storage.BucketID).
				UpdateColumn("usage", gorm.Expr("GREATEST(usage - ?, 0)", fileSize)).Error
			if err != nil {
				log.Printf("Bucket %d 扣减容量失败 size=%d err=%v", storage.BucketID, fileSize, err)
				return err
			}
		}

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

// CheckImageAccessPermission 校验当前用户是否可操作目标图片。
// 规则：超管图片仅超管可动；本人或超管放行；否则需 requiredPerm；游客用 UUID+MD5。
func CheckImageAccessPermission(c *gin.Context, image models.Image, requiredPerm string) bool {
	userId := c.GetInt("user_id")
	userRole := c.GetInt("user_role")

	if image.UserId == models.SuperAdminID && userId != models.SuperAdminID {
		return false
	}
	if (userId > 0 && userId == image.UserId) || userId == models.SuperAdminID {
		return true
	}
	if userId != image.UserId && userRole != models.RoleAdmin {
		return false
	}

	user, exists := middlewares.GetCurrentUser(c)
	if exists && requiredPerm != "" && user.Permission.HasPermission(requiredPerm) {
		return true
	}

	currentUserUUID := GetUUID(c)
	currentUsername := c.GetString("username")
	if image.UUID != "" && image.UUID == currentUserUUID && md5.Md5(currentUsername+image.FileName) == image.MD5 {
		return true
	}
	return false
}

// deleteLocalFile 删除本地 uploads 下相对路径文件（忽略不存在）。
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

// DeleteDefaultStorageImage 删除本地默认存储上的原图与缩略图。
func DeleteDefaultStorageImage(image models.Image) bool {
	deleteLocalFile(image.Url)
	deleteLocalFile(image.Thumbnail)
	return true
}

// deleteS3Object 删除 S3/R2 单个对象。
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

// DeleteR2StorageImage 删除 R2 上的原图与缩略图。
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

// DeleteS3StorageImage 删除 S3 上的原图与缩略图。
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

// DeleteWebDavStorageImage 删除 WebDAV 上的原图与缩略图。
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

// DeleteFtpStorageImage 删除 FTP 上的原图与缩略图。
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

// DeleteTelegramStorageImage 删除 Telegram 消息中的图片与缩略图。
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
