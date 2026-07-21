package middlewares

import (
	"net/http"
	"strings"

	"oneimg/backend/config"

	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/memstore"
	"github.com/gin-gonic/gin"
)

// SessionStore 全局 Session 存储（进程内内存）。
var SessionStore sessions.Store

// SessionMiddleware 初始化 Cookie Session，并挂载到 Gin。
func SessionMiddleware(cfg *config.Config) gin.HandlerFunc {
	SessionStore = memstore.NewStore([]byte(cfg.SessionSecret))
	SessionStore.Options(sessions.Options{
		MaxAge:   24 * 60 * 60, // 24 小时
		HttpOnly: true,
		Secure:   isHTTPS(cfg.AppURL),
		SameSite: http.SameSiteStrictMode,
		Path:     "/",
	})
	return sessions.Sessions("oneimg-session", SessionStore)
}

// isHTTPS 根据配置的 AppURL 判断是否应启用 Secure Cookie。
func isHTTPS(rawURL string) bool {
	return strings.HasPrefix(strings.ToLower(strings.TrimSpace(rawURL)), "https://")
}
