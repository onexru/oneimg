package uploads

import (
	"errors"
	"fmt"
	"mime/multipart"
	"net/http"

	"oneimg/backend/interfaces"
	"oneimg/backend/models"
	"oneimg/backend/utils/result"

	"github.com/gin-gonic/gin"
)

const (
	MaxUploadFiles     = 10
	DefaultStorageType = "default"
)

// UploadContext 上传上下文
type UploadContext struct {
	c *gin.Context
}

// NewUploadContext 创建上传上下文
func NewUploadContext(c *gin.Context) *UploadContext {
	return &UploadContext{c: c}
}

// Fail 统一错误返回
func (uc *UploadContext) Fail(code int, format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	uc.c.JSON(http.StatusOK, result.Error(code, msg))
}

// Success 统一成功返回
func (uc *UploadContext) Success(msg string, data map[string]any) {
	uc.c.JSON(http.StatusOK, result.Success(msg, data))
}

// ParseAndValidateFiles 解析并校验上传文件（数量、非空）
func (uc *UploadContext) ParseAndValidateFiles() ([]*multipart.FileHeader, error) {
	// 解析表单
	form, err := uc.c.MultipartForm()
	if err != nil {
		return nil, fmt.Errorf("解析表单失败：%v", err)
	}

	// 获取文件列表
	files := form.File["images[]"]
	if len(files) == 0 {
		return nil, errors.New("请选择要上传的图片")
	}

	// 校验文件数量
	if len(files) > MaxUploadFiles {
		return nil, fmt.Errorf("最多只能上传%d个文件", MaxUploadFiles)
	}

	return files, nil
}

// GetStorageUploader 根据存储类型获取上传器实例
func (uc *UploadContext) GetStorageUploader(setting *models.Settings, bucket *models.Buckets) (interfaces.StorageUploader, error) {
	switch bucket.Type {
	case "default":
		return &DefaultUploader{}, nil
	case "s3", "r2":
		return &S3R2Uploader{}, nil
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
