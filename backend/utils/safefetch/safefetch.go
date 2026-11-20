package safefetch

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"syscall"
	"time"
)

var (
	ErrInvalidURL       = errors.New("URL 无效")
	ErrBlockedHost      = errors.New("禁止访问内网或保留地址")
	ErrBlockedScheme    = errors.New("仅允许 http/https")
	ErrTooManyRedirects = errors.New("重定向次数过多")
	ErrBodyTooLarge     = errors.New("响应体超过大小限制")
)

// ValidatePublicHTTPURL 校验 URL 是否可安全对外请求（防 SSRF）。
func ValidatePublicHTTPURL(raw string) error {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ErrInvalidURL
	}
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return ErrInvalidURL
	}
	scheme := strings.ToLower(u.Scheme)
	if scheme != "http" && scheme != "https" {
		return ErrBlockedScheme
	}
	host := u.Hostname()
	if host == "" {
		return ErrInvalidURL
	}
	return assertPublicHost(host)
}

func assertPublicHost(host string) error {
	host = strings.TrimSpace(strings.ToLower(host))
	if host == "" {
		return ErrInvalidURL
	}
	if host == "localhost" || strings.HasSuffix(host, ".localhost") || host == "metadata.google.internal" {
		return ErrBlockedHost
	}

	if ip := net.ParseIP(host); ip != nil {
		if isBlockedIP(ip) {
			return ErrBlockedHost
		}
		return nil
	}

	ips, err := net.LookupIP(host)
	if err != nil {
		return fmt.Errorf("DNS 解析失败: %w", err)
	}
	if len(ips) == 0 {
		return ErrBlockedHost
	}
	for _, ip := range ips {
		if isBlockedIP(ip) {
			return ErrBlockedHost
		}
	}
	return nil
}

// AssertPublicHost 校验主机名（域名或 IP 字面量）解析后不落在内网/保留地址。
//
// 供「不经过 HTTP 客户端」的出站连接复用（S3/FTP/WebDAV 等存储驱动：
// 它们自己 dial TCP，套不上 safefetch 的 Transport）。
// 只做解析期校验；连接期仍需二次校验以防 DNS rebinding —— 见 DialControl。
func AssertPublicHost(host string) error {
	return assertPublicHost(host)
}

// AssertPublicEndpoint 校验 host[:port] 形式的端点。
// 端口本身不做限制（存储服务常跑在非标端口），只校验主机部分。
func AssertPublicEndpoint(hostport string) error {
	host, _, err := net.SplitHostPort(hostport)
	if err != nil {
		// 没有端口时 SplitHostPort 会报错，此时整体就是主机名
		host = hostport
	}
	host = strings.Trim(host, "[]")
	return assertPublicHost(host)
}

// DialControl 是给 net.Dialer.Control 用的连接期校验。
//
// 只在解析期校验一次是不够的：攻击者可以让域名第一次解析到公网、
// 第二次解析到 127.0.0.1（DNS rebinding）。Control 在真正建立连接前
// 拿到的是**即将拨号的 IP**，此处再校验一次即可封死这条路径。
//
// 用法：
//
//	d := &net.Dialer{Control: safefetch.DialControl}
func DialControl(network, address string, conn syscall.RawConn) error {
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		host = address
	}
	ip := net.ParseIP(strings.Trim(host, "[]"))
	if ip == nil {
		return ErrBlockedHost
	}
	if isBlockedIP(ip) {
		return ErrBlockedHost
	}
	return nil
}

func isBlockedIP(ip net.IP) bool {
	if ip == nil {
		return true
	}
	if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() ||
		ip.IsMulticast() || ip.IsUnspecified() || ip.IsInterfaceLocalMulticast() {
		return true
	}
	// 常见云 metadata / CGNAT / 文档网段
	if ip4 := ip.To4(); ip4 != nil {
		if ip4[0] == 169 && ip4[1] == 254 {
			return true
		}
		if ip4[0] == 100 && ip4[1] >= 64 && ip4[1] <= 127 { // CGNAT 100.64/10
			return true
		}
		if ip4[0] == 0 {
			return true
		}
	}
	return false
}

// Get 使用安全策略下载远端资源。
func Get(ctx context.Context, rawURL string, timeout time.Duration) (*http.Response, error) {
	return get(ctx, rawURL, timeout, true)
}

// GetNoRedirect never forwards credential-bearing URLs to a redirected host.
func GetNoRedirect(ctx context.Context, rawURL string, timeout time.Duration) (*http.Response, error) {
	return get(ctx, rawURL, timeout, false)
}

// newTransport 构造「取远端图片」专用的 Transport。
//
// Proxy 固定为 nil，刻意**不**读取 HTTP_PROXY / HTTPS_PROXY：
// 走代理时 Transport 拨号的目标是代理服务器，而不是用户提交的 URL，于是
// DialContext 校验的是代理地址（通常公网，必然通过），真正的目标由代理去
// 解析 —— 相当于把 DNS 解析与 IP 校验整段让给了外部进程，DNS rebinding
// 防护也随之失效。容器化部署为出网配 HTTP_PROXY 相当常见，属于
// 「配置一并打开就静默失效」的形态，因此这里选择 fail-closed：
// 宁可取不到，也不让 SSRF 校验被绕过。
//
// 抽成独立函数是为了让回归测试能直接断言 Proxy 为 nil，而不必真实出网。
func newTransport() *http.Transport {
	return &http.Transport{
		Proxy: nil,
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			host, port, err := net.SplitHostPort(addr)
			if err != nil {
				return nil, err
			}
			if err := assertPublicHost(host); err != nil {
				return nil, err
			}
			return dialPublic(ctx, network, host, port)
		},
	}
}

// dialTimeout 是单个地址的拨号超时。
const dialTimeout = 10 * time.Second

// maxDialErrorAddresses 限制错误信息里列出的地址数，避免几十个 A/AAAA 记录把日志刷爆。
const maxDialErrorAddresses = 4

// lookupIPFunc 是 DNS 查询的可替换入口：默认走系统解析器（并响应 ctx 取消），
// 测试里替换成固定结果，才能断言「只解析到 AAAA 时补查 A 记录」这条路径。
var lookupIPFunc = func(ctx context.Context, network, host string) ([]net.IP, error) {
	return net.DefaultResolver.LookupIP(ctx, network, host)
}

// dialFunc 同理：默认用带 Control 校验的 Dialer 真正拨号，测试可替换。
var dialFunc = func(ctx context.Context, dialer *net.Dialer, network, addr string) (net.Conn, error) {
	return dialer.DialContext(ctx, network, addr)
}

type dialAttempt struct {
	addr string
	err  error
}

// DialError 汇总一次拨号的**全部**失败地址。
//
// 原来只把最后一个错误返回：在只有 IPv4 出网的机器上，DNS 常把 AAAA 排在前面，
// 于是用户永远只看到 IPv6 的 ENETUNREACH，IPv4 到底试没试、为什么失败全都不
// 可见，排查方向被直接带偏。这里把每个地址的原因都带上（有条数上限）。
type DialError struct {
	Host     string
	Attempts []dialAttempt
}

func (e *DialError) Error() string {
	parts := make([]string, 0, maxDialErrorAddresses+1)
	for i, attempt := range e.Attempts {
		if i == maxDialErrorAddresses {
			parts = append(parts, fmt.Sprintf("另有 %d 个地址同样失败", len(e.Attempts)-maxDialErrorAddresses))
			break
		}
		parts = append(parts, fmt.Sprintf("%s: %v", attempt.addr, attempt.err))
	}
	return fmt.Sprintf("连接 %s 失败，尝试 %d 个地址均失败：%s", e.Host, len(e.Attempts), strings.Join(parts, "；"))
}

// Unwrap 指向第一个尝试的错误，让 errors.Is(err, syscall.ENETUNREACH) 这类判断仍可用。
func (e *DialError) Unwrap() error {
	if len(e.Attempts) == 0 {
		return nil
	}
	return e.Attempts[0].err
}

// dialPublic 解析 host 后逐个地址拨号：IPv4 优先；全部失败且这一轮没试过 IPv4 时，
// 再显式补查一次 A 记录；最后把每个地址的错误聚合成 DialError。
func dialPublic(ctx context.Context, network, host, port string) (net.Conn, error) {
	ips, err := resolveDialIPs(ctx, host)
	if err != nil {
		return nil, err
	}

	attempts := make([]dialAttempt, 0, len(ips)+2)
	conn, triedIPv4 := dialAddresses(ctx, network, port, ips, &attempts)
	if conn != nil {
		return conn, nil
	}

	// 解析结果里可能只有 AAAA：A 查询失败，或被 CDN 按线路过滤掉。这时上面的
	// 循环里根本没有 IPv4 可试，而域名本身的 A 记录完全可能是通的 —— 纯 IPv4
	// 出网的机器（国内 VPS / 容器很常见）会因此直接失败。只有「一个 IPv4 都没
	// 试过」才补查，避免对着同一批地址再来一遍。
	if !triedIPv4 && ctx.Err() == nil {
		if v4s, v4Err := lookupIPFunc(ctx, "ip4", host); v4Err == nil && len(v4s) > 0 {
			conn, _ := dialAddresses(ctx, network, port, v4s, &attempts)
			if conn != nil {
				return conn, nil
			}
		}
	}

	if len(attempts) == 0 {
		return nil, ErrBlockedHost
	}
	return nil, &DialError{Host: host, Attempts: attempts}
}

func resolveDialIPs(ctx context.Context, host string) ([]net.IP, error) {
	// DialContext 拿到的 host 可能就是字面量 IP（例如重定向直接指向 IP）。
	if ip := net.ParseIP(host); ip != nil {
		return []net.IP{ip}, nil
	}
	ips, err := lookupIPFunc(ctx, "ip", host)
	if err != nil {
		return nil, fmt.Errorf("DNS 解析失败: %w", err)
	}
	return ips, nil
}

// dialAddresses 按 IPv4 优先的顺序逐个拨号，成功的连接立即返回。
// 返回的 triedIPv4 表示这一轮是否真的试过 IPv4（决定要不要补查 A 记录）。
func dialAddresses(ctx context.Context, network, port string, ips []net.IP, attempts *[]dialAttempt) (net.Conn, bool) {
	dialer := net.Dialer{Timeout: dialTimeout, Control: DialControl}
	var triedIPv4 bool
	for _, ip := range orderIPv4First(ips) {
		addr := net.JoinHostPort(ip.String(), port)
		if isBlockedIP(ip) {
			*attempts = append(*attempts, dialAttempt{addr: addr, err: ErrBlockedHost})
			continue
		}
		if ip.To4() != nil {
			triedIPv4 = true
		}
		conn, err := dialFunc(ctx, &dialer, network, addr)
		if err == nil {
			return conn, triedIPv4
		}
		*attempts = append(*attempts, dialAttempt{addr: addr, err: err})
		if ctx.Err() != nil {
			// 客户端已取消或整体超时：再试下去没有意义。
			return nil, triedIPv4
		}
	}
	return nil, triedIPv4
}

// orderIPv4First 把 IPv4 排到前面。
//
// 这里不做 Happy Eyeballs 的并发竞速，只调整顺序：无 IPv6 路由的机器上 IPv6 连接
// 会立刻返回 ENETUNREACH（不是干等超时），顺序调整就够；反过来 IPv6-only 环境里
// IPv4 同样立刻失败，两种环境都不会被拖慢。
func orderIPv4First(ips []net.IP) []net.IP {
	ordered := make([]net.IP, 0, len(ips))
	for _, ip := range ips {
		if ip.To4() != nil {
			ordered = append(ordered, ip)
		}
	}
	for _, ip := range ips {
		if ip.To4() == nil {
			ordered = append(ordered, ip)
		}
	}
	return ordered
}

func get(ctx context.Context, rawURL string, timeout time.Duration, follow bool) (*http.Response, error) {
	if err := ValidatePublicHTTPURL(rawURL); err != nil {
		return nil, err
	}
	if timeout <= 0 {
		timeout = 60 * time.Second
	}
	if ctx == nil {
		ctx = context.Background()
	}

	client := &http.Client{
		Timeout: timeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if !follow {
				return http.ErrUseLastResponse
			}
			if len(via) >= 5 {
				return ErrTooManyRedirects
			}
			if err := ValidatePublicHTTPURL(req.URL.String()); err != nil {
				return err
			}
			return nil
		},
		Transport: newTransport(),
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "OneImg-SafeFetch/1.0")

	return client.Do(req)
}

// ReadLimited 读取响应体并限制最大字节。max+1 哨兵读法使超限可以被
// 显式检测：超限时返回 ErrBodyTooLarge，而非静默截断后让调用方拿到
// 一份被截断的数据并报出误导性错误。
func ReadLimited(r io.Reader, max int64) ([]byte, error) {
	if max <= 0 {
		max = 10 * 1024 * 1024
	}
	data, err := io.ReadAll(io.LimitReader(r, max+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > max {
		return nil, ErrBodyTooLarge
	}
	return data, nil
}
