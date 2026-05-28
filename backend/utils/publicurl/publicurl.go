package publicurl

import (
	"fmt"
	"net/url"
	"strings"

	"oneimg/backend/models"
)

func NormalizeDomain(value string) (string, error) {
	domain := strings.TrimSpace(value)
	if domain == "" {
		return "", nil
	}

	if !strings.Contains(domain, "://") {
		domain = "https://" + domain
	}

	parsed, err := url.Parse(domain)
	if err != nil {
		return "", fmt.Errorf("图片直链域名格式错误")
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", fmt.Errorf("图片直链域名仅支持 http 或 https")
	}
	if parsed.Host == "" {
		return "", fmt.Errorf("图片直链域名缺少域名")
	}
	if parsed.RawQuery != "" || parsed.Fragment != "" {
		return "", fmt.Errorf("图片直链域名不能包含查询参数或锚点")
	}

	parsed.Path = strings.TrimRight(parsed.Path, "/")
	return strings.TrimRight(parsed.String(), "/"), nil
}

func HasDomain(setting models.Settings) bool {
	domain, err := NormalizeDomain(setting.PublicImageDomain)
	return err == nil && domain != ""
}

func Build(setting models.Settings, imagePath string) string {
	path := strings.TrimSpace(imagePath)
	if path == "" || isAbsoluteURL(path) {
		return path
	}

	domain, err := NormalizeDomain(setting.PublicImageDomain)
	if err != nil || domain == "" {
		return path
	}

	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	return strings.TrimRight(domain, "/") + path
}

func BuildForStorage(setting models.Settings, storage string, imagePath string) string {
	if !SupportsStorage(storage) {
		return imagePath
	}
	return Build(setting, imagePath)
}

func SupportsStorage(storage string) bool {
	return storage == "r2" || storage == "s3"
}

func isAbsoluteURL(value string) bool {
	return strings.HasPrefix(value, "http://") || strings.HasPrefix(value, "https://")
}
