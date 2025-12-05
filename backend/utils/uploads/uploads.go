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
	"oneimg/backend/interfaces"
	"oneimg/backend/models"
	"oneimg/backend/utils/images"
	"oneimg/backend/utils/s3"
	"oneimg/backend/utils/webdav"
)

// 所有上传器实现统一接口
type S3R2Uploader struct{}
type WebDAVUploader struct{}
type DefaultUploader struct{}

// S3/R2上传实现
func (u *S3R2Uploader) Upload(c *gin.Context, cfg *config.Config, setting *models.Settings, fileHeader *multipart.FileHeader) (*interfaces.ImageUploadResult, error) {
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

	client, err := s3.NewS3Client(*setting)
	if err != nil {
		return nil, fmt.Errorf("创建S3/R2客户端失败：%v", err)
	}

	// 上传文件到S3/R2
	contentType := "image/webp"
	if !setting.SaveWebp {
		contentType = fileHeader.Header.Get("Content-Type")
	}

	_, err = client.PutObject(context.TODO(), &awss3.PutObjectInput{
		Bucket:      aws.String(setting.S3Bucket),
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
			Bucket:      aws.String(setting.S3Bucket),
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
		FileName:     uniqueFileName,
		FileSize:     int64(len(processedImage.CompressedBytes)),
		MimeType:     contentType,
		URL:          url,
		ThumbnailURL: thumbnailURL,
		Storage:      setting.StorageType,
		CreatedAt:    time.Now().Format("2006-01-02 15:04:05"),
		Width:        processedImage.Width,
		Height:       processedImage.Height,
	}, nil
}

// WebDAV上传实现
func (u *WebDAVUploader) Upload(c *gin.Context, cfg *config.Config, setting *models.Settings, fileHeader *multipart.FileHeader) (*interfaces.ImageUploadResult, error) {
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

	// 初始化WebDAV客户端
	client := webdav.Client(webdav.Config{
		BaseURL:  setting.WebdavURL,
		Username: setting.WebdavUser,
		Password: setting.WebdavPass,
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
		FileName:     uniqueFileName,
		FileSize:     int64(len(processedImage.CompressedBytes)),
		MimeType:     processedImage.MimeType,
		URL:          url,
		ThumbnailURL: thumbnailURL,
		Storage:      setting.StorageType,
		Width:        processedImage.Width,
		Height:       processedImage.Height,
		CreatedAt:    time.Now().Format("2006-01-02 15:04:05"),
	}, nil
}

// 本地默认上传实现
func (u *DefaultUploader) Upload(c *gin.Context, cfg *config.Config, setting *models.Settings, fileHeader *multipart.FileHeader) (*interfaces.ImageUploadResult, error) {
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
		Storage:      setting.StorageType,
		FileName:     uniqueFileName,
		FileSize:     int64(len(processedImage.CompressedBytes)),
		MimeType:     processedImage.MimeType,
		Width:        processedImage.Width,
		Height:       processedImage.Height,
		CreatedAt:    time.Now().Format("2006-01-02 15:04:05"),
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
