package controllers

import (
	"net/url"
	"oneimg/backend/config"
	"strings"

	"github.com/gin-gonic/gin"

	"oneimg/backend/models"
	"oneimg/backend/utils/publicurl"
)

func buildImageResponseURL(c *gin.Context, setting models.Settings, storageType string, bucketID int, imagePath string) string {
	publicPath := applyPublicImageURL(setting, storageType, bucketID, imagePath)
	if publicPath == "" || strings.HasPrefix(publicPath, "http://") || strings.HasPrefix(publicPath, "https://") {
		return publicPath
	}
	return getRequestBaseURL(c) + ensureLeadingSlash(publicPath)
}

func applyPublicImageURL(setting models.Settings, storage string, bucketID int, path string) string {
	return publicurl.BuildForStorage(setting, storage, bucketID, path)
}

func rewriteImageURLs(setting models.Settings, image *models.Image) {
	if image.AccessBucketId > 0 {
		return
	}
	image.Url = applyPublicImageURL(setting, image.Storage, image.BucketId, image.Url)
	image.Thumbnail = applyPublicImageURL(setting, image.Storage, image.BucketId, image.Thumbnail)
}

func getRequestBaseURL(c *gin.Context) string {
	if config.App != nil {
		u, err := url.Parse(strings.TrimSpace(config.App.AppURL))
		if err == nil && (u.Scheme == "http" || u.Scheme == "https") && u.Hostname() != "" && u.User == nil && u.RawQuery == "" && u.Fragment == "" {
			return strings.TrimRight(u.String(), "/")
		}
	}
	// Invalid/missing AppURL produces a relative URL, not an attacker-host URL.
	return ""
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
