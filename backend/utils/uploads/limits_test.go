package uploads

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func newTestContext(t *testing.T) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/upload/images", nil)
	return c, rec
}

// TestAcquireSlotRejectsWhenFullAndSetsRetryAfter 是 issue #51 的回归测试：
// 槽位占满时必须回 429，并且带上 Retry-After，客户端才知道该退避多久。
func TestAcquireSlotRejectsWhenFullAndSetsRetryAfter(t *testing.T) {
	pool := make(chan struct{}, 1)
	c1, _ := newTestContext(t)
	release, ok := AcquireSlot(c1, pool, 0, 0, "忙")
	if !ok {
		t.Fatal("第一个请求应能拿到槽位")
	}

	c2, rec := newTestContext(t)
	release2, ok := AcquireSlot(c2, pool, 0, 0, "服务器正在处理其他上传")
	if ok {
		release2()
		t.Fatal("槽位已满时必须拒绝")
	}
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("状态码应为 429，实际 %d", rec.Code)
	}
	if got := rec.Header().Get("Retry-After"); got == "" {
		t.Fatal("429 响应必须带 Retry-After")
	} else if got != "2" {
		t.Fatalf("Retry-After 应为 2 秒，实际 %q", got)
	}
	var body struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("响应不是合法 JSON：%v (%s)", err, rec.Body.String())
	}
	if body.Code != 429 || !strings.Contains(body.Message, "服务器正在处理其他上传") {
		t.Fatalf("提示语应说明这是服务器全局繁忙，实际：%+v", body)
	}

	release()
	// 归还后应能重新拿到。
	c3, _ := newTestContext(t)
	release3, ok := AcquireSlot(c3, pool, 0, 0, "忙")
	if !ok {
		t.Fatal("槽位归还后应能重新占用")
	}
	release3()
}

func TestAcquireSlotWaitsForRelease(t *testing.T) {
	pool := make(chan struct{}, 1)
	c1, _ := newTestContext(t)
	release, ok := AcquireSlot(c1, pool, 0, 0, "忙")
	if !ok {
		t.Fatal("第一个请求应能拿到槽位")
	}
	go func() {
		time.Sleep(20 * time.Millisecond)
		release()
	}()

	c2, _ := newTestContext(t)
	start := time.Now()
	release2, ok := AcquireSlot(c2, pool, 0, time.Second, "忙")
	if !ok {
		t.Fatal("排队等待后应能拿到槽位")
	}
	defer release2()
	if time.Since(start) < 10*time.Millisecond {
		t.Fatal("应当在排队后才成功，而不是立刻拿到")
	}
}

// TestSlotForcedRelease 覆盖兜底路径：请求异常长时，定时器应把槽位强行归还，
// 而不是让一个请求永久占住限流额度。
func TestSlotForcedRelease(t *testing.T) {
	pool := make(chan struct{}, 1)
	c1, _ := newTestContext(t)
	if _, ok := AcquireSlot(c1, pool, 30*time.Millisecond, 0, "忙"); !ok {
		t.Fatal("第一个请求应能拿到槽位")
	}
	time.Sleep(80 * time.Millisecond)
	if got := len(pool); got != 0 {
		t.Fatalf("超时后槽位应被强制归还，实际仍占用 %d 个", got)
	}
}

func TestReleaseIsIdempotent(t *testing.T) {
	pool := make(chan struct{}, 1)
	c, _ := newTestContext(t)
	release, ok := AcquireSlot(c, pool, time.Minute, 0, "忙")
	if !ok {
		t.Fatal("应能拿到槽位")
	}
	release()
	release() // 二次调用必须是空操作，否则会把别的请求的槽位吞掉
	if got := len(pool); got != 0 {
		t.Fatalf("重复 release 破坏了槽位计数：len=%d", got)
	}
	c2, _ := newTestContext(t)
	release2, ok := AcquireSlot(c2, pool, 0, 0, "忙")
	if !ok {
		t.Fatal("槽位应可用")
	}
	release2()
}

func TestRequestSlotCountFromEnv(t *testing.T) {
	cases := []struct {
		env  string
		want int
	}{
		{"", DefaultRequestSlots},
		{"1", 1},
		{"6", 6},
		{"0", DefaultRequestSlots},
		{"-3", DefaultRequestSlots},
		{"abc", DefaultRequestSlots},
		{"999", MaxRequestSlots},
	}
	for _, tc := range cases {
		t.Run("UPLOAD_SLOTS="+tc.env, func(t *testing.T) {
			t.Setenv("UPLOAD_SLOTS", tc.env)
			if got := RequestSlotCount(); got != tc.want {
				t.Fatalf("RequestSlotCount() = %d，期望 %d", got, tc.want)
			}
		})
	}
}

func TestWrapBodyCapsRequestSize(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/upload/images", strings.NewReader("0123456789"))

	WrapBody(c, 4)
	_, err := io.ReadAll(c.Request.Body)
	if err == nil {
		t.Fatal("超过上限的请求体必须报错，而不是被静默截断")
	}
	var maxErr *http.MaxBytesError
	if !errors.As(err, &maxErr) {
		t.Fatalf("应返回 *http.MaxBytesError，实际 %T: %v", err, err)
	}
}
