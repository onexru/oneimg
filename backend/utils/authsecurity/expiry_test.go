package authsecurity

import (
	"errors"
	"testing"
	"time"
)

// 有效期解析是「用户能不能把自己的 Token 设成 10 年」这件事的唯一入口，
// 边界（最短、最长、非法单位）必须逐条钉住，否则一次手滑就能造出
// 一张永不过期又没人察觉的凭据。
func TestParseTokenExpiry(t *testing.T) {
	cases := []struct {
		spec      string
		permanent bool
		duration  time.Duration
		wantSpec  string
	}{
		// 不传参数保持旧行为：30 天。
		{"", false, 30 * 24 * time.Hour, "30d"},
		{"   ", false, 30 * 24 * time.Hour, "30d"},
		{"permanent", true, 0, "permanent"},
		{"PERMANENT", true, 0, "permanent"},
		{"never", true, 0, "permanent"},
		{"forever", true, 0, "permanent"},
		{" 7d ", false, 7 * 24 * time.Hour, "7d"},
		{"30D", false, 30 * 24 * time.Hour, "30d"},
		{"12h", false, 12 * time.Hour, "12h"},
		{"90m", false, 90 * time.Minute, "90m"},
		{"5m", false, 5 * time.Minute, "5m"},
		{"2w", false, 14 * 24 * time.Hour, "14d"},
		// 纯数字按秒，方便脚本直接传整数。
		{"86400", false, 24 * time.Hour, "1d"},
	}

	for _, c := range cases {
		got, err := ParseTokenExpiry(c.spec)
		if err != nil {
			t.Fatalf("ParseTokenExpiry(%q) 返回错误: %v", c.spec, err)
		}
		if got.Permanent != c.permanent {
			t.Errorf("ParseTokenExpiry(%q).Permanent = %v, want %v", c.spec, got.Permanent, c.permanent)
		}
		if got.Duration != c.duration {
			t.Errorf("ParseTokenExpiry(%q).Duration = %v, want %v", c.spec, got.Duration, c.duration)
		}
		if got.Spec != c.wantSpec {
			t.Errorf("ParseTokenExpiry(%q).Spec = %q, want %q", c.spec, got.Spec, c.wantSpec)
		}
	}
}

func TestParseTokenExpiryRejects(t *testing.T) {
	for _, spec := range []string{"0", "-1", "10x", "abc", "d", "1.5h", "99999999999999999999"} {
		if _, err := ParseTokenExpiry(spec); !errors.Is(err, ErrTokenExpiry) {
			t.Errorf("ParseTokenExpiry(%q) 应当被拒绝，实际 error = %v", spec, err)
		}
	}
	// 低于下限（5 分钟）与高于上限（10 年）都要拒绝，否则「指定时长」就成了
	// 「随便填」：既能造出 1 秒就失效的凭据，也能绕到事实上的永久。
	for _, spec := range []string{"1s", "45s", "4m", "3651d", "100y"} {
		if _, err := ParseTokenExpiry(spec); !errors.Is(err, ErrTokenExpiry) {
			t.Errorf("ParseTokenExpiry(%q) 超出允许范围，应当被拒绝，实际 error = %v", spec, err)
		}
	}
}

func TestTokenExpiryExpiresAt(t *testing.T) {
	now := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	permanent, err := ParseTokenExpiry("permanent")
	if err != nil {
		t.Fatalf("解析 permanent 失败: %v", err)
	}
	// 永久有效必须落在哨兵值上：历史查询靠 expires_at 做索引过滤，
	// 换成零值或 NULL 会让这些查询直接漏掉这张凭据。
	if got := permanent.ExpiresAt(now); !got.Equal(PermanentExpiresAt) {
		t.Errorf("永久有效的 ExpiresAt = %v, 期望 %v", got, PermanentExpiresAt)
	}

	sevenDays, err := ParseTokenExpiry("7d")
	if err != nil {
		t.Fatalf("解析 7d 失败: %v", err)
	}
	if got, want := sevenDays.ExpiresAt(now), now.Add(7*24*time.Hour); !got.Equal(want) {
		t.Errorf("7d 的 ExpiresAt = %v, 期望 %v", got, want)
	}
}

func TestFormatTokenExpiry(t *testing.T) {
	cases := map[time.Duration]string{
		14 * 24 * time.Hour: "14d",
		36 * time.Hour:      "36h",
		90 * time.Minute:    "90m",
		45 * time.Second:    "45s",
	}
	for duration, want := range cases {
		if got := FormatTokenExpiry(duration); got != want {
			t.Errorf("FormatTokenExpiry(%v) = %q, want %q", duration, got, want)
		}
	}
}
