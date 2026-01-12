package uploads

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"mime/multipart"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awss3 "github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/gin-gonic/gin"

	"oneimg/backend/config"
	"oneimg/backend/database"
	"oneimg/backend/interfaces"
	"oneimg/backend/models"
	"oneimg/backend/utils/buckets"
	"oneimg/backend/utils/ftp"
	"oneimg/backend/utils/images"
	"oneimg/backend/utils/s3"
	"oneimg/backend/utils/telegram"
	"oneimg/backend/utils/webdav"
)

// 所有上传器实现统一接口
type S3R2Uploader struct{}
type WebDAVUploader struct{}
type DefaultUploader struct{}
type FTPUploader struct{}
type TelegramUploader struct{}

// S3/R2上传实现
func (u *S3R2Uploader) Upload(c *gin.Context, cfg *config.Config, setting *models.Settings, bucket *models.Buckets, fileHeader *multipart.FileHeader) (*interfaces.ImageUploadResult, error) {
	// 验证图片
	if err := images.ValidateImageFile(fileHeader, cfg); err != nil {
		return nil, fmt.Errorf("图片验证失败: %v", err)
	}

	// 打开文件
	file, err := fileHeader.Open()
	if err != nil {
		return nil, fmt.Errorf("打开文件失败: %v", err)
	}
	defer file.Close()

	// 处理图片
	processedImage, err := images.ImageSvc.ProcessImage(file, fileHeader, *setting)
	if err != nil {
		return nil, fmt.Errorf("图片处理失败: %v", err)
	}

	uniqueFileName := processedImage.UniqueFileName

	// 创建年/月子目录
	now := time.Now()
	year := now.Format("2006")
	month := now.Format("01")
	subDir := PathJoin("uploads", year, month)
	objectKey := PathJoin(subDir, uniqueFileName)

	// 获取S3/R2客户端
	client, err := s3.NewS3Client(*setting, *bucket)
	if err != nil {
		return nil, fmt.Errorf("创建S3/R2客户端失败：%v", err)
	}

	// 上传文件到S3/R2
	contentType := "image/webp"
	if !setting.SaveWebp {
		contentType = fileHeader.Header.Get("Content-Type")
	}

	// 获取Bucket
	storageConfig := buckets.ConvertToS3Bucket(bucket.Config)
	_, err = client.PutObject(context.TODO(), &awss3.PutObjectInput{
		Bucket:      aws.String(storageConfig.S3Bucket),
		Key:         aws.String(objectKey),
		Body:        bytes.NewReader(processedImage.CompressedBytes),
		ContentType: aws.String(contentType),
	})
	if err != nil {
		return nil, fmt.Errorf("S3/R2上传失败：%v", err)
	}

	thumbnailURL := ""
	// 检查是否上传缩略图
	if setting.Thumbnail {
		_, err = client.PutObject(context.TODO(), &awss3.PutObjectInput{
			Bucket:      aws.String(storageConfig.S3Bucket),
			Key:         aws.String(PathJoin("uploads", year, month, "thumbnails", uniqueFileName)), // 缩略图存放路径
			Body:        bytes.NewReader(processedImage.ThumbnailBytes),
			ContentType: aws.String("image/webp"),
		})
		if err == nil {
			thumbnailURL = "/" + PathJoin("uploads", year, month, "thumbnails", uniqueFileName)
		}
	}

	url := "/" + PathJoin("uploads", year, month, uniqueFileName)

	return &interfaces.ImageUploadResult{
		Success:      true,
		Message:      "上传成功",
		FileName:     uniqueFileName,
		FileSize:     int64(len(processedImage.CompressedBytes)),
		MimeType:     contentType,
		URL:          url,
		ThumbnailURL: thumbnailURL,
		Storage:      bucket.Type,
		CreatedAt:    time.Now().Format("2006-01-02 15:04:05"),
		Width:        processedImage.Width,
		Height:       processedImage.Height,
	}, nil
}

// WebDAV上传实现
func (u *WebDAVUploader) Upload(c *gin.Context, cfg *config.Config, setting *models.Settings, bucket *models.Buckets, fileHeader *multipart.FileHeader) (*interfaces.ImageUploadResult, error) {
	// 验证图片
	if err := images.ValidateImageFile(fileHeader, cfg); err != nil {
		return nil, fmt.Errorf("图片验证失败: %v", err)
	}

	// 打开文件
	file, err := fileHeader.Open()
	if err != nil {
		return nil, fmt.Errorf("打开文件失败: %v", err)
	}
	defer file.Close()

	// 处理图片
	processedImage, err := images.ImageSvc.ProcessImage(file, fileHeader, *setting)
	if err != nil {
		return nil, fmt.Errorf("图片处理失败: %v", err)
	}

	// 生成唯一文件名
	uniqueFileName := processedImage.UniqueFileName

	// 创建年/月子目录
	now := time.Now()
	year := now.Format("2006")
	month := now.Format("01")
	subDir := filepath.Join("uploads", year, month)
	objectPath := filepath.Join("/", subDir, uniqueFileName)

	// 获取存储配置
	storageConfig := buckets.ConvertToWebDavBucket(bucket.Config)

	// 初始化WebDAV客户端
	client := webdav.Client(webdav.Config{
		BaseURL:  storageConfig.WebdavURL,
		Username: storageConfig.WebdavUser,
		Password: storageConfig.WebdavPass,
		Timeout:  30 * time.Second,
	})

	// 上传文件到WebDAV服务器
	err = client.WebDAVUpload(context.TODO(), objectPath, bytes.NewReader(processedImage.CompressedBytes))
	if err != nil {
		return nil, fmt.Errorf("WebDAV上传失败：%v", err)
	}

	// 检查是否上传缩略图
	thumbnailURL := ""
	if setting.Thumbnail {
		err = client.WebDAVUpload(context.TODO(), filepath.Join("/", subDir, "thumbnails", uniqueFileName), bytes.NewReader(processedImage.ThumbnailBytes))
		if err == nil {
			thumbnailURL = "/uploads/" + year + "/" + month + "/thumbnails/" + uniqueFileName
		}
	}

	// 构建访问URL
	url := "/uploads/" + year + "/" + month + "/" + uniqueFileName

	return &interfaces.ImageUploadResult{
		Success:      true,
		Message:      "上传成功",
		FileName:     uniqueFileName,
		FileSize:     int64(len(processedImage.CompressedBytes)),
		MimeType:     processedImage.MimeType,
		URL:          url,
		ThumbnailURL: thumbnailURL,
		Storage:      bucket.Type,
		Width:        processedImage.Width,
		Height:       processedImage.Height,
		CreatedAt:    time.Now().Format("2006-01-02 15:04:05"),
	}, nil
}

// FTP上传实现
func (u *FTPUploader) Upload(c *gin.Context, cfg *config.Config, setting *models.Settings, bucket *models.Buckets, fileHeader *multipart.FileHeader) (*interfaces.ImageUploadResult, error) {
	// 验证图片
	if err := images.ValidateImageFile(fileHeader, cfg); err != nil {
		return nil, fmt.Errorf("图片验证失败: %v", err)
	}

	// 打开文件
	file, err := fileHeader.Open()
	if err != nil {
		return nil, fmt.Errorf("打开文件失败: %v", err)
	}
	defer file.Close()

	// 处理图片（压缩、生成缩略图等）
	processedImage, err := images.ImageSvc.ProcessImage(file, fileHeader, *setting)
	if err != nil {
		return nil, fmt.Errorf("图片处理失败: %v", err)
	}

	// 生成唯一文件名
	uniqueFileName := processedImage.UniqueFileName

	// 构建FTP目录结构（年/月）
	now := time.Now()
	year := now.Format("2006")
	month := now.Format("01")
	subDir := PathJoin("uploads", year, month)
	objectPath := PathJoin(subDir, uniqueFileName)

	// 获取存储配置
	storageConfig := buckets.ConvertToFTPBucket(bucket.Config)

	// 初始化FTP客户端
	ftpUtil := ftp.NewFTPUtil(ftp.FTPConfig{
		Host:     storageConfig.FTPHost,
		Port:     storageConfig.FTPPort,
		User:     storageConfig.FTPUser,
		Password: storageConfig.FTPPass,
		Timeout:  10,
	})
	defer ftpUtil.Close()
	err = ftpUtil.UploadImage(
		objectPath,
		processedImage.CompressedBytes,
		processedImage.MimeType,
	)
	if err != nil {
		return nil, fmt.Errorf("FTP上传失败：%v", err)
	}

	// 检查是否上传缩略图
	thumbnailURL := ""
	if setting.Thumbnail {
		err := ftpUtil.UploadImage(
			PathJoin(subDir, "thumbnails", uniqueFileName),
			processedImage.ThumbnailBytes,
			"image/webp",
		)
		if err == nil {
			thumbnailURL = "/uploads/" + year + "/" + month + "/thumbnails/" + uniqueFileName
		}
	}

	url := "/uploads/" + year + "/" + month + "/" + uniqueFileName

	return &interfaces.ImageUploadResult{
		Success:      true,
		Message:      "上传成功",
		FileName:     uniqueFileName,
		FileSize:     int64(len(processedImage.CompressedBytes)),
		MimeType:     processedImage.MimeType,
		URL:          url,
		ThumbnailURL: thumbnailURL,
		Storage:      bucket.Type,
		Width:        processedImage.Width,
		Height:       processedImage.Height,
		CreatedAt:    time.Now().Format("2006-01-02 15:04:05"),
	}, nil
}

// 本地默认上传实现
func (u *DefaultUploader) Upload(c *gin.Context, cfg *config.Config, setting *models.Settings, bucket *models.Buckets, fileHeader *multipart.FileHeader) (*interfaces.ImageUploadResult, error) {
	// 验证图片
	if err := images.ValidateImageFile(fileHeader, cfg); err != nil {
		return nil, fmt.Errorf("图片验证失败: %v", err)
	}

	// 打开文件
	file, err := fileHeader.Open()
	if err != nil {
		return nil, fmt.Errorf("打开文件失败: %v", err)
	}
	defer file.Close()

	// 处理图片
	processedImage, err := images.ImageSvc.ProcessImage(file, fileHeader, *setting)
	if err != nil {
		return nil, fmt.Errorf("图片处理失败: %v", err)
	}

	uniqueFileName := processedImage.UniqueFileName

	// 创建年/月子目录
	now := time.Now()
	year := now.Format("2006")
	month := now.Format("01")
	subDir := filepath.Join("uploads", year, month)

	// 确保年月子目录存在
	fullSubDir := filepath.Join(".", subDir)
	if err := ensureUploadDir(fullSubDir); err != nil {
		return nil, fmt.Errorf("创建子目录失败：%v", err)
	}

	// 构建文件路径
	filePath := filepath.Join(fullSubDir, uniqueFileName)

	// 保存处理后的图片文件
	if err := saveFile(filePath, processedImage.CompressedBytes); err != nil {
		return nil, fmt.Errorf("保存文件失败：%v", err)
	}
	thumbnailURL := ""
	// 检查是否上传缩略图
	if setting.Thumbnail {
		if err := ensureUploadDir(filepath.Join(fullSubDir, "thumbnails")); err == nil {
			// 构建缩略图文件路径
			thumbFilePath := filepath.Join(fullSubDir, "thumbnails", uniqueFileName)
			// 保存缩略图文件
			if err := saveFile(thumbFilePath, processedImage.ThumbnailBytes); err != nil {
				log.Println(err)
				// 忽略错误
			}
			thumbnailURL = "/uploads/" + year + "/" + month + "/thumbnails/" + uniqueFileName
		}
	}

	// 构建访问URL (包含年/月子目录)
	fileURL := "/uploads/" + year + "/" + month + "/" + uniqueFileName

	return &interfaces.ImageUploadResult{
		Success:      true,
		Message:      "上传成功",
		URL:          fileURL,
		ThumbnailURL: thumbnailURL,
		Storage:      bucket.Type,
		FileName:     uniqueFileName,
		FileSize:     int64(len(processedImage.CompressedBytes)),
		MimeType:     processedImage.MimeType,
		Width:        processedImage.Width,
		Height:       processedImage.Height,
		CreatedAt:    time.Now().Format("2006-01-02 15:04:05"),
	}, nil
}

// Telegram上传实现
func (u *TelegramUploader) Upload(c *gin.Context, cfg *config.Config, setting *models.Settings, bucket *models.Buckets, fileHeader *multipart.FileHeader) (*interfaces.ImageUploadResult, error) {
	// 验证图片
	if err := images.ValidateImageFile(fileHeader, cfg); err != nil {
		return nil, fmt.Errorf("图片验证失败: %v", err)
	}

	// 打开文件
	file, err := fileHeader.Open()
	if err != nil {
		return nil, fmt.Errorf("打开文件失败: %v", err)
	}
	defer file.Close()

	// 处理图片
	processedImage, err := images.ImageSvc.ProcessImage(file, fileHeader, *setting)
	if err != nil {
		return nil, fmt.Errorf("图片处理失败: %v", err)
	}

	// 获取存储配置
	storageConfig := buckets.ConvertToTelegramBucket(bucket.Config)

	// 基础参数校验，设置存储时已校验，这里弃用
	// if setting.TGBotToken == "" {
	// 	return nil, fmt.Errorf("telegram bot token 不能为空")
	// }
	// if setting.TGReceivers == "" {
	// 	return nil, fmt.Errorf("telegram receivers 不能为空")
	// }

	now := time.Now()
	year := now.Format("2006")
	month := now.Format("01")

	// 5. 初始化TG客户端
	tgClient := telegram.NewClient(storageConfig.TGBotToken)
	tgClient.Timeout = 20 * time.Second
	tgClient.Retry = 3

	uniqueFileName := processedImage.UniqueFileName

	// 上传主图片
	fileID, messageID, err := tgClient.UploadPhotoByBytes(
		storageConfig.TGReceivers,
		processedImage.CompressedBytes,
		processedImage.UniqueFileName,
		fmt.Sprintf("上传图片: %s", processedImage.UniqueFileName),
	)
	if err != nil {
		return nil, fmt.Errorf("Telegram上传图片失败")
	}

	// 7. 上传缩略图（如果开启）
	thumbnailURL := ""
	thumbFileIDURL := ""
	thumbFileMessageID := 0
	if setting.Thumbnail && len(processedImage.ThumbnailBytes) > 0 {
		thumbFileID, thumbMessageID, err := tgClient.UploadPhotoByBytes(
			storageConfig.TGReceivers,
			processedImage.ThumbnailBytes,
			fmt.Sprintf("thumbnail_%s", uniqueFileName),
			fmt.Sprintf("缩略图: %s", processedImage.UniqueFileName),
		)
		if err == nil {
			// Telegram没有直接的URL，这里存储fileID作为标识
			thumbFileIDURL = thumbFileID
			thumbFileMessageID = thumbMessageID
			thumbnailURL = fmt.Sprintf("/uploads/%s/%s/thumbnails/%s", year, month, uniqueFileName)
		} else {
			log.Printf("Telegram上传缩略图失败: %v", err)
		}
	}

	url := "/uploads/" + year + "/" + month + "/" + uniqueFileName

	telegramModel := models.ImageTeleGram{
		TGFileId:             fileID,
		TGThumbnailFileId:    thumbFileIDURL,
		TGMessageId:          messageID,
		TGThumbnailMessageId: thumbFileMessageID,
		FileName:             uniqueFileName,
	}
	db := database.GetDB()
	if db != nil {
		if err := db.DB.Create(&telegramModel).Error; err != nil {
			return nil, fmt.Errorf("保存telegram图片信息到数据库失败")
		}
	}

	return &interfaces.ImageUploadResult{
		Success:      true,
		Message:      "Telegram上传成功",
		FileName:     processedImage.UniqueFileName,
		FileSize:     int64(len(processedImage.CompressedBytes)),
		MimeType:     processedImage.MimeType,
		URL:          url,
		ThumbnailURL: thumbnailURL,
		Storage:      bucket.Type, // 存储类型标识
		CreatedAt:    time.Now().Format("2006-01-02 15:04:05"),
		Width:        processedImage.Width,
		Height:       processedImage.Height,
	}, nil
}

// ensureUploadDir 确保上传目录存在
func ensureUploadDir(uploadPath string) error {
	if _, err := os.Stat(uploadPath); os.IsNotExist(err) {
		return os.MkdirAll(uploadPath, 0755)
	}
	return nil
}

// saveFile 保存文件到磁盘
func saveFile(filePath string, data []byte) error {
	file, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	_, err = file.Write(data)
	return err
}

// 辅助函数
func PathJoin(parts ...string) string {
	return strings.Join(parts, "/")
}
