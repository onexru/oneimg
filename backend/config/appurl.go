package config

import (
	"log"
	"net/url"
	"strings"
)

// APP_URLS 支持「一个实例多个访问域名」。
//
// 背景：同源校验（SameOriginWrites）、CORS 白名单、直传的 Origin 检查、外部登录
// 回调的同源校验，全都以「本实例自己的地址」为判断依据。只有一个 APP_URL 时，
// 新老域名并存、内网名、CDN 回源域名都会被当成外来来源挡掉（403 请求来源无效）。
//
// 约定：
//   - 配了 APP_URLS 就以它的第一项作为主地址（AppURL）；
//   - 没配 APP_URLS 时行为与以前完全一样，APP_URL 既是主地址也是唯一可信来源；
//   - 主地址只用于对外产物（图片链接、Cookie Secure、外部登录回调默认地址），
//     可信来源列表用于校验，两者分开，避免同一张图在不同域名下返回不同 URL。

// parseAppURLs 把 APP_URLS 解析成规范化、去重后的地址列表。
// 为空时回退到 primary（即 APP_URL），保证老配置行为不变。
func parseAppURLs(rawList, primary string) []string {
	urls := make([]string, 0, 4)
	seen := make(map[string]struct{}, 4)

	for _, raw := range splitAppURLList(rawList) {
		normalized, ok := normalizeAppOrigin(raw)
		if !ok {
			log.Printf("警告：忽略非法的 APP_URLS 项 %q（只接受 http://host[:port] 或 https://host[:port]）", raw)
			continue
		}
		key := strings.ToLower(normalized)
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		urls = append(urls, normalized)
	}

	if len(urls) > 0 {
		return urls
	}
	if normalized, ok := normalizeAppOrigin(primary); ok {
		return []string{normalized}
	}
	// 主地址非法时保留原值，让既有的「APP_URL 无效」判断继续生效。
	if trimmed := strings.TrimSpace(primary); trimmed != "" {
		return []string{trimmed}
	}
	return nil
}

// splitAppURLList 允许逗号、分号、空白混用，避免用户在 .env 里写多行或加空格时踩坑。
func splitAppURLList(raw string) []string {
	fields := strings.FieldsFunc(raw, func(r rune) bool {
		switch r {
		case ',', ';', ' ', '\t', '\r', '\n':
			return true
		}
		return false
	})
	result := make([]string, 0, len(fields))
	for _, field := range fields {
		if trimmed := strings.TrimSpace(field); trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

// normalizeAppOrigin 只接受「纯粹的 scheme://host[:port]」：带路径、查询、片段、
// 用户名密码的一律拒绝——这些写法放进可信来源列表会放大判断口径。
func normalizeAppOrigin(raw string) (string, bool) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", false
	}
	u, err := url.Parse(trimmed)
	if err != nil {
		return "", false
	}
	scheme := strings.ToLower(u.Scheme)
	if scheme != "http" && scheme != "https" {
		return "", false
	}
	if u.User != nil || u.RawQuery != "" || u.Fragment != "" || u.Host == "" {
		return "", false
	}
	if u.Path != "" && u.Path != "/" {
		return "", false
	}
	return scheme + "://" + strings.ToLower(u.Host), true
}

// TrustedAppOrigins 返回可用于同源/CORS 校验的地址列表。
// AppURLs 为空时退回 AppURL，兼容直接构造 Config 的调用方（测试、插件）。
func (c *Config) TrustedAppOrigins() []string {
	if c == nil {
		return nil
	}
	if len(c.AppURLs) > 0 {
		return c.AppURLs
	}
	if normalized, ok := normalizeAppOrigin(c.AppURL); ok {
		return []string{normalized}
	}
	return nil
}

// TrustsOrigin 判断请求的 Origin 是否来自本站。
func (c *Config) TrustsOrigin(origin string) bool {
	return MatchAppOrigin(origin, c.TrustedAppOrigins())
}

// MatchAppOrigin 判断 Origin 是否来自本站，口径只有一条：**只比域名**。
//
//   - 协议与端口都不参与比较：https/http、:8443/:443 视为同一个站；
//   - 不做子域名通配：配了 example.com 不代表 a.example.com 可信，需要就单独列；
//   - 回环写法等价：localhost / 127.0.0.1 / [::1]（浏览器里指向同一个服务）；
//   - 仍然拒绝带路径/查询/片段/账号密码的 Origin——浏览器的 Origin 头永远只有
//     scheme://host[:port]，出现更多内容说明不是普通浏览器请求。
//
// 为什么放宽端口：反向代理、内网端口映射、前端 dev server 会让同一个站点出现
// 多个端口，按端口比对会把自家页面挡成 403「请求来源无效」。判定信任的对象是
// 「哪个域名归本站所有」，不是「哪个端口」；同一域名下的端口都由站点自己控制，
// 换端口不会多出一个新的信任方。
func MatchAppOrigin(origin string, allowed []string) bool {
	x, err := url.Parse(strings.TrimSpace(origin))
	if err != nil {
		return false
	}
	if x.Scheme == "" || x.Host == "" || x.User != nil || x.RawQuery != "" || x.Fragment != "" {
		return false
	}
	if x.Path != "" && x.Path != "/" {
		return false
	}
	host := normalizeMatchHost(x.Hostname())
	if host == "" {
		return false
	}
	for _, raw := range allowed {
		y, err := url.Parse(strings.TrimSpace(raw))
		if err != nil {
			continue
		}
		candidate := normalizeMatchHost(y.Hostname())
		if candidate == "" {
			continue
		}
		if host == candidate {
			return true
		}
		if isLoopbackHost(host) && isLoopbackHost(candidate) {
			return true
		}
	}
	return false
}

// normalizeMatchHost 把主机名归一成可比较形式：小写、去尾点。
// IPv6 的方括号由 url.Hostname() 负责剥掉。
func normalizeMatchHost(host string) string {
	return strings.TrimSuffix(strings.ToLower(strings.TrimSpace(host)), ".")
}

// isLoopbackHost 判断主机名是否为本机回环写法。
// 只认这三个名字：127.0.0.0/8 的其它地址在浏览器里属于不同站点，没有等价的意义。
func isLoopbackHost(host string) bool {
	switch strings.ToLower(strings.TrimSpace(host)) {
	case "localhost", "127.0.0.1", "::1":
		return true
	}
	return false
}

// isLoopbackOrigin 判断一个完整来源地址是否指向本机，用于启动日志提示。
func isLoopbackOrigin(raw string) bool {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return false
	}
	return isLoopbackHost(u.Hostname())
}
