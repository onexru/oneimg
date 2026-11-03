package storage

import (
	"errors"
	"fmt"
	"net/url"
	"strings"

	"oneimg/backend/models"
	"oneimg/backend/utils/safefetch"
	"oneimg/backend/utils/secureconfig"
)

// ErrStorageEndpointBlocked 表示存储端点指向内网/保留地址。
var ErrStorageEndpointBlocked = errors.New("存储地址不允许指向内网或保留地址")

// storageEndpointOf 取出某种存储类型用于出站连接的「主机名」。
//
// 为什么在控制器统一校验，而不是在各 driver 里：
// 存储驱动自己 dial TCP（FTP 用 goftp、WebDAV 用裸 http.Client、S3 用 minio SDK），
// 套不上 safefetch 那套带 DialControl 的 Transport。控制器是唯一公共入口，
// 在这里拦一次就能覆盖所有类型。
func storageEndpointOf(bucketType string, config map[string]any) (host string, ok bool) {
	str := func(key string) string {
		v, exists := config[key]
		if !exists || v == nil {
			return ""
		}
		return strings.TrimSpace(fmt.Sprintf("%v", v))
	}

	switch bucketType {
	case "s3":
		return hostFromEndpoint(str("s3_endpoint"))
	case "r2":
		return hostFromEndpoint(str("r2_endpoint"))
	case "webdav":
		return hostFromEndpoint(str("webdav_url"))
	case "ftp":
		return hostFromEndpoint(str("ftp_host"))
	case "telegram", "default":
		// telegram 的端点由 TG API 固定；本地存储不产生出站连接。
		return "", false
	default:
		return "", false
	}
}

// hostFromEndpoint 从可能带协议的端点串里取出主机名。
// 兼容 "https://host:9000/path"、"host:9000"、"host"、"//host" 四种写法。
func hostFromEndpoint(raw string) (string, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", false
	}
	if strings.Contains(raw, "://") {
		u, err := url.Parse(raw)
		if err != nil || u.Hostname() == "" {
			return "", false
		}
		return u.Hostname(), true
	}
	host := strings.TrimPrefix(raw, "//")
	if idx := strings.Index(host, "/"); idx >= 0 {
		host = host[:idx]
	}
	if strings.HasPrefix(host, "[") {
		if end := strings.Index(host, "]"); end > 0 {
			return host[1:end], true
		}
	}
	if idx := strings.LastIndex(host, ":"); idx > 0 {
		host = host[:idx]
	}
	host = strings.Trim(host, "[]")
	if host == "" {
		return "", false
	}
	return host, true
}

// validateStorageEndpoint 在建立任何出站连接前校验存储端点。
//
// 背景：`/api/buckets/test` 会用库里保存的凭据向用户填写的地址发起真实连接
// （WebDAV 走 BasicAuth 发 Username/Password、FTP 发 USER/PASS），
// 并且 WebDAV/FTP 还会写入一个测试文件再删除。
// 此前完全不校验地址 ⇒ 持有 storage:create 的账号可以把地址指向
// 127.0.0.1 / 10.x / 169.254.169.254（云 metadata），既能探测内网，
// 也能往内网 FTP/WebDAV 写入内容。
//
// 这里校验**解析结果**是否落在公网。连接期的 DNS rebinding 由
// safefetch.DialControl 在 driver 里兜底。
func validateStorageEndpoint(bucketType string, config map[string]any) error {
	host, ok := storageEndpointOf(bucketType, config)
	if !ok {
		return nil
	}
	if err := safefetch.AssertPublicEndpoint(host); err != nil {
		return fmt.Errorf("%w：%s", ErrStorageEndpointBlocked, host)
	}
	return nil
}

// validateBucketOutbound 对库里保存的存储桶做端点校验。
// 配置可能带加密值，先解密再取主机名，避免「加密态取不到 endpoint」而漏检。
func validateBucketOutbound(bucket models.Buckets) error {
	config := bucket.Config
	if decrypted, err := secureconfig.DecryptBucketConfigValues(config); err == nil {
		config = decrypted
	}
	return validateStorageEndpoint(bucket.Type, config)
}
