package middlewares

import (
	"net/http"
	"strings"

	"oneimg/backend/config"
	"oneimg/backend/utils/result"

	"github.com/gin-gonic/gin"
)

// 请求方法 → 动作分类：只有「写操作」在演示模式下被拒绝。
// GET / HEAD / OPTIONS 天然只读，直接放行；其余（POST/PUT/PATCH/DELETE）按路径白名单放行。
var demoWriteBlockedHandler = func(c *gin.Context) {
	c.JSON(http.StatusForbidden, result.Error(http.StatusForbidden, "演示模式：禁止修改操作，仅可浏览与查询"))
	c.Abort()
}

// demoAllowedWritePrefixes：演示模式下仍然放行的写操作路径。
// 原则：只放行「不产生持久数据、或不属于站点内容」的操作——
// 登录/登出（会话本身）、验证（人机验证令牌）、搜索等。
var demoAllowedWritePrefixes = []string{
	"/api/login",          // 登录（含游客登录）
	"/api/logout",         // 登出
	"/api/auth/",          // SSO 流程（登录的一部分）
	"/api/verify/",        // 人机验证令牌签发/校验
	"/api/settings/login", // 登录页读取的是 GET；防御性保留前缀
}

// DemoModeMiddleware：演示模式下的全站只读闸门。
//
// 规则：
//   - DEMO_MODE 未启用 → 直接放行（普通部署零影响）。
//   - GET/HEAD/OPTIONS → 放行（浏览、查询、下载不受影响）。
//   - 其余方法（POST/PUT/PATCH/DELETE）→ 仅登录/登出/SSO/验证类路径放行，
//     其余一律 403「演示模式：禁止修改操作」。
//
// 该中间件挂在路由表最外层（AuthMiddleware 之前），保证上传、删除、
// 设置修改、存储增删、用户管理等所有写接口在进入业务逻辑前就被拦截。
func DemoModeMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !config.App.DemoMode {
			c.Next()
			return
		}

		switch c.Request.Method {
		case http.MethodGet, http.MethodHead, http.MethodOptions:
			c.Next()
			return
		}

		path := c.Request.URL.Path
		for _, prefix := range demoAllowedWritePrefixes {
			if strings.HasPrefix(path, prefix) {
				c.Next()
				return
			}
		}

		demoWriteBlockedHandler(c)
	}
}
