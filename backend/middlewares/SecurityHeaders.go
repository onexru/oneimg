package middlewares

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"oneimg/backend/config"
)

// SecurityHeaders applies the SPA policy before handlers, so image delivery can
// replace it with the stricter sandbox policy required for legacy SVG objects.
func SecurityHeaders(cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("X-Frame-Options", "DENY")
		c.Header("Referrer-Policy", "strict-origin-when-cross-origin")
		c.Header("Permissions-Policy", "camera=(), microphone=(), geolocation=(), payment=()")
		c.Header("Content-Security-Policy", "default-src 'self'; script-src 'self' https:; style-src 'self' 'unsafe-inline'; img-src 'self' data: blob: https: http:; font-src 'self' data:; connect-src 'self' https: http:; frame-src https:; object-src 'none'; base-uri 'self'; form-action 'self'; frame-ancestors 'none'; worker-src 'self' blob:")
		if cfg != nil && strings.HasPrefix(strings.ToLower(cfg.AppURL), "https://") {
			c.Header("Strict-Transport-Security", "max-age=31536000")
		}
		if strings.HasPrefix(c.Request.URL.Path, "/api/") {
			c.Header("Cache-Control", "private, no-store")
			c.Header("Pragma", "no-cache")
		}
		c.Next()
	}
}

// MethodNotAllowed returns an explicit HTTP status, not a success-body error.
func MethodNotAllowed(c *gin.Context) {
	c.JSON(http.StatusMethodNotAllowed, gin.H{"code": http.StatusMethodNotAllowed, "message": "请求方法不允许"})
}
