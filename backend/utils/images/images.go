package images

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"image/jpeg"
	"io"
	"log"
	"math/rand"
	"mime/multipart"
	"oneimg/backend/models"
	"oneimg/backend/utils/watermark"
	"path/filepath"
	"strings"
	"time"

	"github.com/chai2010/webp"
	"github.com/disintegration/imaging"
	"github.com/google/uuid"
	"golang.org/x/exp/slices"
)

// 常量定义
const (
	DefaultCompressQuality = 85
	OriginalQuality        = 100
	ThumbnailMaxWidth      = 300
	ThumbnailMaxHeight     = 300
	ThumbnailQuality       = 80
	CompressSizeThreshold  = 1024 * 1024 // 1MB
)

// 特殊格式常量
var (
	specialFormats   = []string{"gif", "svg"}
	specialMimeTypes = []string{
		"image/gif",
		"image/svg+xml",
	}
	ErrUnsupportedFormat  = errors.New("unsupported image format")
	ErrFileTooLarge       = errors.New("file size exceeds limit")
	ErrMissingContentType = errors.New("missing content type")
	ErrSVGThumbnail       = errors.New("svg thumbnail generation not supported")
)

type ImageService struct{}

var ImageSvc *ImageService

// InitImageService 初始化图片服务（线程安全）
func InitImageService() {
	if ImageSvc == nil {
		ImageSvc = &ImageService{}
	}
}

// ProcessedImage 处理后的图片数据
type ProcessedImage struct {
	OriginalBytes   []byte // 原始文件字节
	CompressedBytes []byte // 处理后的字节
	ThumbnailBytes  []byte // 缩略图字节
	Width           int    // 图片宽度
	Height          int    // 图片高度
	Format          string // 最终格式
	MimeType        string // 最终MIME类型
	OutputExt       string // 输出文件扩展名
	UniqueFileName  string // 唯一文件名
}

// ProcessImage 处理图片（压缩、获取尺寸等）
func (s *ImageService) ProcessImage(
	file multipart.File,
	header *multipart.FileHeader,
	setting models.Settings,
	userRole int,
) (*ProcessedImage, error) {
	// Every backend (including URL ingestion) validates actual raster bytes before
	// a decoder can allocate. Browser/remote MIME is only a mismatch check.
	release := AcquireProcessing()
	defer release()
	fileBytes, err := ReadDirectLimited(file, UploadByteLimit(int64(setting.MaxFileSize)))
	if err != nil { return nil, err }
	img, format, err := decodeDirectImage(fileBytes, declaredImageMIME(header.Header.Get("Content-Type")))
	if err != nil { return nil, fmt.Errorf("decode image failed: %w", err) }
	if _, actual, err := ValidateDirectImage(fileBytes, ""); err != nil { return nil, err
	} else if !AllowedMIME(actual, strings.Split(setting.AllowedTypes, ",")) { return nil, ErrUnsupportedFormat }
	width, height := img.Bounds().Dx(), img.Bounds().Dy()
	mimeType := formatMIME(format)
	originalFileName := header.Filename
	processedBytes, finalFormat, finalMimeType, err := s.processMainImage(fileBytes, img, format, mimeType, int64(len(fileBytes)), setting)
	if err != nil { return nil, fmt.Errorf("process main image failed: %w", err) }

	_, finalMimeType, err = ValidateDirectImage(processedBytes, "")
	if err != nil { return nil, err }; finalFormat = strings.TrimPrefix(finalMimeType, "image/")
	// 5. 处理文件扩展名
	outputExt := map[string]string{
		"image/jpeg":    ".jpg",
		"image/png":     ".png",
		"image/gif":     ".gif",
		"image/webp":    ".webp",
		"image/svg+xml": ".svg",
		"image/bmp":     ".bmp",
		"image/tiff":    ".tiff",
		"image/heic":    ".heic",
		"image/heif":    ".heif",
	}

	// 6. Only generate thumbnail bytes when enabled. All storage uploaders use
	// this pipeline; nil also keeps disabled-thumbnail size/accounting at zero.
	var thumbnailBytes []byte
	if setting.Thumbnail {
		thumbnailBytes, err = s.generateThumbnail(img, finalFormat, finalMimeType, fileBytes)
		if err != nil {
			log.Printf("generate thumbnail failed: %v", err)
			thumbnailBytes = nil
		}
	}

	// Always use the actual encoded extension; never publish user-controlled
	// directories or overwrite an existing logical image with an original name.
	stem := strings.TrimSuffix(filepath.Base(strings.ReplaceAll(originalFileName, "\\", "/")), filepath.Ext(originalFileName))
	if !setting.SaveOriginalName {
		pattern := setting.FileName
		if pattern == "" { pattern = "{random}" }
		stem = s.ReplaceMagicVariables(pattern, originalFileName, userRole)
	}
	stem = safeFilenameStem(stem)
	fileName := stem + "_" + uuid.NewString() + outputExt[finalMimeType]

	// 8. 组装返回结果
	return &ProcessedImage{
		OriginalBytes:   fileBytes,
		CompressedBytes: processedBytes,
		ThumbnailBytes:  thumbnailBytes,
		Width:           width,
		Height:          height,
		Format:          finalFormat,
		MimeType:        finalMimeType,
		OutputExt:       outputExt[finalMimeType],
		UniqueFileName:  fileName,
	}, nil
}

// processMainImage 处理主图片（拆分逻辑，提高可读性）
func (s *ImageService) processMainImage(
	fileBytes []byte,
	img image.Image,
	format, mimeType string,
	fileSize int64,
	setting models.Settings,
) ([]byte, string, string, error) {
	// 特殊格式（GIF/SVG）直接返回原数据，不处理水印和压缩
	if s.isSpecialFormat(format, mimeType) {
		if format == "svg" || mimeType == "image/svg+xml" {
			return fileBytes, "svg", "image/svg+xml", nil
		}
		return fileBytes, format, mimeType, nil
	}

	// 添加水印
	if setting.WatermarkEnable {
		watermarkCfg := watermark.WatermarkSetting(setting)
		fileReader := bytes.NewReader(fileBytes)
		processedReader, err := watermark.ProcessImageWithWatermark(fileReader, mimeType, watermarkCfg)
		if err != nil {
			return nil, "", "", fmt.Errorf("添加水印失败：%w", err)
		}
		fileBytes, err = io.ReadAll(processedReader)
		if err != nil {
			return nil, "", "", fmt.Errorf("读取水印后图片数据失败：%w", err)
		}
		img, _, err = image.Decode(bytes.NewReader(fileBytes))
		if err != nil {
			return nil, "", "", fmt.Errorf("解码水印后图片失败：%w", err)
		}
	}

	// WebP格式处理
	if strings.ToLower(format) == "webp" {
		if setting.CompressImage && fileSize > CompressSizeThreshold {
			compressed, err := s.compressWebP(img, DefaultCompressQuality)
			if err != nil {
				return nil, "", "", fmt.Errorf("compress webp: %w", err)
			}
			return compressed, "webp", "image/webp", nil
		}
		return fileBytes, "webp", "image/webp", nil
	}

	// 其他格式处理
	quality := OriginalQuality
	if setting.CompressImage && fileSize > CompressSizeThreshold {
		quality = DefaultCompressQuality
	}

	// 需要转换为WebP
	if setting.SaveWebp {
		webpData, err := s.convertToWebP(img, quality)
		if err != nil {
			return nil, "", "", fmt.Errorf("convert to webp: %w", err)
		}
		log.Println("转换webp")
		return webpData, "webp", "image/webp", nil
	}

	// 压缩图片
	if setting.CompressImage {
		compressed, err := s.compressWebP(img, DefaultCompressQuality)
		if err != nil {
			return nil, "", "", fmt.Errorf("compress webp: %w", err)
		}
		return compressed, "webp", "image/webp", nil
	}

	return fileBytes, format, mimeType, nil
}

// generateThumbnail 生成缩略图（新增SVG处理）
func (s *ImageService) generateThumbnail(
	img image.Image,
	format, mimeType string,
	originalBytes []byte, // 新增原始字节参数，用于SVG
) ([]byte, error) {
	return s.generateWebPThumbnail(img, ThumbnailMaxWidth, ThumbnailMaxHeight, ThumbnailQuality)
}

// isSpecialFormat 检查是否为特殊格式（需要保持原格式）
func (s *ImageService) isSpecialFormat(format, mimeType string) bool {
	// 检查格式
	if slices.Contains(specialFormats, strings.ToLower(format)) {
		return true
	}

	// 检查MIME类型
	if slices.Contains(specialMimeTypes, mimeType) {
		return true
	}

	return false
}

// decodeImage 解码图片，支持webp/gif/png/jpeg/SVG等格式
// 优化点：增加SVG处理，避免解码失败
func (s *ImageService) decodeImage(reader io.Reader, mimeType string) (image.Image, string, error) {
	data, err := ReadDirectLimited(reader, MaxUploadBytes)
	if err != nil { return nil, "", err }
	return decodeDirectImage(data, declaredImageMIME(mimeType))
}

// convertToWebP 将图片转换为webp格式
func (s *ImageService) convertToWebP(img image.Image, quality int) ([]byte, error) {
	if quality < 0 || quality > 100 {
		return nil, fmt.Errorf("invalid quality: %d (must be 0-100)", quality)
	}

	data, err := webp.EncodeRGBA(img, float32(quality))
	if err != nil {
		return nil, fmt.Errorf("encode webp: %w", err)
	}

	return data, nil
}

// compressWebP 压缩webp图片
func (s *ImageService) compressWebP(img image.Image, quality int) ([]byte, error) {
	return s.convertToWebP(img, quality)
}

// ValidateImage 验证图片格式和大小
func (s *ImageService) ValidateImage(
	header *multipart.FileHeader,
	allowedTypes []string,
	maxSize int64,
) error {
	if header == nil || header.Size <= 0 || header.Size > UploadByteLimit(maxSize) { return ErrFileTooLarge }
	file, err := header.Open()
	if err != nil { return err }
	defer file.Close()
	data, err := ReadDirectLimited(file, UploadByteLimit(maxSize))
	if err != nil { return err }
	_, actual, err := ValidateDirectImage(data, declaredImageMIME(header.Header.Get("Content-Type")))
	if err != nil { return err }
	if !AllowedMIME(actual, allowedTypes) { return ErrUnsupportedFormat }
	return nil
}

// generateJPEGThumbnail 生成JPEG格式缩略图
func (s *ImageService) generateJPEGThumbnail(
	img image.Image,
	maxWidth, maxHeight, quality int,
) ([]byte, error) {
	// 空图片（如SVG）直接返回空字节
	if img.Bounds().Dx() == 0 && img.Bounds().Dy() == 0 {
		return []byte{}, ErrSVGThumbnail
	}

	// 调整图片大小（保持宽高比）
	thumbnail := imaging.Fit(img, maxWidth, maxHeight, imaging.Lanczos)

	// 编码为JPEG
	var buf bytes.Buffer
	err := jpeg.Encode(&buf, thumbnail, &jpeg.Options{Quality: quality})
	if err != nil {
		return nil, fmt.Errorf("encode jpeg: %w", err)
	}

	return buf.Bytes(), nil
}

// generateWebPThumbnail 生成webp格式缩略图
func (s *ImageService) generateWebPThumbnail(
	img image.Image,
	maxWidth, maxHeight, quality int,
) ([]byte, error) {
	// 空图片（如SVG）直接返回空字节
	if img.Bounds().Dx() == 0 && img.Bounds().Dy() == 0 {
		return []byte{}, ErrSVGThumbnail
	}

	// 调整图片大小
	thumbnail := imaging.Fit(img, maxWidth, maxHeight, imaging.Lanczos)

	// 转换为WebP
	return s.convertToWebP(thumbnail, quality)
}

// ValidateImageFile 验证图片文件
func ValidateImageFile(header *multipart.FileHeader, setting *models.Settings) error {
	allowedTypes := strings.Split(setting.AllowedTypes, ",")
	return ImageSvc.ValidateImage(header, allowedTypes, int64(setting.MaxFileSize))
}

// ReadFileContent 读取文件内容
func ReadFileContent(header *multipart.FileHeader) ([]byte, error) {
	file, err := header.Open()
	if err != nil {
		return nil, fmt.Errorf("打开文件失败：%v", err)
	}
	defer file.Close()

	return ReadDirectLimited(file, MaxUploadBytes)
}

// GetFileMimeType 获取文件MIME类型
func GetFileMimeType(header *multipart.FileHeader) string {
	return header.Header.Get("Content-Type")
}

// generateUniqueFileName 生成唯一文件名
func generateUniqueFileName() string {
	timestamp := time.Now().UnixNano()
	hash := fmt.Sprintf("%x", timestamp)
	rand.New(rand.NewSource(time.Now().UnixNano()))
	randomNum := rand.Intn(900) + 100
	return fmt.Sprintf("%s%d", hash, randomNum)
}

// ReplaceMagicVariables 替换魔法变量
func (s *ImageService) ReplaceMagicVariables(pattern string, originalName string, role int) string {
	now := time.Now()

	// 基础时间变量
	pattern = strings.ReplaceAll(pattern, "{year}", now.Format("2006"))
	pattern = strings.ReplaceAll(pattern, "{month}", now.Format("01"))
	pattern = strings.ReplaceAll(pattern, "{moon}", now.Format("01"))
	pattern = strings.ReplaceAll(pattern, "{day}", now.Format("02"))
	pattern = strings.ReplaceAll(pattern, "{hour}", now.Format("15"))
	pattern = strings.ReplaceAll(pattern, "{minute}", now.Format("04"))
	pattern = strings.ReplaceAll(pattern, "{second}", now.Format("05"))

	// 角色变量
	roleStr := "guest"
	if role == 1 {
		roleStr = "admin"
	}
	pattern = strings.ReplaceAll(pattern, "{role}", roleStr)

	// 随机变量
	if strings.Contains(pattern, "{random}") {
		pattern = strings.ReplaceAll(pattern, "{random}", generateUniqueFileName())
	}

	// UUID变量
	if strings.Contains(pattern, "{uuid}") {
		pattern = strings.ReplaceAll(pattern, "{uuid}", uuid.New().String())
	}

	// 原始文件名（不含扩展名）
	ext := filepath.Ext(originalName)
	nameWithoutExt := strings.TrimSuffix(originalName, ext)
	pattern = strings.ReplaceAll(pattern, "{filename}", nameWithoutExt)

	return pattern
}
