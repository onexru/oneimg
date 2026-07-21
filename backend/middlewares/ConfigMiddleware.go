package middlewares

import (
	"oneimg/backend/config"

	"github.com/gin-gonic/gin"
)

// ConfigMiddleware 将应用配置注入到请求上下文（键：config）。
func ConfigMiddleware(cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Set("config", cfg)
		c.Next()
	}
}
