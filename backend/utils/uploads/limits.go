package uploads

// 全局上传槽与「慢连接挂死」防护。
//
// 从 controllers/images 下沉到这里，因为「直传」(controllers/storage) 也要用它：
// 留在 images 会让 storage 反向依赖 images，而 images 又依赖 storage
// （查询副本状态），构成循环。这层逻辑只依赖 gin 与 net/http，
// 放在 utils 层两边都能用。

import (
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gin-gonic/gin"
)

const (
	// DefaultRequestSlots 是「同时处理的上传请求」默认上限。
	//
	// 不能随手调大：每个槽都可能完整持有一份解码后的位图，
	// 峰值内存大致是「槽位数 × 单请求峰值（约 100~200MB）」。
	// 需要放宽时用环境变量 UPLOAD_SLOTS 显式指定，并按机器内存核算。
	DefaultRequestSlots = 2
	// MaxRequestSlots 是 UPLOAD_SLOTS 的上限，防止一次配置失误把机器打爆。
	MaxRequestSlots = 16
	// RetryAfterSeconds 是 429 响应里 Retry-After 的取值（秒），
	// 与 ImageServe 转码繁忙时的 503 保持一致。
	RetryAfterSeconds = 2
)

// SlotHoldLimit bounds how long a single request may hold a global
// upload slot. It is a fallback for transports where the stall-abort body
// reader cannot enforce an idle deadline: a slow-loris style multipart body
// would otherwise pin both slots indefinitely and starve every other upload.
//
// 变量而非常量，便于回归测试缩短它。
var SlotHoldLimit = 15 * time.Minute

// SlotWaitLimit 是「槽位已满时先排队等一会儿」的上限。
//
// 批量上传脚本很容易瞬间发出 4 个请求，而槽位只有 N 个：立刻 429 会让
// 使用者以为功能坏了。短暂排队能把这类抖动吸收掉，同时用有限时长
// （而不是无界队列）保证慢客户端堆积的连接数有上界。
// 设为 0 表示不排队、立即拒绝。
var SlotWaitLimit = 5 * time.Second

var (
	requestSlotsOnce sync.Once
	requestSlots     chan struct{}

	// busyRequestMessage 是上传槽全部被占时返回的提示语。
	// 按当前上限生成，让使用者知道这是「全局并发上限」，
	// 而不是「他自己这张图在排队」。
	busyRequestMessage = "服务器正在处理其他上传，请稍后重试"
)

// RequestSlotCount 解析当前生效的上传并发槽位数。
// UPLOAD_SLOTS 非法时退回默认值而不是拒绝启动：限流参数配错不该导致服务不可用。
func RequestSlotCount() int {
	raw := strings.TrimSpace(os.Getenv("UPLOAD_SLOTS"))
	if raw == "" {
		return DefaultRequestSlots
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n < 1 {
		log.Printf("UPLOAD_SLOTS=%q 无效，回退为默认值 %d", raw, DefaultRequestSlots)
		return DefaultRequestSlots
	}
	if n > MaxRequestSlots {
		log.Printf("UPLOAD_SLOTS=%d 超过上限 %d，按上限生效", n, MaxRequestSlots)
		return MaxRequestSlots
	}
	return n
}

func requestSlotPool() chan struct{} {
	requestSlotsOnce.Do(func() {
		size := RequestSlotCount()
		requestSlots = make(chan struct{}, size)
		busyRequestMessage = "服务器正在处理其他上传（并发上限 " + strconv.Itoa(size) + "），请稍后重试"
		log.Printf("上传并发槽位：%d（UPLOAD_SLOTS 可调；峰值内存随槽位数线性增长）", size)
	})
	return requestSlots
}

// RequestSlots 返回全局上传槽的只读视图，供测试与诊断使用。
func RequestSlots() <-chan struct{} {
	return requestSlotPool()
}

// SlotCount 返回当前生效的并发上限。
func SlotCount() int {
	return cap(requestSlotPool())
}

// AcquireRequest 占用一个全局上传槽。
//
// 失败时已经写好 429 响应（含 Retry-After），调用方直接 return 即可。
func AcquireRequest(c *gin.Context) (func(), bool) {
	return AcquireSlot(c, requestSlotPool(), SlotHoldLimit, SlotWaitLimit, busyRequestMessage)
}

// AcquireSlot 在给定池上占用一个槽位：先尝试立即获取，池满则在 waitLimit
// 内排队（waitLimit<=0 表示立刻拒绝）。
//
// 排队可被客户端断开打断（c.Request.Context()），因此不会出现
// 「客户端早走了、请求还挂在队列里」的堆积。
func AcquireSlot(c *gin.Context, pool chan struct{}, holdLimit, waitLimit time.Duration, busyMessage string) (func(), bool) {
	if !waitForSlot(c, pool, waitLimit) {
		c.Header("Retry-After", strconv.Itoa(RetryAfterSeconds))
		c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
			"code":    http.StatusTooManyRequests,
			"message": busyMessage,
			"data":    nil,
		})
		return nil, false
	}

	lease := &slotLease{pool: pool, path: c.Request.URL.Path, hold: holdLimit}
	if holdLimit > 0 {
		lease.timer.Store(time.AfterFunc(holdLimit, func() { lease.release(true) }))
	}
	return func() { lease.release(false) }, true
}

// waitForSlot 先非阻塞尝试，再在 waitLimit 内排队等待。
func waitForSlot(c *gin.Context, pool chan struct{}, waitLimit time.Duration) bool {
	select {
	case pool <- struct{}{}:
		return true
	default:
	}
	if waitLimit <= 0 {
		return false
	}
	timer := time.NewTimer(waitLimit)
	defer timer.Stop()
	select {
	case pool <- struct{}{}:
		return true
	case <-timer.C:
		return false
	case <-c.Request.Context().Done():
		return false
	}
}

// slotLease 把「归还槽位」与「兜底定时器」绑在一起：
// 正常结束要停掉定时器，否则每个请求都会留一个 15 分钟的定时器；
// 由定时器强制归还时则要留一条 warn 日志，便于发现异常长请求。
type slotLease struct {
	pool  chan struct{}
	once  sync.Once
	timer atomic.Pointer[time.Timer]
	path  string
	hold  time.Duration
}

func (l *slotLease) release(forced bool) {
	l.once.Do(func() {
		if t := l.timer.Load(); t != nil {
			t.Stop()
		}
		if forced {
			log.Printf("[upload-slot] 上传请求持有槽位超过 %s，已强制归还：path=%s（该请求可能仍在运行，实际并发会短暂超过上限）", l.hold, l.path)
		}
		<-l.pool
	})
}

// StallIdle is the maximum gap between two body reads before the
// request is considered stalled and aborted.
const StallIdle = 60 * time.Second

// stallAbortBody aborts the underlying connection when an upload body stops
// delivering data for the idle period. The server deliberately has no global
// ReadTimeout because large uploads are legitimate; this per-request guard
// closes the slow-loris gap without imposing a total transfer deadline.
// It uses http.ResponseController.SetReadDeadline (supported by gin's
// ResponseWriter via Unwrap); transports without deadline support simply
// keep the previous behavior, and SlotHoldLimit still applies.
type StallAbortBody struct {
	rc    *http.ResponseController
	body  io.ReadCloser
	idle  time.Duration
	timer *time.Timer
}

func (b *StallAbortBody) Read(p []byte) (int, error) {
	if b.timer == nil {
		b.timer = time.AfterFunc(b.idle, func() {
			// A deadline in the past unblocks the pending body read with a
			// timeout error, which fails multipart parsing and releases the
			// upload slot.
			_ = b.rc.SetReadDeadline(time.Now().Add(-time.Second))
		})
	} else {
		b.timer.Reset(b.idle)
	}
	return b.body.Read(p)
}

func (b *StallAbortBody) Close() error {
	if b.timer != nil {
		b.timer.Stop()
	}
	return b.body.Close()
}

// WrapBody applies the response-aware size cap and the stall abort to
// an upload request body. Call it before any multipart parsing.
func WrapBody(c *gin.Context, maxBytes int64) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxBytes)
	c.Request.Body = &StallAbortBody{
		rc:   http.NewResponseController(c.Writer),
		body: c.Request.Body,
		idle: StallIdle,
	}
}
