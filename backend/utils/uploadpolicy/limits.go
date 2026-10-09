// Package uploadpolicy exposes the same effective policy to upload controllers
// and parsers. Invalid/zero stored values use defaults; settings edits reject them.
package uploadpolicy

import (
	"fmt"
	"oneimg/backend/config"
	"oneimg/backend/models"
	"strings"
	"unicode/utf8"
)

func bounded(value, fallback, ceiling int) int {
	if value < 1 || value > ceiling {
		return fallback
	}
	return value
}
func MaxFiles(s models.Settings) int {
	return bounded(s.MaxUploadFiles, config.DefaultMaxUploadFiles, config.HardMaxUploadFiles)
}
func TagLength(s models.Settings) int {
	return bounded(s.TagMaxLength, config.DefaultTagMaxLength, config.HardTagMaxLength)
}
func RandomLimit(s models.Settings) int {
	return bounded(s.RandomImageLimit, config.DefaultRandomImageLimit, config.HardRandomImageLimit)
}
func PowTimeoutSeconds(s models.Settings) int {
	return bounded(s.PowVerifyTimeoutSeconds, config.DefaultPowVerifyTimeoutSeconds, config.HardPowVerifyTimeoutSeconds)
}
func PowURL(s models.Settings) string {
	if strings.TrimSpace(s.PowVerifyURL) == "" {
		return config.LegacyPowVerifyURL
	}
	return strings.TrimSpace(s.PowVerifyURL)
}

// RequestBytes keeps the existing hard per-file raster ceiling plus multipart overhead.
func RequestBytes(s models.Settings) int64 { return int64(MaxFiles(s))*(32<<20) + (1 << 20) }
func ValidateCount(s models.Settings, count int) error {
	if count < 1 {
		return fmt.Errorf("请选择要上传的图片")
	}
	if count > MaxFiles(s) {
		return fmt.Errorf("最多只能上传%d个文件", MaxFiles(s))
	}
	return nil
}
func ValidateTag(s models.Settings, tag string) error {
	if !utf8.ValidString(tag) || strings.TrimSpace(tag) == "" {
		return fmt.Errorf("标签名称不能为空且必须是有效文本")
	}
	if utf8.RuneCountInString(tag) > TagLength(s) {
		return fmt.Errorf("标签名称不能超过%d个字符", TagLength(s))
	}
	return nil
}
