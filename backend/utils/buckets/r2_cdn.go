package buckets

import (
	"errors"
	"net"
	"net/url"
	"strings"
)

const R2CDNPublic = "public"
const R2CDNSignedProxy = "signed_proxy"

// NormalizeR2CDN is shared by saving, draft testing and serving. No requests or
// credentials are involved; optional path prefixes are preserved, not guessed.
func NormalizeR2CDN(domain, mode string) (string, string, error) {
	domain = strings.TrimSpace(domain)
	mode = strings.TrimSpace(mode)
	if mode == "" {
		mode = R2CDNPublic
	}
	if mode != R2CDNPublic && mode != R2CDNSignedProxy {
		return "", "", errors.New("R2 CDN 访问方式无效")
	}
	if domain == "" {
		return "", mode, nil
	}
	if !strings.Contains(domain, "://") {
		domain = "https://" + domain
	}
	u, err := url.Parse(domain)
	if err != nil || u.Scheme != "https" || u.Hostname() == "" || u.User != nil || u.RawQuery != "" || u.ForceQuery || u.Fragment != "" || strings.ContainsAny(domain, "\\\r\n\t") {
		return "", "", errors.New("CDN 地址须为 HTTPS 域名，可带路径前缀，不能包含账号、查询参数或锚点")
	}
	host := strings.ToLower(u.Hostname())
	if !strings.Contains(host, ".") || host == "localhost" || strings.HasSuffix(host, ".localhost") || strings.HasSuffix(host, ".local") || strings.HasSuffix(host, ".internal") || strings.HasSuffix(host, ".r2.cloudflarestorage.com") || net.ParseIP(host) != nil {
		return "", "", errors.New("CDN 地址请填写自定义公网域名，不是 R2 S3 API 地址或 IP")
	}
	for _, segment := range strings.Split(u.Path, "/") {
		if segment == "." || segment == ".." {
			return "", "", errors.New("CDN 路径前缀不能包含相对目录")
		}
	}
	// Preserve escapes such as %20 in a configured prefix.
	u.Path = strings.TrimRight(u.Path, "/")
	u.RawPath = strings.TrimRight(u.RawPath, "/")
	return u.String(), mode, nil
}

func R2CDNConfig(config map[string]any) (string, string, error) {
	domain, _ := config["r2_cdn_domain"].(string)
	mode, _ := config["r2_cdn_mode"].(string)
	return NormalizeR2CDN(domain, mode)
}

func R2CDNObjectURL(domain, objectKey string) (string, error) {
	base, _, err := NormalizeR2CDN(domain, R2CDNPublic)
	if err != nil {
		return "", err
	}
	if base == "" {
		return "", errors.New("CDN 地址为空")
	}
	u, _ := url.Parse(base)
	key := strings.TrimPrefix(objectKey, "/")
	if key == "" || strings.ContainsAny(key, "\\\r\n") {
		return "", errors.New("对象路径无效")
	}
	for _, segment := range strings.Split(key, "/") {
		if segment == "." || segment == ".." {
			return "", errors.New("对象路径不能包含相对目录")
		}
	}
	escaped := u.EscapedPath() + "/" + (&url.URL{Path: key}).EscapedPath()
	u.Path, err = url.PathUnescape(escaped)
	u.RawPath = escaped
	if err != nil {
		return "", err
	}
	return u.String(), nil
}

// This is only valid for an explicitly configured reverse proxy that restores
// the original R2 Host and removes the CDN prefix. Native R2 custom domains
// cannot validate S3 SigV4 signatures. Never drop or regenerate its query here.
func R2CDNSignedURL(domain string, signed *url.URL) (string, error) {
	base, _, err := NormalizeR2CDN(domain, R2CDNSignedProxy)
	if err != nil {
		return "", err
	}
	if base == "" || signed == nil || signed.RawQuery == "" {
		return "", errors.New("签名反代配置无效")
	}
	u, _ := url.Parse(base)
	escaped := u.EscapedPath() + signed.EscapedPath()
	u.Path, err = url.PathUnescape(escaped)
	if err != nil {
		return "", err
	}
	u.RawPath = escaped
	u.RawQuery = signed.RawQuery
	return u.String(), nil
}
