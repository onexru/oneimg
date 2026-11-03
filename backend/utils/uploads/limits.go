package uploads

// 全局上传槽与「慢连接挂死」防护。
//
// 从 controllers/images 下沉到这里，因为「直传」(controllers/storage) 也要用它：
// 留在 images 会让 storage 反向依赖 images，而 images 又依赖 storage
// （查询副本状态），构成循环。这层逻辑只依赖 gin 与 net/http，
// 放在 utils 层两边都能用。

import (
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

var RequestSlots = make(chan struct{}, 2)

// SlotHoldLimit bounds how long a single request may hold a global
// upload slot. It is a fallback for transports where the stall-abort body
// reader cannot enforce an idle deadline: a slow-loris style multipart body
// would otherwise pin both slots indefinitely and starve every other upload.
const SlotHoldLimit = 15 * time.Minute

func AcquireRequest(c *gin.Context) (func(), bool) {
	select {
	case RequestSlots <- struct{}{}:
		var once sync.Once
		release := func() { once.Do(func() { <-RequestSlots }) }
		time.AfterFunc(SlotHoldLimit, release)
		return release, true
	default:
		c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{"code": 429, "message": "上传处理中，请稍后重试"})
		return nil, false
	}
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

// wrapUploadBody applies the response-aware size cap and the stall abort to
// an upload request body. Call it before any multipart parsing.
func WrapBody(c *gin.Context, maxBytes int64) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxBytes)
	c.Request.Body = &StallAbortBody{
		rc:   http.NewResponseController(c.Writer),
		body: c.Request.Body,
		idle: StallIdle,
	}
}

