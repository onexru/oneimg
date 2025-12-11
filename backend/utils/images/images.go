package images

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"image/gif"
	"image/jpeg"
	"image/png"
	"io"
	"log"
	"math/rand"
	"mime/multipart"
	"oneimg/backend/config"
	"oneimg/backend/models"
	"oneimg/backend/utils/watermark"
	"strings"
	"time"

	"github.com/chai2010/webp"
	"github.com/disintegration/imaging"
	"golang.org/x/exp/slices"
)

// 常量定义 - 提取魔法数字和固定值
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
	specialFormats   = []string{"gif"}
	specialMimeTypes = []string{
		"image/gif",
		"image/svg+xml",
	}
	ErrUnsupportedFormat  = errors.New("unsupported image format")
	ErrFileTooLarge       = errors.New("file size exceeds limit")
	ErrMissingContentType = errors.New("missing content type")
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
) (*ProcessedImage, error) {
	// 1. 读取文件内容（一次性读取，避免多次IO）
	fileBytes, err := io.ReadAll(file)
	if err != nil {
		return nil, fmt.Errorf("read file failed: %w", err)
	}

	// 2. 解码图片（获取原图信息）
	img, format, err := s.decodeImage(bytes.NewReader(fileBytes))
	if err != nil {
		return nil, fmt.Errorf("decode image failed: %w", err)
	}

	// 3. 获取图片基本信息
	bounds := img.Bounds()
	width, height := bounds.Dx(), bounds.Dy()
	mimeType := header.Header.Get("Content-Type")

	// 4. 处理主图片（压缩/格式转换）
	processedBytes, finalFormat, finalMimeType, err := s.processMainImage(
		fileBytes, img, format, mimeType, header.Size, setting,
	)
	if err != nil {
		return nil, fmt.Errorf("process main image failed: %w", err)
	}

	// 5. 处理文件扩展名
	outputExt := map[string]string{
		"image/jpeg":    ".jpg",  // JPEG格式
		"image/png":     ".png",  // PNG格式
		"image/gif":     ".gif",  // GIF格式
		"image/webp":    ".webp", // WebP格式
		"image/svg+xml": ".svg",  // SVG格式
		"image/bmp":     ".bmp",  // BMP格式
		"image/tiff":    ".tiff", // TIFF格式
		"image/heic":    ".heic", // HEIC格式
		"image/heif":    ".heif", // HEIF格式
	}

	// 将主图转化成image.Image用于生成缩略图
	reader := bytes.NewReader(processedBytes)
	img, _, err = image.Decode(reader)
	if err != nil {
		return nil, fmt.Errorf("decode image failed: %w", err)
	}

	// 6. 生成缩略图
	thumbnailBytes, err := s.generateThumbnail(img, finalFormat, finalMimeType)
	if err != nil {
		return nil, fmt.Errorf("generate thumbnail failed: %w", err)
	}

	// 7. 组装返回结果
	return &ProcessedImage{
		OriginalBytes:   fileBytes,
		CompressedBytes: processedBytes,
		ThumbnailBytes:  thumbnailBytes,
		Width:           width,
		Height:          height,
		Format:          finalFormat,
		MimeType:        finalMimeType,
		OutputExt:       outputExt[finalMimeType],
		UniqueFileName:  generateUniqueFileName(outputExt[finalMimeType]),
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
	// 特殊格式直接返回原数据
	if s.isSpecialFormat(format, mimeType) {
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
		if setting.OriginalImage || fileSize <= CompressSizeThreshold {
			return fileBytes, "webp", "image/webp", nil
		}
		compressed, err := s.compressWebP(img, DefaultCompressQuality)
		if err != nil {
			return nil, "", "", fmt.Errorf("compress webp: %w", err)
		}
		return compressed, "webp", "image/webp", nil
	}

	// 其他格式处理
	quality := OriginalQuality
	if !setting.OriginalImage && fileSize > CompressSizeThreshold {
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

	// 保存原图
	if setting.OriginalImage {
		return fileBytes, format, mimeType, nil
	}

	// 默认进行压缩
	compressed, err := s.compressWebP(img, DefaultCompressQuality)
	if err != nil {
		return nil, "", "", fmt.Errorf("compress webp: %w", err)
	}
	return compressed, format, mimeType, nil
}

// generateThumbnail 生成缩略图
func (s *ImageService) generateThumbnail(
	img image.Image,
	format, mimeType string,
) ([]byte, error) {
	// 特殊格式生成JPEG缩略图
	if s.isSpecialFormat(format, mimeType) {
		return s.generateJPEGThumbnail(img, ThumbnailMaxWidth, ThumbnailMaxHeight, ThumbnailQuality)
	}

	// 普通格式生成WebP缩略图
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

// decodeImage 解码图片，支持webp/gif/png/jpeg等格式
// 优化点：减少内存拷贝，按优先级解码
func (s *ImageService) decodeImage(reader io.Reader) (image.Image, string, error) {
	// 读取数据到缓冲区（复用）
	data, err := io.ReadAll(reader)
	if err != nil {
		return nil, "", fmt.Errorf("read image data: %w", err)
	}
	buf := bytes.NewReader(data)

	// 按优先级解码（常用格式优先）
	decodeFuncs := []struct {
		decode func(*bytes.Reader) (image.Image, error)
		format string
	}{
		{func(r *bytes.Reader) (image.Image, error) { return webp.Decode(r) }, "webp"},
		{func(r *bytes.Reader) (image.Image, error) { return gif.Decode(r) }, "gif"},
		{func(r *bytes.Reader) (image.Image, error) { return png.Decode(r) }, "png"},
		{func(r *bytes.Reader) (image.Image, error) { return jpeg.Decode(r) }, "jpeg"},
	}

	for _, df := range decodeFuncs {
		buf.Seek(0, io.SeekStart) // 重置读取指针
		img, err := df.decode(buf)
		if err == nil {
			return img, df.format, nil
		}
	}

	// 最后尝试标准库的自动检测
	buf.Seek(0, io.SeekStart)
	img, format, err := image.Decode(buf)
	if err != nil {
		return nil, "", fmt.Errorf("%w: %v", ErrUnsupportedFormat, err)
	}

	return img, format, nil
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

// compressWebP 压缩webp图片（复用转换逻辑）
func (s *ImageService) compressWebP(img image.Image, quality int) ([]byte, error) {
	return s.convertToWebP(img, quality)
}

// ValidateImage 验证图片格式和大小
func (s *ImageService) ValidateImage(
	header *multipart.FileHeader,
	allowedTypes []string,
	maxSize int64,
) error {
	// 检查文件大小
	if header.Size > maxSize {
		return fmt.Errorf("%w: max size %d bytes, got %d bytes",
			ErrFileTooLarge, maxSize, header.Size)
	}

	// 检查Content-Type
	mimeType := header.Header.Get("Content-Type")
	if mimeType == "" {
		return ErrMissingContentType
	}

	// 检查是否允许的类型
	if !slices.Contains(allowedTypes, mimeType) {
		return fmt.Errorf("unsupported content type: %s (allowed: %s)",
			mimeType, strings.Join(allowedTypes, ", "))
	}

	return nil
}

// generateJPEGThumbnail 生成JPEG格式缩略图
func (s *ImageService) generateJPEGThumbnail(
	img image.Image,
	maxWidth, maxHeight, quality int,
) ([]byte, error) {
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
	// 调整图片大小
	thumbnail := imaging.Fit(img, maxWidth, maxHeight, imaging.Lanczos)

	// 转换为WebP
	return s.convertToWebP(thumbnail, quality)
}

// ValidateImageFile 验证图片文件
func ValidateImageFile(header *multipart.FileHeader, cfg *config.Config) error {
	return ImageSvc.ValidateImage(header, cfg.AllowedTypes, cfg.MaxFileSize)
}

// ReadFileContent 读取文件内容
func ReadFileContent(header *multipart.FileHeader) ([]byte, error) {
	file, err := header.Open()
	if err != nil {
		return nil, fmt.Errorf("打开文件失败：%v", err)
	}
	defer file.Close()

	return io.ReadAll(file)
}

// GetFileMimeType 获取文件MIME类型
func GetFileMimeType(header *multipart.FileHeader) string {
	return header.Header.Get("Content-Type")
}

// generateUniqueFileName 生成唯一文件名
func generateUniqueFileName(ext string) string {
	timestamp := time.Now().UnixNano()
	hash := fmt.Sprintf("%x", timestamp)
	rand.New(rand.NewSource(time.Now().UnixNano()))
	randomNum := rand.Intn(900) + 100
	return fmt.Sprintf("%s%d%s", hash, randomNum, ext)
}
