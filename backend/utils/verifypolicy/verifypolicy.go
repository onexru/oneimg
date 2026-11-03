// Package verifypolicy 集中存放「实际生效的人机验证方式」这一判定。
//
// 为什么要单独成包：这个判定被两处需要 ——
//   - controllers 用它决定登录/注册时校验哪种 token；
//   - middlewares 用它决定 CSP 是否放行 'wasm-unsafe-eval'（cap-pow 需要）。
//
// 但它要读运行时状态（服务端是否观测到 Turnstile 密钥无效），而该状态当前
// 写在 controllers 里。middlewares 与 controllers 互相引用会构成循环依赖
// （controllers 需要 middlewares 做权限中间件），所以把状态与判定一起下沉到这里，
// 两侧都只依赖本包。
//
// 历史坑：CSP 侧曾直接用 models.EffectiveVerifyMethod（不含回退），导致
// 「配了 Turnstile 但密钥不全 → 实际回退 cap-pow」时，CSP 不放行 wasm，
// cap-widget 的 WebAssembly.compile() 被浏览器拒绝，回退在界面上完全失效。
package verifypolicy

import (
	"sync"
	"time"

	"oneimg/backend/models"
)

const turnstileBrokenTTL = 10 * time.Minute

// turnstileBroken 记录服务端已观测到 Turnstile 密钥配置错误（invalid/missing-input-secret）。
// 置位后在 TTL 内将 Turnstile 视为不可用并回退到 cap-pow——回退完全由服务端判定，
// 不信任任何客户端标记，攻击者无法借此绕过 Turnstile。
var turnstileBroken struct {
	sync.RWMutex
	until time.Time
}

// MarkTurnstileBroken 标记 Turnstile 密钥不可用（TTL 内生效）。
func MarkTurnstileBroken() {
	turnstileBroken.Lock()
	turnstileBroken.until = time.Now().Add(turnstileBrokenTTL)
	turnstileBroken.Unlock()
}

// TurnstileBrokenNow 返回当前是否处于「Turnstile 被判定不可用」的窗口内。
func TurnstileBrokenNow() bool {
	turnstileBroken.RLock()
	defer turnstileBroken.RUnlock()
	return time.Now().Before(turnstileBroken.until)
}

// TurnstileUsable 判定 Turnstile 是否实际可用：
// 公钥格式合法、密钥非空，且服务端未观测到配置错误。
func TurnstileUsable(s models.Settings) bool {
	return models.TurnstileConfigComplete(s) && !TurnstileBrokenNow()
}

// EffectiveMethod 返回最终实际生效的人机验证方式。
//
// 相对 models.EffectiveVerifyMethod 多一层回退：管理员选了 Turnstile
// 但配置不完整（密钥缺失 / 公钥格式明显错误）或服务端已观测到密钥无效时，
// 实际生效的是 cap-pow。调用方必须用本函数，而不是直接读 models 里的那个，
// 否则会出现「校验走 cap-pow、CSP 按 Turnstile 收紧」这类不一致。
func EffectiveMethod(s models.Settings) string {
	if models.EffectiveVerifyMethod(s) == models.VerifyMethodTurnstile && !TurnstileUsable(s) {
		return models.VerifyMethodCappow
	}
	return models.EffectiveVerifyMethod(s)
}
