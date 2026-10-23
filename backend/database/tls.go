package database

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"log"
	"net"
	"net/netip"
	"os"
	"strings"

	"oneimg/backend/config"

	sqlmysql "github.com/go-sql-driver/mysql"
)

// TLS 模式，取值与 PostgreSQL sslmode 对齐。
const (
	TLSModeAuto       = "auto"
	TLSModeDisable    = "disable"
	TLSModePreferred  = "preferred"
	TLSModeRequired   = "required"
	TLSModeVerifyCA   = "verify-ca"
	TLSModeVerifyFull = "verify-full"
)

// lookupHost 便于测试替换。
var lookupHost = net.LookupHost

// resolveTLSMode 返回最终生效的 TLS 模式及判定说明（用于启动日志）。
// DB_TLS_MODE 为具体模式时优先；为空或 auto 时，兼容旧配置 DB_SKIP_CERT_VERIFY=true
// 等价于 required（.env 模板默认写入 auto，不能因此让旧配置失效）；
// 其余情况按 DB_HOST 解析结果判定：全部为回环/私有地址时 preferred，否则 verify-full。
func resolveTLSMode(cfg *config.Config) (string, string) {
	if cfg.DbTLSMode != "" && cfg.DbTLSMode != TLSModeAuto {
		if cfg.DbSkipCertVerify {
			log.Printf("⚠️ 已设置 DB_TLS_MODE=%s，忽略 DB_SKIP_CERT_VERIFY", cfg.DbTLSMode)
		}
		return cfg.DbTLSMode, "DB_TLS_MODE=" + cfg.DbTLSMode
	}
	if cfg.DbSkipCertVerify {
		return TLSModeRequired, "DB_SKIP_CERT_VERIFY=true"
	}
	addrs, err := hostAddrs(cfg.DbHost)
	if err != nil {
		// 解析失败时不降级，连接阶段会给出具体错误。
		return TLSModeVerifyFull, fmt.Sprintf("auto: DB_HOST=%s 解析失败（%v）", cfg.DbHost, err)
	}
	for _, a := range addrs {
		if !isLocalAddr(a) {
			return TLSModeVerifyFull, fmt.Sprintf("auto: DB_HOST=%s → %s（公网地址）", cfg.DbHost, a)
		}
	}
	return TLSModePreferred, fmt.Sprintf("auto: DB_HOST=%s → %s（本机/私有地址）", cfg.DbHost, joinAddrs(addrs))
}

func hostAddrs(host string) ([]netip.Addr, error) {
	if a, err := netip.ParseAddr(host); err == nil {
		return []netip.Addr{a}, nil
	}
	raw, err := lookupHost(host)
	if err != nil {
		return nil, err
	}
	addrs := make([]netip.Addr, 0, len(raw))
	for _, s := range raw {
		if a, err := netip.ParseAddr(s); err == nil {
			addrs = append(addrs, a)
		}
	}
	if len(addrs) == 0 {
		return nil, errors.New("无可用地址")
	}
	return addrs, nil
}

// isLocalAddr 判断回环地址与私有地址（10/8、172.16/12、192.168/16、fc00::/7）。
func isLocalAddr(a netip.Addr) bool {
	a = a.Unmap()
	return a.IsLoopback() || a.IsPrivate()
}

func joinAddrs(addrs []netip.Addr) string {
	s := make([]string, len(addrs))
	for i, a := range addrs {
		s[i] = a.String()
	}
	return strings.Join(s, ", ")
}

func needsCA(mode string) bool {
	return mode == TLSModeVerifyCA || mode == TLSModeVerifyFull
}

// loadRootCAs 读取 DB_CA_CERT_PATH；未配置时返回 nil（使用系统 CA）。
// 配置了但无效时报错，绝不隐式降级。
func loadRootCAs(cfg *config.Config) (*x509.CertPool, string, error) {
	if cfg.DbCaCertPath == "" {
		return nil, "", nil
	}
	certPath, err := validateCAFile(cfg.DbCaCertPath)
	if err != nil {
		return nil, "", err
	}
	caCert, err := os.ReadFile(certPath)
	if err != nil {
		return nil, "", fmt.Errorf("读取CA证书失败: %w", err)
	}
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(caCert) {
		return nil, "", fmt.Errorf("解析CA证书失败")
	}
	return pool, certPath, nil
}

func tlsServerName(cfg *config.Config) string {
	if cfg.DbTLSServerName != "" {
		return cfg.DbTLSServerName
	}
	return cfg.DbHost
}

// mysqlTLSParam 返回 go-sql-driver 的 tls 参数值，必要时注册自定义配置。
func mysqlTLSParam(cfg *config.Config, mode string) (string, error) {
	switch mode {
	case TLSModeDisable:
		return "false", nil
	case TLSModePreferred:
		// 驱动内置：服务端支持则使用 TLS 且不校验证书，否则明文。
		return "preferred", nil
	}

	tlsConfig := &tls.Config{MinVersion: tls.VersionTLS12}
	switch mode {
	case TLSModeRequired:
		tlsConfig.InsecureSkipVerify = true
	case TLSModeVerifyCA:
		roots, _, err := loadRootCAs(cfg)
		if err != nil {
			return "", err
		}
		// 标准库不支持只校验 CA，关闭内置校验后手动验证证书链（不校验主机名）。
		tlsConfig.InsecureSkipVerify = true
		tlsConfig.VerifyConnection = func(cs tls.ConnectionState) error {
			return verifyChain(cs.PeerCertificates, roots)
		}
	case TLSModeVerifyFull:
		roots, _, err := loadRootCAs(cfg)
		if err != nil {
			return "", err
		}
		tlsConfig.RootCAs = roots
		tlsConfig.ServerName = tlsServerName(cfg)
	default:
		return "", fmt.Errorf("未知的 TLS 模式: %s", mode)
	}

	const name = "custom_tls"
	if err := sqlmysql.RegisterTLSConfig(name, tlsConfig); err != nil {
		return "", err
	}
	return name, nil
}

func verifyChain(certs []*x509.Certificate, roots *x509.CertPool) error {
	if len(certs) == 0 {
		return errors.New("服务端未提供证书")
	}
	opts := x509.VerifyOptions{Roots: roots, Intermediates: x509.NewCertPool()}
	for _, c := range certs[1:] {
		opts.Intermediates.AddCert(c)
	}
	_, err := certs[0].Verify(opts)
	return err
}

// postgresSSLParams 返回 sslmode 及（仅校验模式下的）sslrootcert。
func postgresSSLParams(cfg *config.Config, mode string) (map[string]string, error) {
	sslmode := map[string]string{
		TLSModeDisable:    "disable",
		TLSModePreferred:  "prefer",
		TLSModeRequired:   "require",
		TLSModeVerifyCA:   "verify-ca",
		TLSModeVerifyFull: "verify-full",
	}[mode]
	if sslmode == "" {
		return nil, fmt.Errorf("未知的 TLS 模式: %s", mode)
	}
	params := map[string]string{"sslmode": sslmode}
	// 非校验模式不传 sslrootcert：按 libpq 语义，require 带根证书会升级为 verify-ca。
	if needsCA(mode) {
		_, certPath, err := loadRootCAs(cfg)
		if err != nil {
			return nil, err
		}
		if certPath != "" {
			params["sslrootcert"] = certPath
		}
	}
	return params, nil
}
