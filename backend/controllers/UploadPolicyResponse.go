package controllers

import (
	"oneimg/backend/models"
	"oneimg/backend/utils/images"
	"strings"
)

// Return only raster formats the actual decoder and current policy both allow.
func effectiveUploadMIMEs(setting models.Settings) []string {
	allowed := make([]string, 0, 4)
	for _, mime := range []string{"image/jpeg", "image/png", "image/gif", "image/webp"} {
		if images.AllowedMIME(mime, strings.Split(setting.AllowedTypes, ",")) {
			allowed = append(allowed, mime)
		}
	}
	return allowed
}
