package middlewares

import (
	"oneimg/backend/config"

	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/memstore"
	"github.com/gin-gonic/gin"
)

var SessionStore sessions.Store

// SessionMiddleware 配置session中间件
func SessionMiddleware(cfg *config.Config) gin.HandlerFunc {
	// 使用cookie存储session
	SessionStore = memstore.NewStore(
		[]byte(cfg.SessionSecret),
	)

	// 配置session选项
	SessionStore.Options(sessions.Options{
		MaxAge:   24 * 60 * 60, // 24小时，单位秒
		HttpOnly: true,         // 防止XSS攻击
		Secure:   false,        // 生产环境应设为true（需要HTTPS）
		SameSite: 4,            // SameSiteStrictMode，防止CSRF攻击
		Path:     "/",          // cookie路径
	})

	return sessions.Sessions("oneimg-session", SessionStore)
}
