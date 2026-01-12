package interfaces

import (
	"mime/multipart"
	"oneimg/backend/config"
	"oneimg/backend/models"

	"github.com/gin-gonic/gin"
)

// 上传返回结构
type ImageUploadResult struct {
	Success      bool   `json:"success"`
	Message      string `json:"message,omitempty"`
	ID           int    `json:"id,omitempty"`
	URL          string `json:"url,omitempty"`
	ThumbnailURL string `json:"thumbnail_url,omitempty"`
	Storage      string `json:"storage,omitempty"`
	FileName     string `json:"filename,omitempty"`
	FileSize     int64  `json:"file_size,omitempty"`
	MimeType     string `json:"mime_type,omitempty"`
	Width        int    `json:"width,omitempty"`
	Height       int    `json:"height,omitempty"`
	CreatedAt    string `json:"created_at,omitempty"`
}

// Upload 上传处理接口
type StorageUploader interface {
	Upload(c *gin.Context, cfg *config.Config, setting *models.Settings, bucket *models.Buckets, fileHeader *multipart.FileHeader) (*ImageUploadResult, error)
}
