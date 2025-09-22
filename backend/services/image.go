package services

import (
	"bytes"
	"fmt"
	"image"
	"image/gif"
	"image/jpeg"
	"image/png"
	"io"
	"mime/multipart"
	"slices"
	"strings"

	"github.com/chai2010/webp"
	"github.com/disintegration/imaging"
)

type ImageService struct{}

var ImageSvc *ImageService

// InitImageService 初始化图片服务
func InitImageService() {
	ImageSvc = &ImageService{}
}

// ProcessImage 处理图片（压缩、获取尺寸等）
func (s *ImageService) ProcessImage(file multipart.File, header *multipart.FileHeader) (*ProcessedImage, error) {
	// 读取文件内容
	fileBytes, err := io.ReadAll(file)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %v", err)
	}

	// 重置文件指针
	file.Seek(0, 0)

	// 解码图片
	img, format, err := s.decodeImage(bytes.NewReader(fileBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to decode image: %v", err)
	}

	// 获取图片尺寸
	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()

	// 检查是否为特殊格式（保持原格式）
	mimeType := header.Header.Get("Content-Type")
	var processedBytes []byte
	var finalFormat string
	var finalMimeType string

	if s.isSpecialFormat(format, mimeType) {
		// 特殊格式保持原样
		processedBytes = fileBytes
		finalFormat = format
		finalMimeType = mimeType
	} else if strings.ToLower(format) == "webp" {
		// 如果原本就是webp，直接使用原文件或压缩
		if header.Size > 1024*1024 { // 大于1MB时压缩
			processedBytes, err = s.compressWebP(img, 85) // 85%质量
			if err != nil {
				return nil, fmt.Errorf("failed to compress webp image: %v", err)
			}
		} else {
			processedBytes = fileBytes
		}
		finalFormat = "webp"
		finalMimeType = "image/webp"
	} else {
		// 其他格式转换为webp
		processedBytes, err = s.convertToWebP(img, 85) // 85%质量
		if err != nil {
			return nil, fmt.Errorf("failed to convert to webp: %v", err)
		}
		finalFormat = "webp"
		finalMimeType = "image/webp"
	}

	// 生成缩略图（根据最终格式）
	var thumbnailBytes []byte
	if s.isSpecialFormat(finalFormat, finalMimeType) {
		// 特殊格式使用原图作为缩略图或生成jpeg缩略图
		thumbnailBytes, err = s.generateJPEGThumbnail(img, 300, 300, 80)
		if err != nil {
			return nil, fmt.Errorf("failed to generate thumbnail: %v", err)
		}
	} else {
		// 普通格式生成webp缩略图
		thumbnailBytes, err = s.generateWebPThumbnail(img, 300, 300, 80)
		if err != nil {
			return nil, fmt.Errorf("failed to generate webp thumbnail: %v", err)
		}
	}

	return &ProcessedImage{
		OriginalBytes:   fileBytes,
		CompressedBytes: processedBytes,
		ThumbnailBytes:  thumbnailBytes,
		Width:           width,
		Height:          height,
		Format:          finalFormat,
		MimeType:        finalMimeType,
	}, nil
}

// isSpecialFormat 检查是否为特殊格式（需要保持原格式）
func (s *ImageService) isSpecialFormat(format, mimeType string) bool {
	specialFormats := []string{"gif"}
	specialMimeTypes := []string{"image/gif", "image/svg+xml"}

	formatLower := strings.ToLower(format)
	for _, sf := range specialFormats {
		if formatLower == sf {
			return true
		}
	}

	for _, smt := range specialMimeTypes {
		if mimeType == smt {
			return true
		}
	}

	return false
}

// decodeImage 解码图片，支持webp格式
func (s *ImageService) decodeImage(reader io.Reader) (image.Image, string, error) {
	// 读取数据到缓冲区
	data, err := io.ReadAll(reader)
	if err != nil {
		return nil, "", err
	}

	// 尝试解码webp
	if img, err := webp.Decode(bytes.NewReader(data)); err == nil {
		return img, "webp", nil
	}

	// 尝试解码gif
	if img, err := gif.Decode(bytes.NewReader(data)); err == nil {
		return img, "gif", nil
	}

	// 尝试解码png
	if img, err := png.Decode(bytes.NewReader(data)); err == nil {
		return img, "png", nil
	}

	// 使用标准库解码其他格式
	img, format, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, "", err
	}

	return img, format, nil
}

// convertToWebP 将图片转换为webp格式
func (s *ImageService) convertToWebP(img image.Image, quality int) ([]byte, error) {
	// 使用chai2010/webp包进行webp编码
	data, err := webp.EncodeRGBA(img, float32(quality))
	if err != nil {
		return nil, fmt.Errorf("failed to encode webp: %v", err)
	}
	return data, nil
}

// compressWebP 压缩webp图片
func (s *ImageService) compressWebP(img image.Image, quality int) ([]byte, error) {
	return s.convertToWebP(img, quality)
}

// ValidateImage 验证图片格式和大小
func (s *ImageService) ValidateImage(header *multipart.FileHeader, allowedTypes []string, maxSize int64) error {
	// 检查文件大小
	if header.Size > maxSize {
		return fmt.Errorf("file size exceeds limit: %d bytes", maxSize)
	}

	// 检查文件类型
	mimeType := header.Header.Get("Content-Type")
	if mimeType == "" {
		return fmt.Errorf("missing content type")
	}

	if slices.Contains(allowedTypes, mimeType) {
		return nil
	}

	return fmt.Errorf("unsupported file type: %s", mimeType)
}

// generateJPEGThumbnail 生成JPEG格式缩略图
func (s *ImageService) generateJPEGThumbnail(img image.Image, maxWidth, maxHeight, quality int) ([]byte, error) {
	// 调整图片大小，保持宽高比
	thumbnail := imaging.Fit(img, maxWidth, maxHeight, imaging.Lanczos)

	// 转换为JPEG格式
	var buf bytes.Buffer
	err := jpeg.Encode(&buf, thumbnail, &jpeg.Options{Quality: quality})
	if err != nil {
		return nil, fmt.Errorf("failed to encode jpeg thumbnail: %v", err)
	}
	return buf.Bytes(), nil
}

// generateWebPThumbnail 生成webp格式缩略图
func (s *ImageService) generateWebPThumbnail(img image.Image, maxWidth, maxHeight, quality int) ([]byte, error) {
	// 调整图片大小，保持宽高比
	thumbnail := imaging.Fit(img, maxWidth, maxHeight, imaging.Lanczos)

	// 转换为webp格式
	return s.convertToWebP(thumbnail, quality)
}

// ProcessedImage 处理后的图片数据
type ProcessedImage struct {
	OriginalBytes   []byte
	CompressedBytes []byte
	ThumbnailBytes  []byte
	Width           int
	Height          int
	Format          string
	MimeType        string
}
