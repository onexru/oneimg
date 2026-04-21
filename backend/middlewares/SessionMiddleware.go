package middlewares

import (
	"net/http"
	"oneimg/backend/config"
	"strings"

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
		Secure:   isHTTPS(cfg.AppURL),
		SameSite: http.SameSiteStrictMode,
		Path:     "/",          // cookie路径
	})

	return sessions.Sessions("oneimg-session", SessionStore)
}

func isHTTPS(rawURL string) bool {
	return strings.HasPrefix(strings.ToLower(strings.TrimSpace(rawURL)), "https://")
}
