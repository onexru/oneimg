package images

import (
	"sync"
	"time"
)

// watermarkTranscodeCounter 是转码路径的按 IP 令牌桶。
//
// 为什么需要它：/uploads/* 由 NoRoute 直接交给 ImageProxy，不经过
// AuthLimiter（后者只覆盖 /api/login|register|auth/|verify/），
// 因此这条路径此前**完全没有限流**。而 ?watermark=true 会触发一次
// 「32MB 读取 + 16MP 解码 + 重编码」，是纯 CPU 重活，且免鉴权。
//
// 只对真正要转码的请求计费：
//   - 普通看图（不带 watermark 参数，或全局水印开启时浏览器直接取图）
//     仍然不受限，不影响正常浏览体验；
//   - 转码请求按每 IP 一个很小的速率放行，超出直接 429。
//
// 配合 watermark.acquireProcessingSlot 的排队超时，单 IP 无法再堆出
// 「客户端断开后服务器仍在跑」的长队列。
type watermarkTranscodeCounter struct {
	mu      sync.Mutex
	entries map[string]*transcodeEntry
	limit   int
	window  time.Duration
	now     func() time.Time
}

type transcodeEntry struct {
	count int
	reset time.Time
}

const (
	// 每个 IP 每 10 秒最多 6 次转码。正常用户几乎不会重复请求带参数的转码图
	// （同一张图第二次会被浏览器/CDN 缓存），所以 6 次足够人工验证样式；
	// 但足以把单 IP 的攻击速率压到 ~0.6/s，与「填满 2 个转码槽」相差三个数量级。
	watermarkTranscodeLimit  = 6
	watermarkTranscodeWindow = 10 * time.Second
	// 键数量上限，防止大量伪造源地址把 map 撑爆。
	watermarkTranscodeMaxKeys = 4096
)

var watermarkCounter = &watermarkTranscodeCounter{
	entries: make(map[string]*transcodeEntry),
	limit:   watermarkTranscodeLimit,
	window:  watermarkTranscodeWindow,
	now:     time.Now,
}

// allowWatermarkTranscode 判断该 IP 此刻是否可以执行一次转码。
func allowWatermarkTranscode(ip string) bool {
	return watermarkCounter.allow(ip)
}

func (w *watermarkTranscodeCounter) allow(ip string) bool {
	if ip == "" {
		ip = "unknown"
	}
	w.mu.Lock()
	defer w.mu.Unlock()

	now := w.now()
	entry, exists := w.entries[ip]
	if !exists || !now.Before(entry.reset) {
		// 新窗口。顺手清理过期键，避免只增不减。
		if !exists && len(w.entries) >= watermarkTranscodeMaxKeys {
			for key, e := range w.entries {
				if !now.Before(e.reset) {
					delete(w.entries, key)
				}
			}
			// 清完仍然满：拒绝，宁可误伤也不让内存无界增长。
			if len(w.entries) >= watermarkTranscodeMaxKeys {
				return false
			}
		}
		w.entries[ip] = &transcodeEntry{count: 1, reset: now.Add(w.window)}
		return true
	}

	if entry.count >= w.limit {
		return false
	}
	entry.count++
	return true
}
