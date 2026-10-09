package controllers

import (
	"context"
	"mime"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"

	"oneimg/backend/models"
	"oneimg/backend/utils/buckets"
	"oneimg/backend/utils/watermark"

	"github.com/gin-gonic/gin"
	"github.com/minio/minio-go/v7"
)

// Stable OneImg URLs never expire. Only the object-scoped R2 bearer URL does.
const r2SignedURLLifetime = 30 * time.Minute

func safeR2RasterContentType(contentType string) bool {
	switch contentType {
	case "image/jpeg", "image/png", "image/gif", "image/webp", "image/avif", "image/bmp":
		return true
	default:
		return false
	}
}

// redirectR2Object runs only after ImageProxy has checked the database record,
// referer policy, successful access-source replica and bucket enabled state.
// A HEAD request inspects legacy objects without transferring their image bytes.
// Encrypted (octet-stream) objects and dynamic watermarks retain server processing.
// false means the existing proxy must handle the request; true means it is done.
func redirectR2Object(c *gin.Context, imageURL, bucketName string, bucket models.Buckets, client *minio.Client, wm watermark.WatermarkConfig) bool {
	if wm.Enable {
		return false
	}
	if c.Request.Method != http.MethodGet && c.Request.Method != http.MethodHead {
		c.Header("Allow", "GET, HEAD")
		c.AbortWithStatus(http.StatusMethodNotAllowed)
		return true
	}
	objectKey := strings.TrimPrefix(imageURL, "/")
	if objectKey == "" || bucketName == "" {
		c.AbortWithStatus(http.StatusNotFound)
		return true
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 15*time.Second)
	defer cancel()
	info, err := client.StatObject(ctx, bucketName, objectKey, minio.StatObjectOptions{})
	if err != nil {
		// Never silently consume server bandwidth after signing/metadata failures.
		errorCode := minio.ToErrorResponse(err).Code
		if errorCode == "NoSuchKey" || errorCode == "NoSuchObject" || errorCode == "NotFound" {
			c.AbortWithStatus(http.StatusNotFound)
		} else {
			c.AbortWithStatusJSON(http.StatusBadGateway, gin.H{"error": "无法读取 R2 对象信息"})
		}
		return true
	}
	contentType := strings.ToLower(strings.TrimSpace(strings.SplitN(info.ContentType, ";", 2)[0]))
	// Existing writers mark encrypted originals/thumbnails as octet-stream.
	// Do not infer encryption from today's global setting: old encrypted files
	// must still decrypt even after users turn encryption off.
	// Redirect only inert raster formats. SVG (including legacy objects with
	// forged Content-Type metadata) must remain behind the sandboxed proxy.
	// Never hand an active document to a public CDN without our response policy.
	ext := strings.ToLower(path.Ext(objectKey))
	if ext == ".svg" || ext == ".svgz" || !safeR2RasterContentType(contentType) {
		return false
	}
	domain, cdnMode, err := buckets.R2CDNConfig(bucket.Config)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusBadGateway, gin.H{"error": "R2 CDN 配置无效，请检查存储设置"})
		return true
	}
	if domain != "" {
		base, _ := url.Parse(domain)
		if strings.EqualFold(base.Host, c.Request.Host) && base.Path == "" {
			c.AbortWithStatusJSON(http.StatusBadGateway, gin.H{"error": "CDN 地址不能指向本站，避免循环跳转"})
			return true
		}
	}
	if domain != "" && cdnMode == buckets.R2CDNPublic && c.Query("download") != "1" {
		target, err := buckets.R2CDNObjectURL(domain, objectKey)
		if err != nil {
			c.AbortWithStatus(http.StatusBadGateway)
			return true
		}
		c.Header("Cache-Control", "private, no-store")
		c.Header("Referrer-Policy", "no-referrer")
		c.Header("X-Storage-Type", bucket.Type)
		c.Header("X-Image-Delivery", "r2-cdn-public")
		c.Header("Location", target)
		c.Status(http.StatusFound)
		return true
	}

	params := url.Values{"response-cache-control": {"private, no-store"}}
	if c.Query("download") == "1" {
		params.Set("response-content-disposition", mime.FormatMediaType("attachment", map[string]string{
			"filename": path.Base(objectKey),
		}))
	}
	// Signing includes the HTTP method; GET signatures cannot service HEAD.
	// No arbitrary client query parameters or object paths are forwarded.
	signed, err := client.Presign(ctx, c.Request.Method, bucketName, objectKey, r2SignedURLLifetime, params)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusBadGateway, gin.H{"error": "无法生成 R2 临时访问链接"})
		return true
	}

	c.Header("Cache-Control", "private, no-store")
	c.Header("Pragma", "no-cache")
	c.Header("Referrer-Policy", "no-referrer")
	c.Header("X-Storage-Type", bucket.Type)
	c.Header("X-Image-Delivery", "r2-presigned")
	target := signed.String()
	if domain != "" && cdnMode == buckets.R2CDNSignedProxy {
		target, err = buckets.R2CDNSignedURL(domain, signed)
		if err != nil {
			c.AbortWithStatus(http.StatusBadGateway)
			return true
		}
		c.Header("X-Image-Delivery", "r2-cdn-signed-proxy")
	}
	c.Header("Location", target)
	// Empty redirect body: this server never opens or streams the image.
	c.Status(http.StatusFound)
	return true
}
