package images

import (
	"bytes"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"oneimg/backend/models"

	"github.com/gin-gonic/gin"
)

func uploadTestContext(t *testing.T, contentType string, body *bytes.Buffer) *gin.Context {
	t.Helper()
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/upload/images", body)
	c.Request.Header.Set("Content-Type", contentType)
	return c
}

// multipartUploadContext 构造一个带 n 个文件的 multipart 请求。
func multipartUploadContext(t *testing.T, field string, n int) *gin.Context {
	t.Helper()
	body := &bytes.Buffer{}
	w := multipart.NewWriter(body)
	for i := 0; i < n; i++ {
		part, err := w.CreateFormFile(field, "a.png")
		if err != nil {
			t.Fatalf("构造 multipart 失败：%v", err)
		}
		if _, err := part.Write([]byte("png-bytes")); err != nil {
			t.Fatalf("写入 multipart 失败：%v", err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatalf("关闭 multipart 失败：%v", err)
	}
	return uploadTestContext(t, w.FormDataContentType(), body)
}

// TestParseUploadFilesAcceptsDocumentedAndCanonicalFields 覆盖 issue #49：
// 接口文档里写的是 file / file[]，代码只认 images[]，照文档对接必然 400。
// 现在两种写法都能用，规范字段仍是 images[]。
func TestParseUploadFilesAcceptsDocumentedAndCanonicalFields(t *testing.T) {
	setting := models.Settings{MaxUploadFiles: 10}
	cases := []struct {
		field string
		count int
	}{
		{"images[]", 2},
		{"images", 1},
		{"file[]", 2},
		{"file", 1},
	}
	for _, tc := range cases {
		t.Run(tc.field, func(t *testing.T) {
			c := multipartUploadContext(t, tc.field, tc.count)
			files, err := parseConfiguredUploadFiles(c, setting)
			if err != nil {
				t.Fatalf("字段 %q 应被接受，实际报错：%v", tc.field, err)
			}
			if len(files) != tc.count {
				t.Fatalf("字段 %q 解析到 %d 个文件，期望 %d", tc.field, len(files), tc.count)
			}
		})
	}
}

// TestParseUploadFilesIsIdempotent 守住 issue #53 提到的重复解析：
// 同一请求解析两次（入口一次、分支一次）不应产生第二份文件或报错。
func TestParseUploadFilesIsIdempotent(t *testing.T) {
	setting := models.Settings{MaxUploadFiles: 10}
	c := multipartUploadContext(t, "images[]", 2)
	first, err := parseConfiguredUploadFiles(c, setting)
	if err != nil {
		t.Fatalf("首次解析失败：%v", err)
	}
	second, err := parseConfiguredUploadFiles(c, setting)
	if err != nil {
		t.Fatalf("重复解析不应失败：%v", err)
	}
	if len(first) != len(second) || len(second) != 2 {
		t.Fatalf("重复解析结果不一致：%d vs %d", len(first), len(second))
	}
}

func TestParseUploadFilesDistinguishesErrors(t *testing.T) {
	setting := models.Settings{MaxUploadFiles: 10}

	t.Run("Content-Type 不是 multipart", func(t *testing.T) {
		c := uploadTestContext(t, "application/json", bytes.NewBufferString(`{"a":1}`))
		_, err := parseConfiguredUploadFiles(c, setting)
		if err == nil {
			t.Fatal("非 multipart 请求必须报错")
		}
		if !strings.Contains(err.Error(), "multipart/form-data") {
			t.Fatalf("提示语应指出 Content-Type 问题，实际：%v", err)
		}
		if uploadParseErrorStatus(err) != http.StatusBadRequest {
			t.Fatalf("状态码应为 400，实际 %d", uploadParseErrorStatus(err))
		}
	})

	t.Run("字段名不对", func(t *testing.T) {
		c := multipartUploadContext(t, "wrongfield", 1)
		_, err := parseConfiguredUploadFiles(c, setting)
		if err == nil {
			t.Fatal("缺少文件字段必须报错")
		}
		if !strings.Contains(err.Error(), "images[]") {
			t.Fatalf("提示语应告诉调用方正确字段名，实际：%v", err)
		}
	})

	t.Run("超过数量上限", func(t *testing.T) {
		c := multipartUploadContext(t, "images[]", 3)
		_, err := parseConfiguredUploadFiles(c, models.Settings{MaxUploadFiles: 2})
		if err == nil || !strings.Contains(err.Error(), "最多只能上传2个文件") {
			t.Fatalf("应给出数量上限提示，实际：%v", err)
		}
	})
}

func TestUploadParseErrorStatus(t *testing.T) {
	if got := uploadParseErrorStatus(&http.MaxBytesError{Limit: 10}); got != http.StatusRequestEntityTooLarge {
		t.Fatalf("超限应为 413，实际 %d", got)
	}
	if got := uploadParseErrorStatus(ErrNoUploadField); got != http.StatusBadRequest {
		t.Fatalf("缺字段应为 400，实际 %d", got)
	}
}
