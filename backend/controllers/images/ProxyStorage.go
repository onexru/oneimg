package images

import (
	"bytes"
	"context"
	"errors"
	"log"
	"net/http"
	"time"

	"oneimg/backend/storage"
	"oneimg/backend/utils/result"
	"oneimg/backend/utils/watermark"

	"github.com/gin-gonic/gin"
)

func proxyStorageFile(c *gin.Context, driver storage.Driver, ref storage.Ref, mimeType, kind string, wm watermark.WatermarkConfig) {
	// 缓存命中就完全跳过 driver.Get。
	//
	// 这一步比只缓存「转码计算」更值：一次转码请求原本要付两笔成本——
	// 一次回源下载 + 一次解码重编码。命中缓存后两笔都省掉，
	// 对远端存储（S3/R2/Telegram）尤其明显，那些后端每次 Get 都是一次
	// 带外网往返的对象读取。
	cacheKey := ""
	if wm.Enable {
		cacheKey = transcodedCacheKey(normalizedCacheObjectKey(ref.Key, ref.Metadata), wm)
		if entry, ok := transcodedCache.get(cacheKey); ok {
			serveCachedTranscoded(c, entry)
			return
		}
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 60*time.Second)
	defer cancel()
	reader, err := driver.Get(ctx, ref)
	if err != nil {
		status, message := http.StatusBadGateway, "文件获取失败"
		switch {
		case errors.Is(err, storage.ErrNotFound):
			status, message = http.StatusNotFound, "文件不存在"
		case errors.Is(err, storage.ErrInvalidKey):
			status, message = http.StatusForbidden, "文件路径非法"
		case errors.Is(err, storage.ErrAccessDenied):
			status, message = http.StatusForbidden, "文件访问权限不足"
		case errors.Is(err, context.DeadlineExceeded):
			status, message = http.StatusGatewayTimeout, "存储请求超时"
		}
		c.JSON(status, result.Error(status, message))
		return
	}
	defer reader.Close()
	if err = serveStoredImage(c, reader, mimeType, kind, wm, cacheKey); err != nil {
		// Don't log credential-bearing upstream transport errors or signed URLs.
		log.Printf("[%s]图片传输失败", kind)
		if !c.Writer.Written() {
			c.JSON(http.StatusInternalServerError, result.Error(500, "文件读取失败"))
		}
	}
}

// serveCachedTranscoded 直接输出缓存里的转码结果。
//
// 响应头必须与「刚转码完」完全一致：如果缓存路径用了不同的 Cache-Control，
// 同一个 URL 会因命中与否返回不同的缓存策略，CDN 侧行为就变得不可预测。
func serveCachedTranscoded(c *gin.Context, entry *transcodedEntry) {
	if c.Request.Method != http.MethodGet && c.Request.Method != http.MethodHead {
		c.Header("Allow", "GET, HEAD")
		c.AbortWithStatus(http.StatusMethodNotAllowed)
		return
	}
	if entry.mimeType == "" {
		entry.mimeType = http.DetectContentType(entry.data)
	}
	imageResponseHeaders(c, entry.mimeType, "", false)
	applyTranscodedCacheHeaders(c)
	// ServeContent 会处理 Content-Length / Range / If-None-Match，
	// 手工 io.Copy 会丢掉 206 与条件请求的能力。
	http.ServeContent(c.Writer, c.Request, "", time.Time{}, bytes.NewReader(entry.data))
	c.Writer.WriteHeaderNow()
}
