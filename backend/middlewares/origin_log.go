package middlewares

import (
	"log"
	"strings"
	"sync"
)

// 被拒绝的跨来源请求只在进程内记一次。
//
// 换域名、反向代理改写 Host/Origin、本地换端口联调，这些事故在服务端如果不留痕，
// 用户只能从浏览器控制台看一句 CORS 报错去猜。但每个请求都记又会把日志刷满
// （前端一次加载就是十几个请求），所以按 Origin 去重；去重表设上限，
// 避免伪造 Origin 把内存刷爆——超限后不再记录，只是少了日志，不影响拒绝本身。
const maxLoggedRejectedOrigins = 32

var rejectedOrigins = struct {
	sync.Mutex
	seen map[string]struct{}
}{seen: make(map[string]struct{}, 8)}

// LogRejectedOrigin 在拒绝一个跨来源请求时调用：同一个 Origin 只输出一行，
// 让自建用户能从日志直接看出「该把这个地址加进 APP_URLS」。
func LogRejectedOrigin(origin string) {
	origin = strings.TrimSpace(origin)
	if origin == "" {
		return
	}

	rejectedOrigins.Lock()
	_, seen := rejectedOrigins.seen[origin]
	full := len(rejectedOrigins.seen) >= maxLoggedRejectedOrigins
	if !seen && !full {
		rejectedOrigins.seen[origin] = struct{}{}
	}
	rejectedOrigins.Unlock()

	if seen || full {
		return
	}
	log.Printf("⚠️ 已拒绝跨来源请求：Origin=%q 不在 APP_URL/APP_URLS 中；反向代理换了域名（或本地换端口联调）时，请把该地址加入 APP_URLS", origin)
}
