package database

import (
	"errors"
	"net/netip"
	"testing"

	"oneimg/backend/config"
)

func TestResolveTLSMode(t *testing.T) {
	hosts := map[string][]string{
		"mysql":        {"172.18.0.5"},
		"db.internal":  {"10.0.0.8", "fd00::8"},
		"tidb.example": {"34.1.2.3"},
		"split.local":  {"192.168.1.2", "8.8.8.8"},
	}
	old := lookupHost
	lookupHost = func(h string) ([]string, error) {
		if a, ok := hosts[h]; ok {
			return a, nil
		}
		return nil, errors.New("no such host")
	}
	defer func() { lookupHost = old }()

	cases := []struct {
		name string
		cfg  config.Config
		want string
	}{
		{"docker service name", config.Config{DbHost: "mysql"}, TLSModePreferred},
		{"explicit auto", config.Config{DbHost: "mysql", DbTLSMode: "auto"}, TLSModePreferred},
		{"loopback ip", config.Config{DbHost: "127.0.0.1"}, TLSModePreferred},
		{"localhost v6", config.Config{DbHost: "::1"}, TLSModePreferred},
		{"private v4+v6", config.Config{DbHost: "db.internal"}, TLSModePreferred},
		{"public host", config.Config{DbHost: "tidb.example"}, TLSModeVerifyFull},
		{"public ip", config.Config{DbHost: "8.8.8.8"}, TLSModeVerifyFull},
		{"mixed addresses", config.Config{DbHost: "split.local"}, TLSModeVerifyFull},
		{"lookup failure", config.Config{DbHost: "nope"}, TLSModeVerifyFull},
		{"explicit overrides auto", config.Config{DbHost: "mysql", DbTLSMode: TLSModeVerifyFull}, TLSModeVerifyFull},
		{"explicit disable on public", config.Config{DbHost: "tidb.example", DbTLSMode: TLSModeDisable}, TLSModeDisable},
		{"legacy skip verify", config.Config{DbHost: "tidb.example", DbSkipCertVerify: true}, TLSModeRequired},
		{"skip verify ignored when explicit", config.Config{DbHost: "mysql", DbSkipCertVerify: true, DbTLSMode: TLSModeVerifyCA}, TLSModeVerifyCA},
		{"skip verify with template auto", config.Config{DbHost: "tidb.example", DbSkipCertVerify: true, DbTLSMode: "auto"}, TLSModeRequired},
	}
	for _, c := range cases {
		cfg := c.cfg
		if got, reason := resolveTLSMode(&cfg); got != c.want {
			t.Errorf("%s: got %s (%s), want %s", c.name, got, reason, c.want)
		}
	}
}

func TestIsLocalAddr(t *testing.T) {
	for s, want := range map[string]bool{
		"127.0.0.1": true, "::1": true, "10.1.2.3": true, "172.16.0.1": true, "172.31.255.255": true,
		"192.168.0.1": true, "fd12::1": true, "::ffff:192.168.0.1": true,
		"172.32.0.1": false, "8.8.8.8": false, "100.64.0.1": false, "169.254.1.1": false, "2001:db8::1": false,
	} {
		if got := isLocalAddr(netip.MustParseAddr(s)); got != want {
			t.Errorf("%s: got %v, want %v", s, got, want)
		}
	}
}

func TestPostgresSSLParamsOmitsRootCertWithoutVerification(t *testing.T) {
	cfg := &config.Config{DbCaCertPath: "./does-not-exist.pem"}
	for _, mode := range []string{TLSModeDisable, TLSModePreferred, TLSModeRequired} {
		p, err := postgresSSLParams(cfg, mode)
		if err != nil {
			t.Fatalf("%s: %v", mode, err)
		}
		if _, ok := p["sslrootcert"]; ok {
			t.Errorf("%s: sslrootcert must not be set", mode)
		}
	}
	// 校验模式下，配置了无效 CA 必须报错而不是降级。
	if _, err := postgresSSLParams(cfg, TLSModeVerifyFull); err == nil {
		t.Error("verify-full with missing CA must fail")
	}
}
