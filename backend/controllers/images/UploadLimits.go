package images

import (
	"errors"
	"fmt"
	"mime"
	"net/http"
	"strings"

	"mime/multipart"
	"oneimg/backend/models"
	"oneimg/backend/utils/uploadpolicy"
	"oneimg/backend/utils/uploads"

	"github.com/gin-gonic/gin"
)

var (
	// ErrNotMultipartForm：Content-Type 不是 multipart/form-data。
	// 单独成一条错误，是为了和「字段名不对」区分开 —— 两者以前共用
	// 「文件解析失败」前缀，使用者看不出到底该改哪里。
	ErrNotMultipartForm = errors.New(`请求 Content-Type 必须是 multipart/form-data，文件字段名使用 images[]`)
	// ErrNoUploadField：是 multipart 请求，但没有取到任何文件。
	ErrNoUploadField = errors.New(`未找到上传文件：multipart 字段名应为 images[]（兼容 file / file[]）`)
)

// parseConfiguredUploadFiles 是 /api/upload 与 /api/upload/images 共用的文件解析入口。
//
// 解析是幂等的：已经解析过就直接复用，避免同一请求反复走 multipart 解析
// （历史上 ImageUpload 入口与分支各解析一次，第一次的结果被直接丢弃）。
func parseConfiguredUploadFiles(c *gin.Context, s models.Settings) ([]*multipart.FileHeader, error) {
	if c.Request.MultipartForm == nil {
		if err := c.Request.ParseMultipartForm(1 << 20); err != nil {
			var tooLarge *http.MaxBytesError
			if errors.As(err, &tooLarge) {
				return nil, err
			}
			if !isMultipartFormData(c.ContentType()) {
				return nil, ErrNotMultipartForm
			}
			return nil, fmt.Errorf("解析上传表单失败：%v", err)
		}
	}
	if c.Request.MultipartForm == nil {
		return nil, ErrNoUploadField
	}
	files := uploads.PickUploadFiles(c.Request.MultipartForm)
	if len(files) == 0 {
		return nil, ErrNoUploadField
	}
	if err := uploadpolicy.ValidateCount(s, len(files)); err != nil {
		return nil, err
	}
	return files, nil
}

func isMultipartFormData(contentType string) bool {
	mediaType, _, err := mime.ParseMediaType(contentType)
	if err != nil {
		return false
	}
	return strings.EqualFold(mediaType, "multipart/form-data")
}

func uploadParseErrorStatus(err error) int {
	var tooLarge *http.MaxBytesError
	if errors.As(err, &tooLarge) {
		return http.StatusRequestEntityTooLarge
	}
	return http.StatusBadRequest
}

// failUploadParse 统一输出上传解析失败的响应。
// 413 与 400 分开，且提示语保留具体原因（缺字段 / Content-Type 不对 / 超过数量上限）。
func failUploadParse(uc *uploads.UploadContext, err error) {
	status := uploadParseErrorStatus(err)
	message := err.Error()
	if status == http.StatusRequestEntityTooLarge {
		message = "上传内容超过请求体上限：请减少文件数量或缩小单张图片"
	}
	uc.Fail(status, "%s", message)
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
