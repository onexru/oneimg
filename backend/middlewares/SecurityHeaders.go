package middlewares

import (
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"oneimg/backend/config"
	"oneimg/backend/models"
	"oneimg/backend/utils/settings"
	"oneimg/backend/utils/verifypolicy"
)

// cspCache 缓存派生出的 CSP 头字符串，使每请求成本保持为一次
// mutex+时间检查，而不是复制整行 settings。
var cspCache struct {
	mu      sync.Mutex
	value   string
	expires time.Time
}

const cspCacheTTL = 5 * time.Second

// buildCSP 生成收紧后的 CSP：
//   - script-src 收敛为 'self' + Turnstile + POW 提供方（原先的 https:
//     通配允许 XSS 从任意外部域拉取载荷）；
//   - connect-src 收敛为 'self' + GitHub releases 检查 + Turnstile + POW
//     提供方（原先放行任意 http/https）；
//   - frame-src 同步收敛；
//   - img-src 保持宽松：自定义 CDN 域名（r2_cdn_domain）是管理员可配置
//     项且以 <img> 呈现，图片本身不执行脚本。
func buildCSP() string {
	cspCache.mu.Lock()
	if cspCache.expires.After(time.Now()) {
		v := cspCache.value
		cspCache.mu.Unlock()
		return v
	}
	cspCache.mu.Unlock()

	var method string
	if row, err := settings.GetSettings(); err == nil {
		// 必须用带 fallback 的判定：管理员选了 Turnstile 但密钥没配全时，
		// 实际生效的是 cap-pow。若这里仍按库里存的 'turnstile' 判，CSP 就不会
		// 放行 'wasm-unsafe-eval'，cap-widget 的 WebAssembly.compile() 会被
		// 浏览器直接拒绝，回退形同虚设 —— 用户卡在「请完成人机验证」且组件
		// 永远解不出题（实测报错：violates "script-src 'self' https://challenges.cloudflare.com"）。
		method = verifypolicy.EffectiveMethod(row)
	}

	// script-src 默认不放行 wasm-unsafe-eval。
	scriptSrc := "'self' https://challenges.cloudflare.com"
	if needsWasm(method) {
		// cap-pow 的求解器在浏览器里跑 WebAssembly。
		// 少了这条指令，instantiateStreaming 会被 CSP 直接拦截，
		// 组件会永远停在「验证中」并无限重试拉取 wasm。
		// 只对真正需要 wasm 的验证方式放行，其余场景维持原策略。
		scriptSrc += " 'wasm-unsafe-eval'"
	}
	connectSrc := "'self' https://api.github.com https://challenges.cloudflare.com"
	frameSrc := "'self' https://challenges.cloudflare.com"

	value := "default-src 'self'; script-src " + scriptSrc +
		"; style-src 'self' 'unsafe-inline'; img-src 'self' data: blob: https: http:" +
		"; font-src 'self' data:; connect-src " + connectSrc +
		"; frame-src " + frameSrc +
		"; object-src 'none'; base-uri 'self'; form-action 'self'; frame-ancestors 'none'; worker-src 'self' blob:"

	cspCache.mu.Lock()
	cspCache.value = value
	cspCache.expires = time.Now().Add(cspCacheTTL)
	cspCache.mu.Unlock()
	return value
}

// needsWasm 判断当前验证方式是否在浏览器里跑 WebAssembly。
// cap-pow 的求解器是 wasm；Turnstile 与无验证都不需要。
func needsWasm(method string) bool {
	return method == models.VerifyMethodCappow
}

// SecurityHeaders applies the SPA policy before handlers, so image delivery can
// replace it with the stricter sandbox policy required for legacy SVG objects.
func SecurityHeaders(cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("X-Frame-Options", "DENY")
		c.Header("Referrer-Policy", "strict-origin-when-cross-origin")
		c.Header("Permissions-Policy", "camera=(), microphone=(), geolocation=(), payment=()")
		c.Header("Content-Security-Policy", buildCSP())
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
