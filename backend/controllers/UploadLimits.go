package controllers

import (
	"errors"
	"fmt"
	"github.com/gin-gonic/gin"
	"mime/multipart"
	"net/http"
	"oneimg/backend/models"
	"oneimg/backend/utils/uploadpolicy"
)

var uploadRequestSlots = make(chan struct{}, 2)

func acquireUploadRequest(c *gin.Context) (func(), bool) {
	select {
	case uploadRequestSlots <- struct{}{}:
		return func() { <-uploadRequestSlots }, true
	default:
		c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{"code": 429, "message": "上传处理中，请稍后重试"})
		return nil, false
	}
}

// parseConfiguredUploadFiles replaces the legacy fixed-count parser at both
// controller entry points without changing storage uploader interfaces.
func parseConfiguredUploadFiles(c *gin.Context, s models.Settings) ([]*multipart.FileHeader, error) {
	if err := c.Request.ParseMultipartForm(1 << 20); err != nil {
		return nil, err
	}
	if c.Request.MultipartForm == nil {
		return nil, fmt.Errorf("请选择要上传的图片")
	}
	files := c.Request.MultipartForm.File["images[]"]
	if err := uploadpolicy.ValidateCount(s, len(files)); err != nil {
		return nil, err
	}
	return files, nil
}
func uploadParseErrorStatus(err error) int {
	var tooLarge *http.MaxBytesError
	if errors.As(err, &tooLarge) {
		return http.StatusRequestEntityTooLarge
	}
	return http.StatusBadRequest
}
func validateUploadTags(s models.Settings, tags []string) error {
	if len(tags) > 100 {
		return fmt.Errorf("每次最多选择100个标签")
	}
	for _, tag := range tags {
		if err := uploadpolicy.ValidateTag(s, tag); err != nil {
			return err
		}
	}
	return nil
}
