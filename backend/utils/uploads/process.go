package uploads

import (
	"fmt"
	"github.com/gin-gonic/gin"
	"mime/multipart"
	"oneimg/backend/models"
	"oneimg/backend/utils/images"
)

func processUpload(c *gin.Context, setting *models.Settings, bucket *models.Buckets, header *multipart.FileHeader) (*images.ProcessedImage, error) {
	if header == nil || header.Size <= 0 || header.Size > images.UploadByteLimit(int64(setting.MaxFileSize)) {
		return nil, images.ErrFileTooLarge
	}
	file, err := header.Open()
	if err != nil {
		return nil, fmt.Errorf("打开文件失败: %w", err)
	}
	defer file.Close()
	return images.ImageSvc.ProcessImage(file, header, getProcessingSettings(setting, bucket), c.GetInt("user_role"))
}
