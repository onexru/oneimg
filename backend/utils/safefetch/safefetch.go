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
	"time"
)

var (
	ErrInvalidURL       = errors.New("URL 无效")
	ErrBlockedHost      = errors.New("禁止访问内网或保留地址")
	ErrBlockedScheme    = errors.New("仅允许 http/https")
	ErrTooManyRedirects = errors.New("重定向次数过多")
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
			if len(via) >= 5 {
				return ErrTooManyRedirects
			}
			if err := ValidatePublicHTTPURL(req.URL.String()); err != nil {
				return err
			}
			return nil
		},
		Transport: &http.Transport{
			Proxy: http.ProxyFromEnvironment,
			DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				host, port, err := net.SplitHostPort(addr)
				if err != nil {
					return nil, err
				}
				if err := assertPublicHost(host); err != nil {
					return nil, err
				}
				ips, err := net.LookupIP(host)
				if err != nil {
					return nil, err
				}
				var lastErr error
				d := net.Dialer{Timeout: 10 * time.Second}
				for _, ip := range ips {
					if isBlockedIP(ip) {
						lastErr = ErrBlockedHost
						continue
					}
					conn, dialErr := d.DialContext(ctx, network, net.JoinHostPort(ip.String(), port))
					if dialErr == nil {
						return conn, nil
					}
					lastErr = dialErr
				}
				if lastErr == nil {
					lastErr = ErrBlockedHost
				}
				return nil, lastErr
			},
			// 禁用 HTTP/2 与 keep-alive 不是必须，保持默认即可
		},
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "OneImg-SafeFetch/1.0")

	return client.Do(req)
}

// ReadLimited 读取响应体并限制最大字节。
func ReadLimited(r io.Reader, max int64) ([]byte, error) {
	if max <= 0 {
		max = 10 * 1024 * 1024
	}
	return io.ReadAll(io.LimitReader(r, max+1))
}
