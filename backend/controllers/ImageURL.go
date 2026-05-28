package controllers

import (
	"strings"

	"github.com/gin-gonic/gin"

	"oneimg/backend/models"
	"oneimg/backend/utils/publicurl"
)

func buildImageResponseURL(c *gin.Context, setting models.Settings, storage string, path string) string {
	publicPath := publicurl.BuildForStorage(setting, storage, path)
	if publicPath == "" || strings.HasPrefix(publicPath, "http://") || strings.HasPrefix(publicPath, "https://") {
		return publicPath
	}
	return getRequestBaseURL(c) + ensureLeadingSlash(publicPath)
}

func applyPublicImageURL(setting models.Settings, storage string, path string) string {
	return publicurl.BuildForStorage(setting, storage, path)
}

func rewriteImageURLs(setting models.Settings, image *models.Image) {
	image.Url = applyPublicImageURL(setting, image.Storage, image.Url)
	image.Thumbnail = applyPublicImageURL(setting, image.Storage, image.Thumbnail)
}

func getRequestBaseURL(c *gin.Context) string {
	scheme := "http"
	if proto := firstForwardedValue(c.GetHeader("X-Forwarded-Proto")); proto != "" {
		scheme = proto
	} else if c.Request.TLS != nil {
		scheme = "https"
	}

	host := c.Request.Host
	if forwardedHost := firstForwardedValue(c.GetHeader("X-Forwarded-Host")); forwardedHost != "" {
		host = forwardedHost
	}

	return strings.TrimSuffix(scheme+"://"+host, "/")
}

func ensureLeadingSlash(path string) string {
	if path == "" || strings.HasPrefix(path, "/") {
		return path
	}
	return "/" + path
}

func firstForwardedValue(value string) string {
	if value == "" {
		return ""
	}
	return strings.TrimSpace(strings.Split(value, ",")[0])
}
