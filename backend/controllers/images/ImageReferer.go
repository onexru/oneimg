package images

import (
	"errors"
	"net/url"
	"oneimg/backend/config"
	"strings"

	"github.com/gin-gonic/gin"
)

// 防盗链判断的也是「哪个站」：只比域名，协议与端口都不参与比较。
// 与同源校验（config.MatchAppOrigin）保持同一口径，避免同一台实例上
// 「CORS 放行、图片却 403」这种自相矛盾的表现。

// 辅助函数，校验来源
func checkReferer(referer, whiteList string, selfDomains []string) bool {
	return checkRefererPolicy(referer, whiteList, selfDomains, true)
}
func checkRefererPolicy(referer, whiteList string, selfDomains []string, allowEmpty bool) bool {
	if strings.TrimSpace(referer) == "" {
		return allowEmpty
	}
	domain, err := extractDomainFromReferer(referer)
	if err != nil {
		return false
	}
	allowed := append(strings.Split(whiteList, ","), selfDomains...)
	for _, raw := range allowed {
		host, ok := normalizeRefererHost(raw)
		if !ok {
			continue
		}
		// 只认精确域名：配了 example.com 不等于信任 cdn.example.com。
		// 需要子域名就把它单独写进白名单（或用 APP_URLS 列出），避免
		// 「随便一个 *.example.com 都能热链」这种口子。
		if domain == host {
			return true
		}
	}
	return false
}

// normalizeRefererHost 把白名单条目归一成可比较的主机名。
//
// 防盗链判断的是「哪个站」，协议、端口、路径都不参与比较（Referer 的 host 本身
// 也只剩主机名），所以这里统一剥掉：`https://a.com/`、`a.com:8080`、`[::1]:8080`
// 都会被认成正确的主机名。
//
// 以前遇到冒号、斜杠就整条跳过，等于配置静默失效——用户按习惯填了
// `https://a.com`，自家图片照样 403，还没有任何提示。真正该跳过的只有
// 通配符和写坏的条目。
func normalizeRefererHost(raw string) (string, bool) {
	entry := strings.ToLower(strings.TrimSpace(raw))
	if entry == "" || strings.ContainsAny(entry, " \t\r\n*") {
		return "", false
	}
	if i := strings.Index(entry, "://"); i >= 0 {
		switch entry[:i] {
		case "http", "https":
			entry = entry[i+3:]
		default:
			return "", false
		}
	}
	if i := strings.IndexAny(entry, "/?#"); i >= 0 {
		entry = entry[:i]
	}
	if entry == "" {
		return "", false
	}
	// 裸 IPv6（如 ::1）没有方括号时补上，否则 url.Parse 会把后半段当成端口。
	if strings.Count(entry, ":") > 1 && !strings.HasPrefix(entry, "[") {
		entry = "[" + entry + "]"
	}
	u, err := url.Parse("http://" + entry)
	if err != nil || u.User != nil || u.Hostname() == "" {
		return "", false
	}
	host := strings.TrimSuffix(u.Hostname(), ".")
	if host == "" || strings.ContainsAny(host, "/?#@ \t\r\n") {
		return "", false
	}
	return host, true
}
func extractDomainFromReferer(referer string) (string, error) {
	u, err := url.Parse(referer)
	if err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.User != nil || u.Hostname() == "" {
		return "", errors.New("invalid referer")
	}
	return strings.ToLower(u.Hostname()), nil
}

// Never derive a trust boundary from client-supplied Host/Forwarded headers.
// 多域名部署时每个配置地址都算「自己」，否则从备用域名打开的页面会被防盗链挡住。
func GetSelfDomains(c *gin.Context) []string {
	if config.App == nil {
		return nil
	}
	origins := config.App.TrustedAppOrigins()
	domains := make([]string, 0, len(origins))
	seen := make(map[string]struct{}, len(origins))
	for _, raw := range origins {
		host, ok := normalizeRefererHost(raw)
		if !ok {
			continue
		}
		if _, exists := seen[host]; exists {
			continue
		}
		seen[host] = struct{}{}
		domains = append(domains, host)
		// localhost / 127.0.0.1 / [::1] 是同一台机器的三种写法（浏览器对
		// *.localhost 也按回环解析）。本站含其中任一项时补齐另外两项，
		// 否则「CORS 认 127.0.0.1、防盗链不认」会在本地联调时表现成图片 403。
		for _, alias := range loopbackAliases(host) {
			if _, exists := seen[alias]; exists {
				continue
			}
			seen[alias] = struct{}{}
			domains = append(domains, alias)
		}
	}
	return domains
}

// loopbackAliases 给出回环主机名的等价写法；非回环主机返回 nil。
func loopbackAliases(host string) []string {
	switch host {
	case "localhost":
		return []string{"127.0.0.1", "::1"}
	case "127.0.0.1":
		return []string{"localhost", "::1"}
	case "::1":
		return []string{"localhost", "127.0.0.1"}
	}
	return nil
}
