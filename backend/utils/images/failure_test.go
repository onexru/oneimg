package images

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
)

// 分类必须稳定：调用方按 code 分支，文案可以随便改。
func TestDescribeFailureClassifies(t *testing.T) {
	cases := []struct {
		name     string
		err      error
		maxBytes int64
		code     string
		contains string
	}{
		{"体积超限", ErrFileTooLarge, 0, FailureTooLarge, "32 MB"},
		{"体积超限(自定义上限)", ErrFileTooLarge, 5 << 20, FailureTooLarge, "上限 5 MB"},
		{"体积超限(非整 MB)", ErrFileTooLarge, 3<<20 + 512<<10, FailureTooLarge, "3.5 MB"},
		{"像素超限", ErrImageDimensions, 0, FailureDimensions, "1600 万像素"},
		{"格式不支持", ErrUnsupportedFormat, 0, FailureFormat, "仅支持 JPEG"},
		{"解码失败", ErrDecodeFailed, 0, FailureDecode, "解码失败"},
		{"超时", context.DeadlineExceeded, 0, FailureTimeout, "超时"},
		{"客户端断开", context.Canceled, 0, FailureTimeout, "超时"},
		// 上层包装过也要认得出来：ProcessImage 会在解码错误上套一层。
		{"被包装的像素超限", fmt.Errorf("decode image failed: %w", ErrImageDimensions), 0, FailureDimensions, "1600"},
		// 不认识的原因交回调用方决定（存储失败 / 通用失败）。
		{"未知错误", errors.New("boom"), 0, "", ""},
		{"nil", nil, 0, "", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			code, message := DescribeFailure(tc.err, tc.maxBytes)
			if code != tc.code {
				t.Fatalf("code = %q, want %q", code, tc.code)
			}
			if tc.contains != "" && !strings.Contains(message, tc.contains) {
				t.Fatalf("message = %q, want it to contain %q", message, tc.contains)
			}
			if tc.code == "" && message != "" {
				t.Fatalf("未分类时不应给文案，got %q", message)
			}
		})
	}
}

// 四类失败的文案必须彼此可区分 —— 这正是 issue #58 的核心诉求。
func TestDescribeFailureMessagesAreDistinct(t *testing.T) {
	errs := []error{ErrFileTooLarge, ErrImageDimensions, ErrUnsupportedFormat, ErrDecodeFailed}
	seen := make(map[string]string, len(errs))
	for _, err := range errs {
		code, message := DescribeFailure(err, 0)
		if message == "" {
			t.Fatalf("%v 没有文案", err)
		}
		if prev, ok := seen[message]; ok {
			t.Fatalf("%s 与 %s 的文案相同：%q", code, prev, message)
		}
		seen[message] = code
	}
}

// 上限文案与常量同源：改了常量，提示会跟着变。
func TestDimensionLimitTextFollowsConstants(t *testing.T) {
	text := DimensionLimitText()
	if want := fmt.Sprintf("%d 万像素", DirectMaxPixels/10000); !strings.Contains(text, want) {
		t.Fatalf("DimensionLimitText() = %q, want it to contain %q", text, want)
	}
	if want := fmt.Sprintf("%d px", DirectMaxDimension); !strings.Contains(text, want) {
		t.Fatalf("DimensionLimitText() = %q, want it to contain %q", text, want)
	}
}
