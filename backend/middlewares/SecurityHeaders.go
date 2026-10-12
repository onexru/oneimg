package middlewares

import (
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"oneimg/backend/config"
	"oneimg/backend/utils/settings"
)

// cspCache 缓存派生出的 CSP 头字符串，使每请求成本保持为一次
// mutex+时间检查，而不是复制整行 settings。
var cspCache struct {
	mu      sync.Mutex
	value   string
	expires time.Time
}

const cspCacheTTL = 5 * time.Second

// httpsOrigin 提取 URL 的 https 源（scheme://host）。非 https、带凭据
// 或解析失败一律返回空串——绝不放宽策略。
func httpsOrigin(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.Host == "" || u.User != nil {
		return ""
	}
	return "https://" + u.Host
}

// powProviderOrigins 返回管理员配置的 legacy POW 提供方源（脚本与组件
// 两个 URL），仅接受 https。内置默认提供方 https://cha.eta.im 恒在白名单
// 内（与前端 LEGACY_POW_*_URL 回退值一致），自定义源按配置追加。
func powProviderOrigins() []string {
	origins := []string{"https://cha.eta.im"}
	row, err := settings.GetSettings()
	if err != nil {
		return origins
	}
	for _, raw := range []string{row.PowScriptURL, row.PowWidgetURL} {
		if o := httpsOrigin(raw); o != "" {
			origins = append(origins, o)
		}
	}
	return origins
}

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

	pow := strings.Join(uniqueStrings(powProviderOrigins()), " ")
	scriptSrc := "'self' https://challenges.cloudflare.com " + pow
	connectSrc := "'self' https://api.github.com https://challenges.cloudflare.com " + pow
	frameSrc := "'self' https://challenges.cloudflare.com " + pow

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

func uniqueStrings(in []string) []string {
	seen := make(map[string]struct{}, len(in))
	out := make([]string, 0, len(in))
	for _, s := range in {
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	return out
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
