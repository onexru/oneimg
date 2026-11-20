package safefetch

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestTransportIgnoresEnvironmentProxy 是 issue #50 的回归测试。
//
// 一旦有人把 Proxy 改回 http.ProxyFromEnvironment，DialContext 拨号的就会变成
// 代理服务器本身，assertPublicHost 校验的是代理而非用户提交的目标，
// SSRF 防护（含 DNS rebinding 二次校验）会被整段绕过。
func TestTransportIgnoresEnvironmentProxy(t *testing.T) {
	t.Setenv("HTTP_PROXY", "http://127.0.0.1:3128")
	t.Setenv("HTTPS_PROXY", "http://127.0.0.1:3128")
	t.Setenv("ALL_PROXY", "http://127.0.0.1:3128")

	tr := newTransport()
	if tr.Proxy != nil {
		t.Fatal("safefetch 的 Transport 不允许使用代理：设置了 HTTP(S)_PROXY 时会绕过 SSRF 校验")
	}
	if tr.DialContext == nil {
		t.Fatal("DialContext 必须保留：解析期校验之外还需要连接期二次校验")
	}
}

func TestValidatePublicHTTPURL(t *testing.T) {
	blocked := []string{
		"http://127.0.0.1/x.png",
		"http://localhost/x.png",
		"http://foo.localhost/x.png",
		"http://169.254.169.254/latest/meta-data/",
		"http://10.0.0.8/x.png",
		"http://192.168.1.1/x.png",
		"http://172.16.0.1/x.png",
		"http://100.64.0.1/x.png",
		"http://[::1]/x.png",
		"http://0.0.0.0/x.png",
		"http://metadata.google.internal/x.png",
		"file:///etc/passwd",
		"gopher://example.com/x",
		"",
		"   ",
	}
	for _, raw := range blocked {
		if err := ValidatePublicHTTPURL(raw); err == nil {
			t.Errorf("期望被拒绝，实际通过了：%q", raw)
		}
	}

	// IP 字面量不会触发 DNS，适合做离线断言。
	if err := ValidatePublicHTTPURL("https://1.1.1.1/a.png"); err != nil {
		t.Errorf("公网 IP 不应被拒绝：%v", err)
	}
	if err := ValidatePublicHTTPURL("http://93.184.216.34/a.png"); err != nil {
		t.Errorf("公网 IP 不应被拒绝：%v", err)
	}
}

func TestIsBlockedIP(t *testing.T) {
	cases := map[string]bool{
		"127.0.0.1":       true,
		"10.1.2.3":        true,
		"192.168.0.1":     true,
		"172.20.1.1":      true,
		"169.254.169.254": true,
		"100.64.1.1":      true,
		"0.0.0.0":         true,
		"8.8.8.8":         false,
		"1.1.1.1":         false,
	}
	for raw, want := range cases {
		ip := mustParseIP(t, raw)
		if got := isBlockedIP(ip); got != want {
			t.Errorf("isBlockedIP(%s) = %v，期望 %v", raw, got, want)
		}
	}
	if !isBlockedIP(nil) {
		t.Error("nil IP 必须视为禁止")
	}
}

// TestDialControlRejectsPrivateAddress 覆盖连接期二次校验：
// 即使解析期通过了，真正拨号前仍要按 IP 再判一次（DNS rebinding）。
func TestDialControlRejectsPrivateAddress(t *testing.T) {
	if err := DialControl("tcp", "127.0.0.1:80", nil); !errors.Is(err, ErrBlockedHost) {
		t.Errorf("回环地址应被拒绝，实际：%v", err)
	}
	if err := DialControl("tcp", "169.254.169.254:80", nil); !errors.Is(err, ErrBlockedHost) {
		t.Errorf("metadata 地址应被拒绝，实际：%v", err)
	}
	if err := DialControl("tcp", "1.1.1.1:443", nil); err != nil {
		t.Errorf("公网地址不应被拒绝：%v", err)
	}
}

// TestGetNeverUsesProxy 用本地「假代理」验证行为层：把 HTTP_PROXY 指向一个
// 记录请求的 httptest 服务，再请求一个内网地址，断言代理没有收到任何请求。
func TestGetNeverUsesProxy(t *testing.T) {
	var proxyHits int
	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		proxyHits++
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer proxy.Close()

	t.Setenv("HTTP_PROXY", proxy.URL)
	t.Setenv("HTTPS_PROXY", proxy.URL)

	_, err := Get(context.Background(), "http://169.254.169.254/latest/meta-data/", 0)
	if !errors.Is(err, ErrBlockedHost) {
		t.Fatalf("内网地址应被拒绝，实际：%v", err)
	}
	if proxyHits != 0 {
		t.Fatalf("代理不应收到请求（收到 %d 次），否则 SSRF 校验被绕过", proxyHits)
	}
}

func TestReadLimited(t *testing.T) {
	data, err := ReadLimited(strings.NewReader("hello"), 5)
	if err != nil || string(data) != "hello" {
		t.Fatalf("恰好等于上限时应成功：%v %q", err, data)
	}
	if _, err := ReadLimited(strings.NewReader("hello!"), 5); !errors.Is(err, ErrBodyTooLarge) {
		t.Fatalf("超过上限必须显式报错，实际：%v", err)
	}
}

func mustParseIP(t *testing.T, raw string) net.IP {
	t.Helper()
	parsed := net.ParseIP(raw)
	if parsed == nil {
		t.Fatalf("无法解析 IP：%s", raw)
	}
	return parsed
}
