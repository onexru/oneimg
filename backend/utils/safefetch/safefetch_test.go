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

// stubLookup 替换包级 DNS 查询，返回按 network（"ip"/"ip4"）区分的固定结果。
func stubLookup(t *testing.T, byNetwork map[string][]net.IP) *[]string {
	t.Helper()
	var queried []string
	original := lookupIPFunc
	lookupIPFunc = func(ctx context.Context, network, host string) ([]net.IP, error) {
		queried = append(queried, network)
		if ips, ok := byNetwork[network]; ok {
			return ips, nil
		}
		return nil, nil
	}
	t.Cleanup(func() { lookupIPFunc = original })
	return &queried
}

// stubDial 替换实际拨号：failing 里的地址返回 error（模拟 ENETUNREACH），
// 其余地址返回一条 net.Pipe 连接。返回被尝试过的地址列表。
func stubDial(t *testing.T, failing map[string]error) *[]string {
	t.Helper()
	var tried []string
	original := dialFunc
	dialFunc = func(ctx context.Context, dialer *net.Dialer, network, addr string) (net.Conn, error) {
		tried = append(tried, addr)
		if err, ok := failing[addr]; ok {
			return nil, err
		}
		conn, peer := net.Pipe()
		t.Cleanup(func() { _ = peer.Close() })
		return conn, nil
	}
	t.Cleanup(func() { dialFunc = original })
	return &tried
}

// TestOrderIPv4First 覆盖排序：IPv4 必须排在 IPv6 之前。
func TestOrderIPv4First(t *testing.T) {
	ordered := orderIPv4First([]net.IP{
		mustParseIP(t, "2606:4700::1"),
		mustParseIP(t, "1.1.1.1"),
		mustParseIP(t, "2001:db8::1"),
		mustParseIP(t, "8.8.8.8"),
	})
	want := []string{"1.1.1.1", "8.8.8.8", "2606:4700::1", "2001:db8::1"}
	if len(ordered) != len(want) {
		t.Fatalf("排序后长度应为 %d，实际 %d", len(want), len(ordered))
	}
	for i := range want {
		if ordered[i].String() != want[i] {
			t.Fatalf("第 %d 位应为 %s，实际 %s", i, want[i], ordered[i])
		}
	}
}

// TestDialPublicStartsWithIPv4 是 issue #55 的核心回归：
// DNS 把 AAAA 排在前面时，拨号也必须先试 IPv4，而不是先撞 IPv6 的 ENETUNREACH。
func TestDialPublicStartsWithIPv4(t *testing.T) {
	stubLookup(t, map[string][]net.IP{
		"ip": {mustParseIP(t, "2606:4700::1"), mustParseIP(t, "1.1.1.1")},
	})
	tried := stubDial(t, map[string]error{})

	conn, err := dialPublic(context.Background(), "tcp", "example.com", "443")
	if err != nil {
		t.Fatalf("应先连上 IPv4，实际报错：%v", err)
	}
	conn.Close()
	if len(*tried) == 0 || (*tried)[0] != "1.1.1.1:443" {
		t.Fatalf("首个拨号目标应为 1.1.1.1:443，实际 %v", *tried)
	}
}

// TestDialPublicFallsBackToARecord 覆盖「只解析到 AAAA」的场景：
// 必须补查一次 A 记录，让纯 IPv4 出网的机器仍能取到图。
func TestDialPublicFallsBackToARecord(t *testing.T) {
	queried := stubLookup(t, map[string][]net.IP{
		"ip":  {mustParseIP(t, "2606:4700::1")},
		"ip4": {mustParseIP(t, "93.184.216.34")},
	})
	tried := stubDial(t, map[string]error{
		"[2606:4700::1]:443": errors.New("connect: network is unreachable"),
	})

	conn, err := dialPublic(context.Background(), "tcp", "example.com", "443")
	if err != nil {
		t.Fatalf("补查 A 记录后应连上，实际报错：%v", err)
	}
	conn.Close()

	sawIPv4Lookup := false
	for _, network := range *queried {
		if network == "ip4" {
			sawIPv4Lookup = true
		}
	}
	if !sawIPv4Lookup {
		t.Fatalf("只解析到 AAAA 时必须补查 A 记录，实际查询序列 %v", *queried)
	}
	if last := (*tried)[len(*tried)-1]; last != "93.184.216.34:443" {
		t.Fatalf("最后一次拨号应为补查到的 IPv4，实际 %v", *tried)
	}
}

// TestDialPublicAggregatesAllErrors 覆盖错误可读性：
// 每个地址的失败原因都要出现在错误里，而不是只剩最后一个。
func TestDialPublicAggregatesAllErrors(t *testing.T) {
	stubLookup(t, map[string][]net.IP{
		"ip": {mustParseIP(t, "1.1.1.1"), mustParseIP(t, "2606:4700::1")},
	})
	v6Err := errors.New("connect: network is unreachable")
	v4Err := errors.New("connect: connection refused")
	stubDial(t, map[string]error{
		"1.1.1.1:443":        v4Err,
		"[2606:4700::1]:443": v6Err,
	})

	_, err := dialPublic(context.Background(), "tcp", "example.com", "443")
	if err == nil {
		t.Fatal("全部地址失败时必须返回错误")
	}
	var dialErr *DialError
	if !errors.As(err, &dialErr) {
		t.Fatalf("应返回 *DialError，实际 %T：%v", err, err)
	}
	if len(dialErr.Attempts) != 2 {
		t.Fatalf("应记录 2 次尝试，实际 %d", len(dialErr.Attempts))
	}
	for _, want := range []string{"1.1.1.1:443", "connection refused", "2606:4700::1", "network is unreachable"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("错误信息应包含 %q，实际：%v", want, err)
		}
	}
	if !errors.Is(err, v4Err) {
		t.Fatalf("Unwrap 应指回首个尝试的错误，实际：%v", err)
	}
}

// TestDialPublicSkipsBlockedAddress 确认内网地址不会被拨号，即使 DNS 返回了它。
func TestDialPublicSkipsBlockedAddress(t *testing.T) {
	stubLookup(t, map[string][]net.IP{
		"ip": {mustParseIP(t, "127.0.0.1")},
	})
	tried := stubDial(t, map[string]error{})

	_, err := dialPublic(context.Background(), "tcp", "evil.example", "80")
	if err == nil {
		t.Fatal("内网地址必须失败")
	}
	if !strings.Contains(err.Error(), ErrBlockedHost.Error()) {
		t.Fatalf("错误应说明命中了禁止地址，实际：%v", err)
	}
	if len(*tried) != 0 {
		t.Fatalf("内网地址不应被拨号，实际尝试 %v", *tried)
	}
}

// TestDialPublicLiteralIPv4SkipsDNS 确认字面量 IP 不查 DNS。
func TestDialPublicLiteralIPv4SkipsDNS(t *testing.T) {
	queried := stubLookup(t, map[string][]net.IP{})
	tried := stubDial(t, map[string]error{})

	conn, err := dialPublic(context.Background(), "tcp", "1.1.1.1", "443")
	if err != nil {
		t.Fatalf("字面量 IP 应可直接拨号：%v", err)
	}
	conn.Close()
	if len(*queried) != 0 {
		t.Fatalf("字面量 IP 不应触发 DNS 查询，实际 %v", *queried)
	}
	if (*tried)[0] != "1.1.1.1:443" {
		t.Fatalf("拨号目标应为 1.1.1.1:443，实际 %v", *tried)
	}
}

