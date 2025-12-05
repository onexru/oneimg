package middlewares

import (
	"oneimg/backend/config"

	"github.com/gin-gonic/gin"
)

// ConfigMiddleware 配置中间件，将配置实例注入到上下文中
func ConfigMiddleware(cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Set("config", cfg)
		c.Next()
	}
}
