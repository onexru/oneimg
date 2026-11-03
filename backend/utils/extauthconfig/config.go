package extauthconfig

import (
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"strings"

	"oneimg/backend/config"
	"oneimg/backend/models"
)

// OIDCClaimNameRegex 校验 OIDC 用户名 Claim 的格式。
// 与设置校验（system 包）和认证流程（auth 包）共用，避免两处各写一份而走样。
var OIDCClaimNameRegex = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_.:-]{0,127}$`)

func NormalizeOIDCIssuer(raw string) (string, error) {
	normalized, parsed, err := NormalizeExternalURL(raw, false)
	if err != nil {
		return "", fmt.Errorf("OIDC Issuer URL 无效: %w", err)
	}
	if normalized == "" {
		return "", nil
	}
	if parsed.Scheme != "https" && !IsLoopbackHostname(parsed.Hostname()) {
		return "", errors.New("OIDC Issuer 必须使用 HTTPS（localhost 开发环境除外）")
	}
	return parsed.String(), nil
}

func NormalizeCASServerURL(raw string) (string, error) {
	normalized, parsed, err := NormalizeExternalURL(raw, false)
	if err != nil {
		return "", fmt.Errorf("CAS Server URL 无效: %w", err)
	}
	if normalized == "" {
		return "", nil
	}
	if parsed.Scheme != "https" && !IsLoopbackHostname(parsed.Hostname()) {
		return "", errors.New("CAS Server 必须使用 HTTPS（localhost 开发环境除外）")
	}
	parsed.Path = strings.TrimRight(parsed.Path, "/")
	return parsed.String(), nil
}

func NormalizeExternalCallbackURL(raw string) (string, error) {
	normalized, _, err := NormalizeExternalURL(raw, true)
	if err != nil {
		return "", fmt.Errorf("回调 URL 无效: %w", err)
	}
	return normalized, nil
}

func NormalizeOIDCCallbackURL(raw string) (string, error) {
	return NormalizeConfiguredCallbackURL(raw, "/api/auth/oidc/callback", "state", "code", "error", "error_description")
}

func NormalizeCASCallbackURL(raw string) (string, error) {
	return NormalizeConfiguredCallbackURL(raw, "/api/auth/cas/callback", "state", "ticket")
}

func NormalizeConfiguredCallbackURL(raw, callbackPath string, reservedQueryKeys ...string) (string, error) {
	normalized, err := NormalizeExternalCallbackURL(raw)
	if err != nil || normalized == "" {
		return normalized, err
	}
	expected, err := CallbackURLFromApp(callbackPath)
	if err != nil {
		return "", err
	}
	configuredURL, _ := url.Parse(normalized)
	expectedURL, _ := url.Parse(expected)
	if !strings.EqualFold(configuredURL.Scheme, expectedURL.Scheme) ||
		!strings.EqualFold(configuredURL.Host, expectedURL.Host) || configuredURL.Path != expectedURL.Path {
		return "", fmt.Errorf("回调 URL 必须与 APP_URL 同源且路径为 %s", expectedURL.Path)
	}
	query := configuredURL.Query()
	for _, key := range reservedQueryKeys {
		if _, exists := query[key]; exists {
			return "", fmt.Errorf("回调 URL 不得预置协议参数 %s", key)
		}
	}
	return configuredURL.String(), nil
}

func NormalizeExternalURL(raw string, allowQuery bool) (string, *url.URL, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", nil, nil
	}
	if len(raw) > 4096 {
		return "", nil, errors.New("URL 长度超过限制")
	}
	parsed, err := url.Parse(raw)
	if err != nil || !parsed.IsAbs() || parsed.Host == "" {
		return "", nil, errors.New("必须是完整的绝对 URL")
	}
	parsed.Scheme = strings.ToLower(parsed.Scheme)
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", nil, errors.New("仅支持 HTTP/HTTPS")
	}
	if parsed.User != nil || parsed.Fragment != "" || (!allowQuery && (parsed.RawQuery != "" || parsed.ForceQuery)) {
		return "", nil, errors.New("URL 不得包含用户信息、片段或非法查询参数")
	}
	return parsed.String(), parsed, nil
}

func NormalizeOIDCScopes(raw string) (string, error) {
	fields := strings.Fields(raw)
	if len(fields) == 0 {
		fields = []string{"openid", "profile", "email"}
	}
	seen := make(map[string]bool, len(fields))
	normalized := make([]string, 0, len(fields))
	for _, scope := range fields {
		if len(scope) > 128 || strings.ContainsAny(scope, `"\\`) {
			return "", errors.New("OIDC Scope 格式不正确")
		}
		if !seen[scope] {
			seen[scope] = true
			normalized = append(normalized, scope)
		}
	}
	if !seen["openid"] {
		return "", errors.New("OIDC Scopes 必须包含 openid")
	}
	return strings.Join(normalized, " "), nil
}

func OIDCCallbackURL(setting models.Settings) (string, error) {
	if strings.TrimSpace(setting.OIDCRedirectURL) != "" {
		return NormalizeOIDCCallbackURL(setting.OIDCRedirectURL)
	}
	return CallbackURLFromApp("/api/auth/oidc/callback")
}

func CASCallbackURL(setting models.Settings) (string, error) {
	if strings.TrimSpace(setting.CASServiceURL) != "" {
		return NormalizeCASCallbackURL(setting.CASServiceURL)
	}
	return CallbackURLFromApp("/api/auth/cas/callback")
}

func CallbackURLFromApp(callbackPath string) (string, error) {
	if config.App == nil {
		return "", errors.New("APP_URL 未配置")
	}
	_, parsed, err := NormalizeExternalURL(config.App.AppURL, false)
	if err != nil || parsed == nil {
		return "", errors.New("APP_URL 无效")
	}
	parsed.Path = strings.TrimRight(parsed.Path, "/") + callbackPath
	parsed.RawPath = ""
	return parsed.String(), nil
}

func OidcSettingsComplete(setting models.Settings) bool {
	issuer, issuerErr := NormalizeOIDCIssuer(setting.OIDCIssuer)
	callback, callbackErr := OIDCCallbackURL(setting)
	scopes, scopeErr := NormalizeOIDCScopes(setting.OIDCScopes)
	claim := strings.TrimSpace(setting.OIDCUsernameClaim)
	clientID := strings.TrimSpace(setting.OIDCClientID)
	clientSecret := strings.TrimSpace(setting.OIDCClientSecret)
	return issuerErr == nil && issuer != "" && callbackErr == nil && callback != "" && scopeErr == nil && scopes != "" &&
		clientID != "" && len(clientID) <= 512 && clientSecret != "" && len(clientSecret) <= 4096 && OIDCClaimNameRegex.MatchString(claim)
}

func OIDCSettingsReady(setting models.Settings) bool {
	return setting.OIDCEnable && OidcSettingsComplete(setting)
}

func CasSettingsComplete(setting models.Settings) bool {
	server, serverErr := NormalizeCASServerURL(setting.CASServerURL)
	callback, callbackErr := CASCallbackURL(setting)
	return serverErr == nil && server != "" && callbackErr == nil && callback != ""
}

func CASSettingsReady(setting models.Settings) bool {
	return setting.CASEnable && CasSettingsComplete(setting)
}

func ExternalLoginDisplayName(value, fallback string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback
	}
	return TruncateUTF8(value, 40)
}

func IsLoopbackHostname(host string) bool {
	host = strings.ToLower(strings.TrimSpace(host))
	return host == "localhost" || host == "127.0.0.1" || host == "::1"
}

func AddQueryValue(rawURL, key, value string) (string, error) {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return "", err
	}
	query := parsed.Query()
	query.Set(key, value)
	parsed.RawQuery = query.Encode()
	return parsed.String(), nil
}

func TruncateASCII(value string, max int) string {
	if max <= 0 {
		return ""
	}
	if len(value) <= max {
		return value
	}
	return value[:max]
}

func TruncateUTF8(value string, max int) string {
	runes := []rune(strings.TrimSpace(value))
	if len(runes) <= max {
		return string(runes)
	}
	return string(runes[:max])
}
