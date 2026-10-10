package uploads

import (
	"errors"
	"fmt"
	"mime/multipart"
	"net/http"

	"oneimg/backend/interfaces"
	"oneimg/backend/models"
	"oneimg/backend/utils/result"
	"oneimg/backend/utils/settings"
	"oneimg/backend/utils/uploadpolicy"

	"github.com/gin-gonic/gin"
)

const (
	MaxUploadFiles           = 10 // Deprecated: use uploadpolicy.MaxFiles for runtime policy.
	MaxRequestBytes    int64 = (MaxUploadFiles * (32 << 20)) + (1 << 20)
	DefaultStorageType       = "default"
)

// UploadContext 上传上下文
type UploadContext struct {
	c *gin.Context
}

// NewUploadContext 创建上传上下文
func NewUploadContext(c *gin.Context) *UploadContext {
	return &UploadContext{c: c}
}

// Gin 返回底层 gin 上下文（供 controllers 层统一返回结果）
func (uc *UploadContext) Gin() *gin.Context {
	return uc.c
}

// Fail 统一错误返回
func (uc *UploadContext) Fail(code int, format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	uc.c.JSON(code, result.Error(code, msg))
}

// Success 统一成功返回
func (uc *UploadContext) Success(msg string, data map[string]any) {
	uc.c.JSON(http.StatusOK, result.Success(msg, data))
}

// ParseAndValidateFiles 解析并校验上传文件（数量、非空）
func (uc *UploadContext) ParseAndValidateFiles() ([]*multipart.FileHeader, error) {
	policy, err := settings.GetSettings()
	if err != nil {
		return nil, fmt.Errorf("读取上传限制失败: %w", err)
	}
	return uc.ParseAndValidateFilesWithSettings(policy)
}

func (uc *UploadContext) ParseAndValidateFilesWithSettings(policy models.Settings) ([]*multipart.FileHeader, error) {
	uc.c.Request.Body = http.MaxBytesReader(uc.c.Writer, uc.c.Request.Body, uploadpolicy.RequestBytes(policy))
	// 解析表单
	err := uc.c.Request.ParseMultipartForm(1 << 20)
	form := uc.c.Request.MultipartForm
	if err != nil {
		return nil, fmt.Errorf("解析表单失败：%v", err)
	}

	// 获取文件列表
	files := form.File["images[]"]
	if len(files) == 0 {
		return nil, errors.New("请选择要上传的图片")
	}

	// 校验文件数量
	if err := uploadpolicy.ValidateCount(policy, len(files)); err != nil {
		return nil, err
	}

	return files, nil
}

// GetStorageUploader 根据存储类型获取上传器实例
func (uc *UploadContext) GetStorageUploader(setting *models.Settings, bucket *models.Buckets) (interfaces.StorageUploader, error) {
	switch bucket.Type {
	case "default":
		return &DefaultUploader{}, nil
	case "r2":
		return &R2Uploader{}, nil
	case "s3":
		return &S3Uploader{}, nil
	case "webdav":
		return &WebDAVUploader{}, nil
	case "ftp":
		return &FTPUploader{}, nil
	case "telegram":
		return &TelegramUploader{}, nil
	default:
		return nil, fmt.Errorf("不支持的存储类型：%s", bucket.Type)
	}
}
