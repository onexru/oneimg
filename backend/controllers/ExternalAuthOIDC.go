package controllers

import (
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/gin-gonic/gin"
	"golang.org/x/oauth2"
	"net/http"
	"oneimg/backend/models"
	settingsutil "oneimg/backend/utils/settings"
	"strings"
	"time"
)

// StartOIDCLogin 启动 Authorization Code + PKCE 流程。
func StartOIDCLogin(c *gin.Context) {
	setting, err := settingsutil.GetSettingsWithSecrets("oidc_client_secret")
	if err != nil || !oidcSettingsReady(setting) {
		externalAuthFailure(c, "oidc", "not_configured", err)
		return
	}

	issuer, _ := normalizeOIDCIssuer(setting.OIDCIssuer)
	scopes, _ := normalizeOIDCScopes(setting.OIDCScopes)
	redirectURL, err := oidcCallbackURL(setting)
	if err != nil {
		externalAuthFailure(c, "oidc", "not_configured", err)
		return
	}

	ctx := externalOIDCContext(c.Request.Context())
	provider, err := oidc.NewProvider(ctx, issuer)
	if err != nil {
		externalAuthFailure(c, "oidc", "provider_error", fmt.Errorf("OIDC discovery 失败: %w", err))
		return
	}
	if err := validateOIDCProviderEndpoints(provider); err != nil {
		externalAuthFailure(c, "oidc", "provider_error", err)
		return
	}

	state, err := randomURLToken(32)
	if err != nil {
		externalAuthFailure(c, "oidc", "internal_error", err)
		return
	}
	nonce, err := randomURLToken(32)
	if err != nil {
		externalAuthFailure(c, "oidc", "internal_error", err)
		return
	}
	verifier := oauth2.GenerateVerifier()

	flow := models.ExternalAuthFlow{
		StateHash:    hashExternalAuthState(state),
		Provider:     "oidc",
		Issuer:       issuer,
		ClientID:     strings.TrimSpace(setting.OIDCClientID),
		Nonce:        nonce,
		CodeVerifier: verifier,
		CallbackURL:  redirectURL,
		ExpiresAt:    time.Now().Add(externalAuthFlowTTL),
	}
	if err := saveExternalAuthFlow(c, state, &flow); err != nil {
		externalAuthFailure(c, "oidc", "internal_error", err)
		return
	}

	oauthConfig := oauth2.Config{
		ClientID:     flow.ClientID,
		ClientSecret: setting.OIDCClientSecret,
		Endpoint:     provider.Endpoint(),
		RedirectURL:  flow.CallbackURL,
		Scopes:       strings.Fields(scopes),
	}
	authURL := oauthConfig.AuthCodeURL(
		state,
		oidc.Nonce(nonce),
		oauth2.S256ChallengeOption(verifier),
	)
	c.Redirect(http.StatusFound, authURL)
}

// OIDCCallback 验证 state、PKCE、ID Token 签名/claims 以及 nonce。
func OIDCCallback(c *gin.Context) {
	state, stateOK := singleQueryValue(c, "state", 512)
	code, codeOK := singleQueryValue(c, "code", 16*1024)
	providerError, providerErrorOK := singleQueryValue(c, "error", 1024)
	// 访问日志已跳过该路由；同时尽早丢弃 code，避免后续中间件误用。
	c.Request.URL.RawQuery = ""

	if !stateOK {
		externalAuthFailure(c, "oidc", "invalid_state", nil)
		return
	}
	flow, err := consumeExternalAuthFlow(c, state, "oidc")
	if err != nil {
		externalAuthFailure(c, "oidc", "invalid_state", err)
		return
	}
	if providerErrorOK && providerError != "" {
		code := "provider_error"
		if providerError == "access_denied" {
			code = "access_denied"
		}
		externalAuthFailure(c, "oidc", code, errors.New("OIDC provider returned an authorization error"))
		return
	}
	if !codeOK || strings.TrimSpace(code) == "" {
		externalAuthFailure(c, "oidc", "missing_code", nil)
		return
	}

	setting, err := settingsutil.GetSettingsWithSecrets("oidc_client_secret")
	if err != nil || !setting.OIDCEnable || strings.TrimSpace(setting.OIDCClientSecret) == "" {
		externalAuthFailure(c, "oidc", "not_configured", err)
		return
	}
	currentIssuer, issuerErr := normalizeOIDCIssuer(setting.OIDCIssuer)
	if issuerErr != nil || currentIssuer != flow.Issuer || strings.TrimSpace(setting.OIDCClientID) != flow.ClientID {
		externalAuthFailure(c, "oidc", "not_configured", errors.New("OIDC 配置在登录过程中已变更"))
		return
	}

	ctx := externalOIDCContext(c.Request.Context())
	provider, err := oidc.NewProvider(ctx, flow.Issuer)
	if err != nil {
		externalAuthFailure(c, "oidc", "provider_error", fmt.Errorf("OIDC discovery 失败: %w", err))
		return
	}
	if err := validateOIDCProviderEndpoints(provider); err != nil {
		externalAuthFailure(c, "oidc", "provider_error", err)
		return
	}
	scopes, _ := normalizeOIDCScopes(setting.OIDCScopes)
	oauthConfig := oauth2.Config{
		ClientID:     flow.ClientID,
		ClientSecret: setting.OIDCClientSecret,
		Endpoint:     provider.Endpoint(),
		RedirectURL:  flow.CallbackURL,
		Scopes:       strings.Fields(scopes),
	}
	token, err := oauthConfig.Exchange(ctx, code, oauth2.VerifierOption(flow.CodeVerifier))
	if err != nil {
		externalAuthFailure(c, "oidc", "token_exchange_failed", fmt.Errorf("OIDC code 交换失败: %w", err))
		return
	}
	rawIDToken, ok := token.Extra("id_token").(string)
	if !ok || strings.TrimSpace(rawIDToken) == "" {
		externalAuthFailure(c, "oidc", "token_exchange_failed", errors.New("OIDC token 响应缺少 id_token"))
		return
	}
	idToken, err := provider.Verifier(&oidc.Config{ClientID: flow.ClientID}).Verify(ctx, rawIDToken)
	if err != nil {
		externalAuthFailure(c, "oidc", "authentication_failed", fmt.Errorf("OIDC ID Token 校验失败: %w", err))
		return
	}
	claims, err := validateOIDCIDToken(idToken, flow, token.AccessToken)
	if err != nil {
		externalAuthFailure(c, "oidc", "authentication_failed", err)
		return
	}

	usernameClaim := strings.TrimSpace(setting.OIDCUsernameClaim)
	username := oidcStringClaim(claims, usernameClaim)
	if username == "" {
		username = oidcStringClaim(claims, "preferred_username")
	}
	email := oidcStringClaim(claims, "email")
	displayName := oidcStringClaim(claims, "name")
	if username == "" {
		username = email
	}
	if username == "" {
		username = displayName
	}
	if username == "" {
		username = idToken.Subject
	}

	user, err := resolveExternalCallbackUser(externalIdentityProfile{
		Provider:    "oidc",
		Issuer:      flow.Issuer,
		Subject:     idToken.Subject,
		Username:    username,
		Email:       email,
		DisplayName: displayName,
	}, username, setting.OIDCSuperAdminUsername, setting.OIDCAutoProvision)
	if err != nil {
		externalAuthFailure(c, "oidc", "account_error", err)
		return
	}
	if _, err := saveUserSession(c, user); err != nil {
		externalAuthFailure(c, "oidc", "internal_error", err)
		return
	}
	externalAuthSuccess(c, "oidc")
}

func validateOIDCIDToken(idToken *oidc.IDToken, flow models.ExternalAuthFlow, accessToken string) (map[string]json.RawMessage, error) {
	if idToken == nil || strings.TrimSpace(idToken.Subject) == "" || len(idToken.Subject) > 2048 {
		return nil, errors.New("OIDC ID Token 缺少有效 sub")
	}
	if subtle.ConstantTimeCompare([]byte(idToken.Nonce), []byte(flow.Nonce)) != 1 {
		return nil, errors.New("OIDC nonce 校验失败")
	}
	if idToken.IssuedAt.IsZero() || idToken.IssuedAt.After(time.Now().Add(5*time.Minute)) {
		return nil, errors.New("OIDC iat 无效")
	}
	if idToken.AccessTokenHash != "" {
		if strings.TrimSpace(accessToken) == "" {
			return nil, errors.New("OIDC at_hash 存在但缺少 access token")
		}
		if err := idToken.VerifyAccessToken(accessToken); err != nil {
			return nil, fmt.Errorf("OIDC at_hash 校验失败: %w", err)
		}
	}
	var claims map[string]json.RawMessage
	if err := idToken.Claims(&claims); err != nil {
		return nil, fmt.Errorf("OIDC claims 解析失败: %w", err)
	}
	azp := oidcStringClaim(claims, "azp")
	if len(idToken.Audience) > 1 && azp == "" {
		return nil, errors.New("OIDC 多 audience Token 缺少 azp")
	}
	if azp != "" && azp != flow.ClientID {
		return nil, errors.New("OIDC azp 校验失败")
	}
	if raw, ok := claims["nbf"]; ok {
		var nbf json.Number
		if err := json.Unmarshal(raw, &nbf); err != nil {
			return nil, errors.New("OIDC nbf 格式无效")
		}
		nbfSeconds, err := nbf.Int64()
		if err != nil || time.Unix(nbfSeconds, 0).After(time.Now().Add(time.Minute)) {
			return nil, errors.New("OIDC Token 尚未生效")
		}
	}
	return claims, nil
}

func oidcStringClaim(claims map[string]json.RawMessage, name string) string {
	if name == "" {
		return ""
	}
	raw, ok := claims[name]
	if !ok {
		return ""
	}
	var value string
	if err := json.Unmarshal(raw, &value); err != nil {
		return ""
	}
	return strings.TrimSpace(value)
}

func validateOIDCProviderEndpoints(provider *oidc.Provider) error {
	if provider == nil {
		return errors.New("OIDC provider 为空")
	}
	endpoint := provider.Endpoint()
	var metadata struct {
		JWKSURL string `json:"jwks_uri"`
	}
	if err := provider.Claims(&metadata); err != nil {
		return fmt.Errorf("OIDC discovery metadata 解析失败: %w", err)
	}
	for name, raw := range map[string]string{
		"authorization_endpoint": endpoint.AuthURL,
		"token_endpoint":         endpoint.TokenURL,
		"jwks_uri":               metadata.JWKSURL,
	} {
		_, parsed, err := normalizeExternalURL(raw, true)
		if err != nil || parsed == nil {
			return fmt.Errorf("OIDC %s 无效", name)
		}
		if parsed.Scheme != "https" && !isLoopbackHostname(parsed.Hostname()) {
			return fmt.Errorf("OIDC %s 必须使用 HTTPS", name)
		}
	}
	return nil
}
