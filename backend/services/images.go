package services

import (
	"bytes"
	"fmt"
	"image"
	"mime/multipart"
	"oneimg/backend/models"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"
)

// ImageUploadResult 上传返回结构
type ImageUploadResult struct {
	Success   bool   `json:"success"`
	Message   string `json:"message,omitempty"`
	ID        int    `json:"id,omitempty"`
	URL       string `json:"url,omitempty"`
	FileName  string `json:"filename,omitempty"`
	FileSize  int64  `json:"file_size,omitempty"`
	MimeType  string `json:"mime_type,omitempty"`
	Width     int    `json:"width,omitempty"`
	Height    int    `json:"height,omitempty"`
	CreatedAt string `json:"created_at,omitempty"`
}

func ValidateImage(fileHeader *multipart.FileHeader, allowedTypes []string, maxSize int64) error {
	return nil // 简化实现，实际项目中应添加详细验证逻辑
}

func UploadToLocal(fileBytes []byte, fileHeader *multipart.FileHeader, setting models.Settings) (*models.Image, error) {
	// 1. 生成唯一文件名
	uniqueFileName := generateUniqueFileName(fileHeader.Filename)

	// 2. 保存文件到本地
	savePath := "./uploads"
	if err := SaveFile(fileBytes, savePath, uniqueFileName); err != nil {
		return nil, fmt.Errorf("保存文件失败：%v", err)
	}

	// 3. 获取图片尺寸
	width, height, err := GetImageInfo(fileBytes)
	if err != nil {
		return nil, fmt.Errorf("解析图片尺寸失败：%v", err)
	}

	// 4. 构造返回信息
	imageModel := &models.Image{
		Url:      fmt.Sprintf("/uploads/%s", uniqueFileName),
		FileName: fileHeader.Filename,
		FileSize: fileHeader.Size,
		Width:    width,
		Height:   height,
		Storage:  "default",
	}

	return imageModel, nil
}

func UploadToS3(fileBytes []byte, fileHeader *multipart.FileHeader, setting models.Settings) (*models.Image, error) {
	// S3上传逻辑（简化实现）
	return UploadToLocal(fileBytes, fileHeader, setting)
}

func UploadToWebDAV(fileBytes []byte, fileHeader *multipart.FileHeader, setting models.Settings) (*models.Image, error) {
	// WebDAV上传逻辑（简化实现）
	return UploadToLocal(fileBytes, fileHeader, setting)
}

func SaveFile(fileBytes []byte, savePath, fileName string) error {
	// 创建目录（如果不存在）
	if err := os.MkdirAll(savePath, 0755); err != nil {
		return fmt.Errorf("创建目录失败：%v", err)
	}

	// 写入文件
	fullPath := filepath.Join(savePath, fileName)
	file, err := os.Create(fullPath)
	if err != nil {
		return fmt.Errorf("创建文件失败：%v", err)
	}
	defer file.Close()

	_, err = file.Write(fileBytes)
	if err != nil {
		return fmt.Errorf("写入文件失败：%v", err)
	}

	return nil
}

func GetImageInfo(fileBytes []byte) (width, height int, err error) {
	// 解码图片
	img, _, err := image.Decode(bytes.NewReader(fileBytes))
	if err != nil {
		return 0, 0, fmt.Errorf("解码图片失败：%v", err)
	}

	bounds := img.Bounds()
	return bounds.Dx(), bounds.Dy(), nil
}

// generateUniqueFileName 生成唯一文件名
func generateUniqueFileName(originalName string) string {
	ext := filepath.Ext(originalName)
	// 时间戳+UUID保证唯一性
	timestamp := time.Now().UnixMicro()
	uuidStr := uuid.New().String()[:8] // 取UUID前8位
	return fmt.Sprintf("%d_%s%s", timestamp, uuidStr, ext)
}
