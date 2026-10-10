package uploads

import (
	"oneimg/backend/models"
	"oneimg/backend/utils/publicurl"
	"strings"
)

// Legacy factory names are compatibility aliases; every provider uses the same
// artifact pipeline and shared raw storage driver.
type R2Uploader = driverUploader
type S3Uploader = driverUploader
type WebDAVUploader = driverUploader
type DefaultUploader = driverUploader
type FTPUploader = driverUploader
type TelegramUploader = driverUploader

func getProcessingSettings(setting *models.Settings, bucket *models.Buckets) models.Settings {
	result := *setting
	if publicurl.HasDomain(*setting) && publicurl.SupportsStorage(bucket.Type) {
		result.WatermarkEnable = false
	}
	return result
}
func storageContentType(contentType string, encrypted bool) string {
	if encrypted {
		return "application/octet-stream"
	}
	return contentType
}
func PathJoin(parts ...string) string { return strings.Join(parts, "/") }
