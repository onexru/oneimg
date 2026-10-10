// Package powverify implements the legacy {token}/{success} JSON contract.
// Only the administrator-configured HTTPS endpoint is contacted: no proxy,
// redirect, implicit fallback host, or browser User-Agent impersonation.
package powverify

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strings"
	"time"
)

const maxResponseBytes = 64 << 10

var ErrEndpoint = errors.New("POW 验证地址必须为公共 HTTPS 地址（443 端口，无凭据、查询参数或片段）")
var blockedPrefixes = []netip.Prefix{
	netip.MustParsePrefix("0.0.0.0/8"), netip.MustParsePrefix("10.0.0.0/8"), netip.MustParsePrefix("100.64.0.0/10"),
	netip.MustParsePrefix("127.0.0.0/8"), netip.MustParsePrefix("169.254.0.0/16"), netip.MustParsePrefix("172.16.0.0/12"),
	netip.MustParsePrefix("192.0.0.0/24"), netip.MustParsePrefix("192.88.99.0/24"), netip.MustParsePrefix("192.0.2.0/24"), netip.MustParsePrefix("192.168.0.0/16"),
	netip.MustParsePrefix("198.18.0.0/15"), netip.MustParsePrefix("198.51.100.0/24"), netip.MustParsePrefix("203.0.113.0/24"),
	netip.MustParsePrefix("224.0.0.0/3"), netip.MustParsePrefix("::/96"), netip.MustParsePrefix("64:ff9b::/96"),
	netip.MustParsePrefix("64:ff9b:1::/48"), netip.MustParsePrefix("100::/64"), netip.MustParsePrefix("2001::/23"),
	netip.MustParsePrefix("2001:db8::/32"), netip.MustParsePrefix("2002::/16"), netip.MustParsePrefix("fc00::/7"),
	netip.MustParsePrefix("fe80::/10"), netip.MustParsePrefix("fec0::/10"), netip.MustParsePrefix("ff00::/8"), netip.MustParsePrefix("3fff::/20"),
}

func publicIP(ip netip.Addr) bool {
	ip = ip.Unmap()
	if !ip.IsValid() || !ip.IsGlobalUnicast() || ip.IsPrivate() || ip.IsLoopback() || ip.IsLinkLocalUnicast() {
		return false
	}
	for _, p := range blockedPrefixes {
		if p.Contains(ip) {
			return false
		}
	}
	return true
}

// NormalizeURL is intentionally offline. Dial resolves again under the request
// deadline and pins a vetted IP, preventing DNS rebinding and mixed-answer SSRF.
func NormalizeURL(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", nil
	}
	if len(raw) > 2048 {
		return "", ErrEndpoint
	}
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.Host == "" || u.User != nil || u.RawQuery != "" || u.ForceQuery || u.Fragment != "" || strings.Contains(raw, "#") || u.Opaque != "" {
		return "", ErrEndpoint
	}
	host := strings.ToLower(strings.TrimSuffix(u.Hostname(), "."))
	if host == "" || len(host) > 253 || strings.ContainsAny(host, "%\\ /\t\r\n") || (u.Port() != "" && u.Port() != "443") || strings.HasSuffix(u.Host, ":") {
		return "", ErrEndpoint
	}
	if ip, err := netip.ParseAddr(host); err == nil {
		if !publicIP(ip) {
			return "", ErrEndpoint
		}
	} else {
		if !strings.Contains(host, ".") || host == "localhost" {
			return "", ErrEndpoint
		}
		for _, suffix := range []string{".localhost", ".local", ".internal", ".lan", ".home", ".test", ".invalid", ".example", ".onion"} {
			if strings.HasSuffix(host, suffix) {
				return "", ErrEndpoint
			}
		}
		for _, label := range strings.Split(host, ".") {
			if len(label) < 1 || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
				return "", ErrEndpoint
			}
			for _, r := range label {
				if !(r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '-') {
					return "", ErrEndpoint
				}
			}
		}
	}
	return u.String(), nil
}

type lookupFunc func(context.Context, string) ([]net.IPAddr, error)
type dialFunc func(context.Context, string, string) (net.Conn, error)

func publicDial(lookup lookupFunc, dial dialFunc) dialFunc {
	return func(ctx context.Context, network, address string) (net.Conn, error) {
		host, port, err := net.SplitHostPort(address)
		if err != nil || port != "443" {
			return nil, ErrEndpoint
		}
		var ips []net.IPAddr
		if ip := net.ParseIP(host); ip != nil {
			ips = []net.IPAddr{{IP: ip}}
		} else {
			ips, err = lookup(ctx, host)
			if err != nil {
				return nil, err
			}
		}
		if len(ips) == 0 {
			return nil, ErrEndpoint
		}
		// Reject all mixed public/private DNS answers, rather than merely skip private.
		for _, ip := range ips {
			addr, ok := netip.AddrFromSlice(ip.IP)
			if !ok || ip.Zone != "" || !publicIP(addr) {
				return nil, ErrEndpoint
			}
		}
		for _, ip := range ips {
			conn, e := dial(ctx, network, net.JoinHostPort(ip.IP.String(), port))
			if e == nil {
				return conn, nil
			}
			err = e
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
		}
		return nil, err
	}
}
func newClient(timeout time.Duration) (*http.Client, *http.Transport) {
	d := &net.Dialer{Timeout: timeout}
	tr := &http.Transport{Proxy: nil, DisableCompression: true, DisableKeepAlives: true,
		DialContext:         publicDial(net.DefaultResolver.LookupIPAddr, d.DialContext),
		TLSHandshakeTimeout: timeout, ResponseHeaderTimeout: timeout, MaxResponseHeaderBytes: 16 << 10}
	return &http.Client{Timeout: timeout, Transport: tr, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}, tr
}

func Validate(ctx context.Context, endpoint, token string, timeout time.Duration) bool {
	if timeout < time.Second || timeout > 15*time.Second {
		return false
	}
	client, tr := newClient(timeout)
	defer tr.CloseIdleConnections()
	return validate(ctx, client, endpoint, token, timeout)
}
func validate(ctx context.Context, client *http.Client, endpoint, token string, timeout time.Duration) bool {
	if ctx == nil || strings.TrimSpace(token) == "" || len(token) > 16<<10 {
		return false
	}
	endpoint, err := NormalizeURL(endpoint)
	if err != nil || endpoint == "" {
		return false
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	body, err := json.Marshal(struct {
		Token string `json:"token"`
	}{token})
	if err != nil {
		return false
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return false
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "OneImg-POW/1.0")
	resp, err := client.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return false
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes+1))
	if err != nil || len(data) > maxResponseBytes || ctx.Err() != nil {
		return false
	}
	success, err := parseSuccess(data)
	return err == nil && success
}

// Reject duplicate keys, non-object/boolean responses and trailing JSON. Other
// provider fields remain compatible but never influence the decision.
func parseSuccess(body []byte) (bool, error) {
	dec := json.NewDecoder(bytes.NewReader(body))
	token, err := dec.Token()
	if err != nil || token != json.Delim('{') {
		return false, fmt.Errorf("malformed POW response")
	}
	seen := map[string]bool{}
	success := false
	for dec.More() {
		token, err = dec.Token()
		if err != nil {
			return false, fmt.Errorf("malformed POW response")
		}
		key, ok := token.(string)
		if !ok || seen[key] {
			return false, fmt.Errorf("malformed POW response")
		}
		seen[key] = true
		var raw json.RawMessage
		if err := dec.Decode(&raw); err != nil {
			return false, fmt.Errorf("malformed POW response")
		}
		if key == "success" {
			value := string(bytes.TrimSpace(raw))
			if value != "true" && value != "false" {
				return false, fmt.Errorf("malformed POW response")
			}
			success = value == "true"
		}
	}
	token, err = dec.Token()
	if err != nil || token != json.Delim('}') || !seen["success"] {
		return false, fmt.Errorf("malformed POW response")
	}
	var extra any
	if err := dec.Decode(&extra); !errors.Is(err, io.EOF) {
		return false, fmt.Errorf("malformed POW response")
	}
	return success, nil
}
