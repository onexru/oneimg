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

// CSP 的取用纪律：请求路径上绝不查库。
//
// SecurityHeaders 是注册顺序里的第一道全局中间件，每个响应（含 /assets/*、
// /uploads/** 这些静态资源）都要过一次。只要它在这里等一次数据库往返，
// 就把全站可用性绑在了数据库的连接可用性上 —— SQLite 的连接池只有 1 条
// （backend/database/pool.go），任何一个长事务占着连接，所有请求就一起排队。
//
// 所以这里只做两件事：命中缓存直接返回；过期了就先用旧值（stale-while-
// revalidate）并在后台刷新，绝不让响应等刷新完成。
var cspCache struct {
	mu         sync.Mutex
	value      string
	expires    time.Time
	refreshing bool
	// done 非 nil 表示有刷新在飞行中，供 WarmCSP 做有界等待。
	done chan struct{}
}

const (
	cspCacheTTL = 5 * time.Second
	// 读设置失败时的重试间隔：比正常 TTL 短，但足够挡住「数据库挂着时
	// 每个请求都去撞一次」的放大效应。
	cspRetryTTL = 2 * time.Second
)

// bootCSP 是「还什么都不知道」时用的兜底值。见 deriveCSP 对空 method 的处理。
var bootCSP = deriveCSP("")

// deriveCSP 生成收紧后的 CSP：
//   - script-src 收敛为 'self' + Turnstile + POW 提供方（原先的 https:
//     通配允许 XSS 从任意外部域拉取载荷）；
//   - connect-src 收敛为 'self' + GitHub releases 检查 + Turnstile + POW
//     提供方（原先放行任意 http/https）；
//   - frame-src 同步收敛；
//   - img-src 保持宽松：自定义 CDN 域名（r2_cdn_domain）是管理员可配置
//     项且以 <img> 呈现，图片本身不执行脚本。
//
// method 为空串表示「读不到设置、不知道当前用哪种验证」，此时按需要 wasm
// 处理：漏放的代价是所有人卡在「请完成人机验证」且永远解不出题（无法登录），
// 多放的代价只是允许本地 wasm 编译 —— 不放行远程脚本，权衡明确。
func deriveCSP(method string) string {
	// script-src 默认不放行 wasm-unsafe-eval，只对确实需要它的场景开。
	scriptSrc := "'self' https://challenges.cloudflare.com"
	if method == "" || needsWasm(method) {
		// cap-pow 的求解器在浏览器里跑 WebAssembly。
		// 少了这条指令，instantiateStreaming 会被 CSP 直接拦截，
		// 组件会永远停在「验证中」并无限重试拉取 wasm。
		scriptSrc += " 'wasm-unsafe-eval'"
	}
	connectSrc := "'self' https://api.github.com https://challenges.cloudflare.com"
	frameSrc := "'self' https://challenges.cloudflare.com"

	return "default-src 'self'; script-src " + scriptSrc +
		"; style-src 'self' 'unsafe-inline'; img-src 'self' data: blob: https: http:" +
		"; font-src 'self' data:; connect-src " + connectSrc +
		"; frame-src " + frameSrc +
		"; object-src 'none'; base-uri 'self'; form-action 'self'; frame-ancestors 'none'; worker-src 'self' blob:"
}

// buildCSP 返回当前应使用的 CSP，保证不阻塞。
func buildCSP() string {
	cspCache.mu.Lock()
	value, expired := cspCache.value, !time.Now().Before(cspCache.expires)
	cspCache.mu.Unlock()

	if value == "" {
		// 启动瞬间还没成功读到过设置：先给兜底值，同时后台去取。
		refreshCSPAsync()
		return bootCSP
	}
	if expired {
		// 过期不阻塞：旧值再顶一会儿，后台刷新。读的还是同一行设置，
		// 5 秒的滞后不会让 CSP 与实际验证方式错到不可用。
		refreshCSPAsync()
	}
	return value
}

func refreshCSPAsync() {
	cspCache.mu.Lock()
	busy := cspCache.refreshing
	cspCache.mu.Unlock()
	if busy {
		return
	}
	go refreshCSP(0)
}

// refreshCSP 刷新缓存值。wait > 0 时最多等这么久（供启动期预热使用）。
//
// 无论成功与否都会释放 refreshing 与 done，否则后续请求会永远看到 busy。
func refreshCSP(wait time.Duration) {
	cspCache.mu.Lock()
	if cspCache.refreshing {
		done := cspCache.done
		cspCache.mu.Unlock()
		if done != nil && wait > 0 {
			select {
			case <-done:
			case <-time.After(wait):
			}
		}
		return
	}
	done := make(chan struct{})
	cspCache.refreshing = true
	cspCache.done = done
	cspCache.mu.Unlock()

	go func() {
		// 必须用带 fallback 的判定：管理员选了 Turnstile 但密钥没配全时，
		// 实际生效的是 cap-pow。若这里仍按库里存的 'turnstile' 判，CSP 就不会
		// 放行 'wasm-unsafe-eval'，cap-widget 的 WebAssembly.compile() 会被
		// 浏览器直接拒绝，回退形同虚设 —— 用户卡在「请完成人机验证」且组件
		// 永远解不出题（实测报错：violates "script-src 'self' https://challenges.cloudflare.com"）。
		method := ""
		if row, err := settings.GetSettings(); err == nil {
			method = verifypolicy.EffectiveMethod(row)
		}

		cspCache.mu.Lock()
		if method != "" {
			cspCache.value = deriveCSP(method)
			cspCache.expires = time.Now().Add(cspCacheTTL)
		} else {
			// 读不到设置：有旧值就保持旧值（已知信息不该降级成兜底值），
			// 没旧值才用兜底值。两种都按更短的间隔重试，避免数据库挂着时
			// 每个请求都去撞一次。
			if cspCache.value == "" {
				cspCache.value = bootCSP
			}
			cspCache.expires = time.Now().Add(cspRetryTTL)
		}
		cspCache.refreshing = false
		cspCache.done = nil
		cspCache.mu.Unlock()
		close(done)
	}()

	if wait > 0 {
		select {
		case <-done:
		case <-time.After(wait):
		}
	}
}

// WarmCSP 在启动期把 CSP 预热一次，让首批请求就能拿到与实际验证方式一致的值。
// 有界等待：数据库再慢也不该拖住进程启动。
func WarmCSP() {
	refreshCSP(2 * time.Second)
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
