package controllers

import (
	"context"
	"errors"
	"github.com/gin-gonic/gin"
	"log"
	"net/http"
	"oneimg/backend/storage"
	"oneimg/backend/utils/result"
	"oneimg/backend/utils/watermark"
	"time"
)

func proxyStorageFile(c *gin.Context, driver storage.Driver, ref storage.Ref, mimeType, kind string, wm watermark.WatermarkConfig) {
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
	if err = serveStoredImage(c, reader, mimeType, kind, wm); err != nil {
		// Don't log credential-bearing upstream transport errors or signed URLs.
		log.Printf("[%s]图片传输失败", kind)
		if !c.Writer.Written() {
			c.JSON(http.StatusInternalServerError, result.Error(500, "文件读取失败"))
		}
	}
}
