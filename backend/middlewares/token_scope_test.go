package middlewares

import (
	"net/http"
	"testing"
)

// tokenRouteScope 决定 API Token 能碰到哪些路由。漏掉一条就会在检查 scopes
// 之前直接 403（#60 里删除接口就是这样被打回的）。
func TestTokenRouteScope(t *testing.T) {
	cases := []struct {
		method string
		path   string
		want   string
	}{
		// 删除：单张与批量都必须映射到 image:delete。
		{http.MethodDelete, "/api/images/409", "image:delete"},
		{http.MethodDelete, "/api/images/1", "image:delete"},
		{http.MethodPost, "/api/images/batch-delete", "image:delete"},

		// 标签删除归 image:tag:delete 管，不能靠 image:delete 顺手放行。
		{http.MethodDelete, "/api/images/tag", ""},
		{http.MethodDelete, "/api/images/tags", ""},
		{http.MethodDelete, "/api/images/409/tags", ""},

		// 原有 scope 不能被改动影响。
		{http.MethodGet, "/api/images/409", "image:read"},
		{http.MethodGet, "/api/images", "image:read"},
		{http.MethodPost, "/api/upload/images", "upload:write"},
		{http.MethodPost, "/api/images/url", "upload:write"},

		// 未开放的操作仍然拦在中间件。
		{http.MethodPut, "/api/images/409/access-source", ""},
		{http.MethodGet, "/api/settings/seo", ""},
	}

	for _, c := range cases {
		if got := tokenRouteScope(c.method, c.path); got != c.want {
			t.Errorf("tokenRouteScope(%s, %s) = %q, want %q", c.method, c.path, got, c.want)
		}
	}
}

func TestImageIDPath(t *testing.T) {
	ok := []string{"/api/images/1", "/api/images/409", "/api/images/0"}
	bad := []string{"/api/images/", "/api/images", "/api/images/abc", "/api/images/4a",
		"/api/images/409/tags", "/api/images/tag", "/uploads/409", "/api/images/-1"}

	for _, p := range ok {
		if !imageIDPath(p) {
			t.Errorf("imageIDPath(%q) = false, want true", p)
		}
	}
	for _, p := range bad {
		if imageIDPath(p) {
			t.Errorf("imageIDPath(%q) = true, want false", p)
		}
	}
}
