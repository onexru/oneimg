package controllers

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
	"golang.org/x/oauth2"
	"gorm.io/gorm"

	"oneimg/backend/config"
	"oneimg/backend/database"
	"oneimg/backend/models"
	settingsutil "oneimg/backend/utils/settings"
)

const (
	externalAuthFlowTTL      = 10 * time.Minute
	externalAuthCookiePrefix = "oneimg-auth-flow-"
	externalAuthMaxHTTPBody  = int64(2 << 20)
	casMaxResponseBody       = int64(1 << 20)
	casXMLNamespace          = "http://www.yale.edu/tp/cas"
)

var oidcClaimNameRegex = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_.:-]{0,127}$`)

var externalAuthStartLimit = struct {
	sync.Mutex
	WindowStart time.Time
	GlobalCount int
	Clients     map[string]externalAuthRateEntry
}{Clients: make(map[string]externalAuthRateEntry)}

type externalAuthRateEntry struct {
	WindowStart time.Time
	Count       int
}

type externalIdentityProfile struct {
	Provider    string
	Issuer      string
	Subject     string
	Username    string
	Email       string
	DisplayName string
}

// StartOIDCLogin 启动 Authorization Code + PKCE 流程。
func StartOIDCLogin(c *gin.Context) {
	if !allowExternalAuthStart(c, "oidc") {
		externalAuthFailure(c, "oidc", "provider_error", errors.New("外部登录请求过于频繁"))
		return
	}
	setting, err := settingsutil.GetSettings()
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

	setting, err := settingsutil.GetSettings()
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

// StartCASLogin 以含 state 的精确 service URL 启动 CAS 登录。
func StartCASLogin(c *gin.Context) {
	if !allowExternalAuthStart(c, "cas") {
		externalAuthFailure(c, "cas", "provider_error", errors.New("外部登录请求过于频繁"))
		return
	}
	setting, err := settingsutil.GetSettings()
	if err != nil || !casSettingsReady(setting) {
		externalAuthFailure(c, "cas", "not_configured", err)
		return
	}
	serverURL, _ := normalizeCASServerURL(setting.CASServerURL)
	serviceBase, err := casCallbackURL(setting)
	if err != nil {
		externalAuthFailure(c, "cas", "not_configured", err)
		return
	}
	state, err := randomURLToken(32)
	if err != nil {
		externalAuthFailure(c, "cas", "internal_error", err)
		return
	}
	serviceURL, err := addQueryValue(serviceBase, "state", state)
	if err != nil {
		externalAuthFailure(c, "cas", "internal_error", err)
		return
	}
	flow := models.ExternalAuthFlow{
		StateHash:  hashExternalAuthState(state),
		Provider:   "cas",
		Issuer:     serverURL,
		ServiceURL: serviceURL,
		ExpiresAt:  time.Now().Add(externalAuthFlowTTL),
	}
	if err := saveExternalAuthFlow(c, state, &flow); err != nil {
		externalAuthFailure(c, "cas", "internal_error", err)
		return
	}
	loginURL, err := casProtocolURL(serverURL, "/login", url.Values{"service": []string{serviceURL}})
	if err != nil {
		externalAuthFailure(c, "cas", "internal_error", err)
		return
	}
	c.Redirect(http.StatusFound, loginURL)
}

// CASCallback 固定使用 CAS 3.0 /p3/serviceValidate 并解析带命名空间的 XML。
func CASCallback(c *gin.Context) {
	state, stateOK := singleQueryValue(c, "state", 512)
	ticket, ticketOK := singleQueryValue(c, "ticket", 4096)
	c.Request.URL.RawQuery = ""
	if !stateOK {
		externalAuthFailure(c, "cas", "invalid_state", nil)
		return
	}
	flow, err := consumeExternalAuthFlow(c, state, "cas")
	if err != nil {
		externalAuthFailure(c, "cas", "invalid_state", err)
		return
	}
	if !ticketOK || strings.TrimSpace(ticket) == "" {
		externalAuthFailure(c, "cas", "missing_ticket", nil)
		return
	}

	setting, err := settingsutil.GetSettings()
	if err != nil || !setting.CASEnable {
		externalAuthFailure(c, "cas", "not_configured", err)
		return
	}
	currentServer, serverErr := normalizeCASServerURL(setting.CASServerURL)
	if serverErr != nil || currentServer != flow.Issuer {
		externalAuthFailure(c, "cas", "not_configured", errors.New("CAS 配置在登录过程中已变更"))
		return
	}
	casResult, err := validateCAS3Ticket(c.Request.Context(), flow.Issuer, flow.ServiceURL, ticket)
	if err != nil {
		externalAuthFailure(c, "cas", "invalid_ticket", err)
		return
	}

	username := casResult.User
	email := firstCASAttribute(casResult.Attributes, "email", "mail")
	displayName := firstCASAttribute(casResult.Attributes, "displayName", "name", "cn")
	preferredUsername := firstCASAttribute(casResult.Attributes, "preferred_username", "username", "uid")
	if preferredUsername != "" {
		username = preferredUsername
	}
	user, err := resolveExternalCallbackUser(externalIdentityProfile{
		Provider:    "cas",
		Issuer:      flow.Issuer,
		Subject:     casResult.User,
		Username:    username,
		Email:       email,
		DisplayName: displayName,
	}, casResult.User, setting.CASSuperAdminUsername, setting.CASAutoProvision)
	if err != nil {
		externalAuthFailure(c, "cas", "account_error", err)
		return
	}
	if _, err := saveUserSession(c, user); err != nil {
		externalAuthFailure(c, "cas", "internal_error", err)
		return
	}
	externalAuthSuccess(c, "cas")
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

func resolveOrProvisionExternalIdentity(profile externalIdentityProfile) (*models.User, error) {
	return resolveExternalIdentity(profile, true)
}

func resolveExternalCallbackUser(profile externalIdentityProfile, callbackUsername, superAdminUsername string, allowProvision bool) (*models.User, error) {
	callbackUsername = strings.TrimSpace(callbackUsername)
	superAdminUsername = strings.TrimSpace(superAdminUsername)
	if superAdminUsername != "" && len(callbackUsername) == len(superAdminUsername) &&
		subtle.ConstantTimeCompare([]byte(callbackUsername), []byte(superAdminUsername)) == 1 {
		db := database.GetDB()
		if db == nil {
			return nil, errors.New("数据库未初始化")
		}
		var superAdmin models.User
		if err := db.DB.Where("id = ? AND role = ?", models.SuperAdminID, models.RoleAdmin).First(&superAdmin).Error; err != nil {
			return nil, errors.New("本地超级管理员账户不存在或角色无效")
		}
		return &superAdmin, nil
	}
	return resolveExternalIdentity(profile, allowProvision)
}

func resolveExternalIdentity(profile externalIdentityProfile, allowProvision bool) (*models.User, error) {
	profile.Provider = strings.ToLower(strings.TrimSpace(profile.Provider))
	profile.Issuer = strings.TrimSpace(profile.Issuer)
	if profile.Provider == "" || profile.Issuer == "" || len(profile.Issuer) > 2048 || profile.Subject == "" || len(profile.Subject) > 2048 {
		return nil, errors.New("外部身份信息不完整")
	}
	identityKey := externalIdentityKey(profile.Provider, profile.Issuer, profile.Subject)
	db := database.GetDB()
	if db == nil {
		return nil, errors.New("数据库未初始化")
	}

	var resolved models.User
	err := db.DB.Transaction(func(tx *gorm.DB) error {
		var identity models.ExternalIdentity
		err := tx.Where("identity_key = ?", identityKey).First(&identity).Error
		if err == nil {
			if identity.Provider != profile.Provider || identity.Issuer != profile.Issuer || identity.Subject != profile.Subject {
				return errors.New("外部身份键冲突")
			}
			if identity.Disabled || identity.UserID <= 0 {
				return errors.New("该外部身份已被禁用")
			}
			if err := tx.First(&resolved, identity.UserID).Error; err != nil {
				return errors.New("外部身份绑定的本地用户不存在")
			}
			return tx.Model(&identity).Updates(map[string]any{
				"email":         truncateUTF8(profile.Email, 320),
				"display_name":  truncateUTF8(profile.DisplayName, 255),
				"last_login_at": time.Now(),
			}).Error
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		if !allowProvision {
			return errors.New("管理员未允许外部身份自动创建用户")
		}

		username, err := availableExternalUsername(tx, profile, identityKey)
		if err != nil {
			return err
		}
		randomPassword, err := randomURLToken(32)
		if err != nil {
			return err
		}
		hashedPassword, err := bcrypt.GenerateFromPassword([]byte(randomPassword), bcrypt.DefaultCost)
		if err != nil {
			return err
		}
		resolved = models.User{
			Role:       models.RoleUser,
			Username:   username,
			Password:   string(hashedPassword),
			Permission: models.Permission{Buckets: []int{}},
		}
		if err := tx.Create(&resolved).Error; err != nil {
			return err
		}
		identity = models.ExternalIdentity{
			UserID:      resolved.ID,
			Provider:    profile.Provider,
			Issuer:      profile.Issuer,
			Subject:     profile.Subject,
			IdentityKey: identityKey,
			Email:       truncateUTF8(profile.Email, 320),
			DisplayName: truncateUTF8(profile.DisplayName, 255),
			LastLoginAt: time.Now(),
		}
		return tx.Create(&identity).Error
	})
	if err == nil {
		return &resolved, nil
	}

	// 并发首次登录时，唯一索引失败的事务会回滚，再读取已成功的绑定。
	var identity models.ExternalIdentity
	if lookupErr := db.DB.Where("identity_key = ?", identityKey).First(&identity).Error; lookupErr == nil {
		if identity.Disabled || identity.UserID <= 0 {
			return nil, errors.New("该外部身份已被禁用")
		}
		if userErr := db.DB.First(&resolved, identity.UserID).Error; userErr == nil {
			return &resolved, nil
		}
	}
	return nil, fmt.Errorf("创建外部登录用户失败: %w", err)
}

func availableExternalUsername(tx *gorm.DB, profile externalIdentityProfile, identityKey string) (string, error) {
	base := sanitizeExternalUsername(profile.Username)
	if base == "" {
		base = sanitizeExternalUsername(profile.DisplayName)
	}
	if base == "" {
		base = profile.Provider + "_user"
	}
	base = safeExternalUsername(base, profile.Provider)
	if !externalUsernameExists(tx, base) {
		return base, nil
	}
	suffix := "_" + profile.Provider + "_" + identityKey[:8]
	base = truncateASCII(base, 50-len(suffix)) + suffix
	if !externalUsernameExists(tx, base) {
		return base, nil
	}
	for i := 2; i <= 100; i++ {
		numberedSuffix := fmt.Sprintf("%s_%d", suffix, i)
		candidate := truncateASCII(sanitizeExternalUsername(profile.Username), 50-len(numberedSuffix)) + numberedSuffix
		candidate = safeExternalUsername(candidate, profile.Provider)
		if !externalUsernameExists(tx, candidate) {
			return candidate, nil
		}
	}
	return "", errors.New("无法生成唯一的本地用户名")
}

func sanitizeExternalUsername(value string) string {
	value = strings.TrimSpace(value)
	var b strings.Builder
	lastUnderscore := false
	for _, r := range value {
		allowed := r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || strings.ContainsRune("._@-", r)
		if allowed {
			b.WriteRune(r)
			lastUnderscore = false
		} else if !lastUnderscore {
			b.WriteByte('_')
			lastUnderscore = true
		}
	}
	return truncateASCII(strings.Trim(b.String(), "._-@"), 50)
}

func safeExternalUsername(value, provider string) string {
	value = truncateASCII(value, 50)
	lower := strings.ToLower(value)
	if value == "" || lower == "guest" || strings.HasPrefix(lower, "guest_") {
		value = provider + "_user"
	}
	if _, err := uuid.Parse(value); err == nil {
		value = provider + "_" + truncateASCII(strings.ReplaceAll(value, "-", ""), 43)
	}
	return truncateASCII(value, 50)
}

func externalUsernameExists(tx *gorm.DB, username string) bool {
	var count int64
	if err := tx.Model(&models.User{}).Where("LOWER(username) = LOWER(?)", username).Count(&count).Error; err != nil {
		return true
	}
	return count > 0
}

func externalIdentityKey(provider, issuer, subject string) string {
	sum := sha256.Sum256([]byte(provider + "\x00" + issuer + "\x00" + subject))
	return hex.EncodeToString(sum[:])
}

func saveExternalAuthFlow(c *gin.Context, state string, flow *models.ExternalAuthFlow) error {
	db := database.GetDB()
	if db == nil {
		return errors.New("数据库未初始化")
	}
	if flow.StateHash != hashExternalAuthState(state) {
		return errors.New("登录事务 state 不匹配")
	}
	_ = db.DB.Where("expires_at < ?", time.Now()).Delete(&models.ExternalAuthFlow{}).Error
	if err := db.DB.Create(flow).Error; err != nil {
		return fmt.Errorf("保存登录事务失败: %w", err)
	}
	setExternalAuthCookie(c, state, externalAuthFlowTTL)
	return nil
}

func consumeExternalAuthFlow(c *gin.Context, state, provider string) (models.ExternalAuthFlow, error) {
	var flow models.ExternalAuthFlow
	if len(state) < 32 || len(state) > 512 {
		return flow, errors.New("state 长度无效")
	}
	cookieName := externalAuthCookieName(state)
	cookieValue, err := c.Cookie(cookieName)
	if err != nil || subtle.ConstantTimeCompare([]byte(cookieValue), []byte(externalAuthCookieSignature(state))) != 1 {
		return flow, errors.New("登录事务 Cookie 校验失败")
	}
	clearExternalAuthCookie(c, state)

	db := database.GetDB()
	if db == nil {
		return flow, errors.New("数据库未初始化")
	}
	stateHash := hashExternalAuthState(state)
	err = db.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("state_hash = ?", stateHash).First(&flow).Error; err != nil {
			return err
		}
		result := tx.Where("state_hash = ?", stateHash).Delete(&models.ExternalAuthFlow{})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return errors.New("登录事务已被消费")
		}
		return nil
	})
	if err != nil {
		return flow, err
	}
	if flow.Provider != provider || time.Now().After(flow.ExpiresAt) {
		return models.ExternalAuthFlow{}, errors.New("登录事务已过期或类型不匹配")
	}
	return flow, nil
}

func hashExternalAuthState(state string) string {
	sum := sha256.Sum256([]byte(state))
	return hex.EncodeToString(sum[:])
}

func externalAuthCookieName(state string) string {
	hash := hashExternalAuthState(state)
	return externalAuthCookiePrefix + hash[:16]
}

func externalAuthCookieSignature(state string) string {
	secret := ""
	if config.App != nil {
		secret = config.App.SessionSecret
	}
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(state))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func setExternalAuthCookie(c *gin.Context, state string, ttl time.Duration) {
	secure := config.App != nil && strings.HasPrefix(strings.ToLower(strings.TrimSpace(config.App.AppURL)), "https://")
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     externalAuthCookieName(state),
		Value:    externalAuthCookieSignature(state),
		Path:     "/api/auth",
		MaxAge:   int(ttl.Seconds()),
		Expires:  time.Now().Add(ttl),
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
	})
}

func clearExternalAuthCookie(c *gin.Context, state string) {
	secure := config.App != nil && strings.HasPrefix(strings.ToLower(strings.TrimSpace(config.App.AppURL)), "https://")
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     externalAuthCookieName(state),
		Value:    "",
		Path:     "/api/auth",
		MaxAge:   -1,
		Expires:  time.Unix(1, 0),
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
	})
}

func randomURLToken(size int) (string, error) {
	value := make([]byte, size)
	if _, err := rand.Read(value); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(value), nil
}

func singleQueryValue(c *gin.Context, key string, maxLength int) (string, bool) {
	values, ok := c.Request.URL.Query()[key]
	if !ok || len(values) != 1 || len(values[0]) > maxLength {
		return "", false
	}
	return values[0], true
}

func externalAuthSuccess(c *gin.Context, provider string) {
	query := url.Values{"external_login": []string{"success"}, "provider": []string{provider}}
	c.Redirect(http.StatusFound, "/login?"+query.Encode())
}

func externalAuthFailure(c *gin.Context, provider, code string, internalErr error) {
	if internalErr != nil {
		log.Printf("[%s 登录] %s: %s", strings.ToUpper(provider), code, truncateUTF8(internalErr.Error(), 500))
	}
	allowed := map[string]bool{
		"access_denied": true, "invalid_state": true, "missing_code": true,
		"missing_ticket": true, "invalid_ticket": true, "token_exchange_failed": true,
		"user_info_failed": true, "account_error": true, "not_configured": true,
		"provider_error": true, "authentication_failed": true, "callback_failed": true,
		"internal_error": true,
	}
	if !allowed[code] {
		code = "internal_error"
	}
	query := url.Values{"external_login": []string{"error"}, "error_code": []string{code}}
	c.Redirect(http.StatusFound, "/login?"+query.Encode())
}

func normalizeOIDCIssuer(raw string) (string, error) {
	normalized, parsed, err := normalizeExternalURL(raw, false)
	if err != nil {
		return "", fmt.Errorf("OIDC Issuer URL 无效: %w", err)
	}
	if normalized == "" {
		return "", nil
	}
	if parsed.Scheme != "https" && !isLoopbackHostname(parsed.Hostname()) {
		return "", errors.New("OIDC Issuer 必须使用 HTTPS（localhost 开发环境除外）")
	}
	return parsed.String(), nil
}

func normalizeCASServerURL(raw string) (string, error) {
	normalized, parsed, err := normalizeExternalURL(raw, false)
	if err != nil {
		return "", fmt.Errorf("CAS Server URL 无效: %w", err)
	}
	if normalized == "" {
		return "", nil
	}
	if parsed.Scheme != "https" && !isLoopbackHostname(parsed.Hostname()) {
		return "", errors.New("CAS Server 必须使用 HTTPS（localhost 开发环境除外）")
	}
	parsed.Path = strings.TrimRight(parsed.Path, "/")
	return parsed.String(), nil
}

func normalizeExternalCallbackURL(raw string) (string, error) {
	normalized, _, err := normalizeExternalURL(raw, true)
	if err != nil {
		return "", fmt.Errorf("回调 URL 无效: %w", err)
	}
	return normalized, nil
}

func normalizeOIDCCallbackURL(raw string) (string, error) {
	return normalizeConfiguredCallbackURL(raw, "/api/auth/oidc/callback", "state", "code", "error", "error_description")
}

func normalizeCASCallbackURL(raw string) (string, error) {
	return normalizeConfiguredCallbackURL(raw, "/api/auth/cas/callback", "state", "ticket")
}

func normalizeConfiguredCallbackURL(raw, callbackPath string, reservedQueryKeys ...string) (string, error) {
	normalized, err := normalizeExternalCallbackURL(raw)
	if err != nil || normalized == "" {
		return normalized, err
	}
	expected, err := callbackURLFromApp(callbackPath)
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

func normalizeExternalURL(raw string, allowQuery bool) (string, *url.URL, error) {
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

func normalizeOIDCScopes(raw string) (string, error) {
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

func oidcCallbackURL(setting models.Settings) (string, error) {
	if strings.TrimSpace(setting.OIDCRedirectURL) != "" {
		return normalizeOIDCCallbackURL(setting.OIDCRedirectURL)
	}
	return callbackURLFromApp("/api/auth/oidc/callback")
}

func casCallbackURL(setting models.Settings) (string, error) {
	if strings.TrimSpace(setting.CASServiceURL) != "" {
		return normalizeCASCallbackURL(setting.CASServiceURL)
	}
	return callbackURLFromApp("/api/auth/cas/callback")
}

func callbackURLFromApp(callbackPath string) (string, error) {
	if config.App == nil {
		return "", errors.New("APP_URL 未配置")
	}
	_, parsed, err := normalizeExternalURL(config.App.AppURL, false)
	if err != nil || parsed == nil {
		return "", errors.New("APP_URL 无效")
	}
	parsed.Path = strings.TrimRight(parsed.Path, "/") + callbackPath
	parsed.RawPath = ""
	return parsed.String(), nil
}

func oidcSettingsComplete(setting models.Settings) bool {
	issuer, issuerErr := normalizeOIDCIssuer(setting.OIDCIssuer)
	callback, callbackErr := oidcCallbackURL(setting)
	scopes, scopeErr := normalizeOIDCScopes(setting.OIDCScopes)
	claim := strings.TrimSpace(setting.OIDCUsernameClaim)
	clientID := strings.TrimSpace(setting.OIDCClientID)
	clientSecret := strings.TrimSpace(setting.OIDCClientSecret)
	return issuerErr == nil && issuer != "" && callbackErr == nil && callback != "" && scopeErr == nil && scopes != "" &&
		clientID != "" && len(clientID) <= 512 && clientSecret != "" && len(clientSecret) <= 4096 && oidcClaimNameRegex.MatchString(claim)
}

func oidcSettingsReady(setting models.Settings) bool {
	return setting.OIDCEnable && oidcSettingsComplete(setting)
}

func casSettingsComplete(setting models.Settings) bool {
	server, serverErr := normalizeCASServerURL(setting.CASServerURL)
	callback, callbackErr := casCallbackURL(setting)
	return serverErr == nil && server != "" && callbackErr == nil && callback != ""
}

func casSettingsReady(setting models.Settings) bool {
	return setting.CASEnable && casSettingsComplete(setting)
}

func externalLoginDisplayName(value, fallback string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback
	}
	return truncateUTF8(value, 40)
}

func isLoopbackHostname(host string) bool {
	host = strings.ToLower(strings.TrimSpace(host))
	return host == "localhost" || host == "127.0.0.1" || host == "::1"
}

func allowExternalAuthStart(c *gin.Context, provider string) bool {
	now := time.Now()
	clientHost, _, err := net.SplitHostPort(strings.TrimSpace(c.Request.RemoteAddr))
	if err != nil || clientHost == "" {
		clientHost = strings.TrimSpace(c.Request.RemoteAddr)
	}
	if clientHost == "" {
		clientHost = "unknown"
	}
	key := provider + "\x00" + clientHost

	externalAuthStartLimit.Lock()
	defer externalAuthStartLimit.Unlock()
	if externalAuthStartLimit.WindowStart.IsZero() || now.Sub(externalAuthStartLimit.WindowStart) >= time.Minute {
		externalAuthStartLimit.WindowStart = now
		externalAuthStartLimit.GlobalCount = 0
	}
	if externalAuthStartLimit.GlobalCount >= 600 {
		return false
	}
	entry := externalAuthStartLimit.Clients[key]
	if entry.WindowStart.IsZero() || now.Sub(entry.WindowStart) >= time.Minute {
		entry = externalAuthRateEntry{WindowStart: now}
	}
	if entry.Count >= 120 {
		return false
	}
	entry.Count++
	externalAuthStartLimit.Clients[key] = entry
	externalAuthStartLimit.GlobalCount++
	if len(externalAuthStartLimit.Clients) > 2048 {
		for clientKey, clientEntry := range externalAuthStartLimit.Clients {
			if now.Sub(clientEntry.WindowStart) >= time.Minute {
				delete(externalAuthStartLimit.Clients, clientKey)
			}
		}
	}
	return true
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

func addQueryValue(rawURL, key, value string) (string, error) {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return "", err
	}
	query := parsed.Query()
	query.Set(key, value)
	parsed.RawQuery = query.Encode()
	return parsed.String(), nil
}

func casProtocolURL(serverURL, endpoint string, query url.Values) (string, error) {
	parsed, err := url.Parse(serverURL)
	if err != nil {
		return "", err
	}
	parsed.Path = strings.TrimRight(parsed.Path, "/") + endpoint
	parsed.RawPath = ""
	parsed.RawQuery = query.Encode()
	return parsed.String(), nil
}

type limitedResponseTransport struct {
	base  http.RoundTripper
	limit int64
}

func (transport limitedResponseTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	response, err := transport.base.RoundTrip(request)
	if err != nil {
		return nil, err
	}
	response.Body = &limitedReadCloser{Reader: io.LimitReader(response.Body, transport.limit+1), Closer: response.Body}
	return response, nil
}

type limitedReadCloser struct {
	io.Reader
	io.Closer
}

func externalHTTPClient(noRedirect bool, responseLimit int64) *http.Client {
	base, ok := http.DefaultTransport.(*http.Transport)
	if !ok {
		base = &http.Transport{}
	} else {
		base = base.Clone()
	}
	base.ResponseHeaderTimeout = 10 * time.Second
	base.TLSHandshakeTimeout = 10 * time.Second
	base.MaxResponseHeaderBytes = 1 << 20
	client := &http.Client{
		Timeout:   15 * time.Second,
		Transport: limitedResponseTransport{base: base, limit: responseLimit},
	}
	if noRedirect {
		client.CheckRedirect = func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }
	}
	return client
}

func externalOIDCContext(ctx context.Context) context.Context {
	// Discovery、Token 和 JWKS 端点都应提供最终 URL；禁止 30x 避免 code/secret 被转发。
	return oidc.ClientContext(ctx, externalHTTPClient(true, externalAuthMaxHTTPBody))
}

type casAuthenticationSuccess struct {
	XMLName    xml.Name
	User       casXMLValue   `xml:"user"`
	Attributes casAttributes `xml:"attributes"`
}

type casXMLValue struct {
	XMLName xml.Name
	Value   string `xml:",chardata"`
}

type casAuthenticationFailure struct {
	XMLName xml.Name
	Code    string `xml:"code,attr"`
}

type casServiceResponse struct {
	XMLName xml.Name
	Success *casAuthenticationSuccess `xml:"authenticationSuccess"`
	Failure *casAuthenticationFailure `xml:"authenticationFailure"`
}

type casAttributes map[string][]string

func (attributes *casAttributes) UnmarshalXML(decoder *xml.Decoder, start xml.StartElement) error {
	result := make(casAttributes)
	for {
		token, err := decoder.Token()
		if err != nil {
			return err
		}
		switch value := token.(type) {
		case xml.StartElement:
			var content string
			if err := decoder.DecodeElement(&content, &value); err != nil {
				return err
			}
			if value.Name.Space != casXMLNamespace {
				continue
			}
			content = strings.TrimSpace(content)
			if content != "" {
				result[value.Name.Local] = append(result[value.Name.Local], content)
			}
		case xml.EndElement:
			if value.Name == start.Name {
				*attributes = result
				return nil
			}
		}
	}
}

type casValidatedIdentity struct {
	User       string
	Attributes casAttributes
}

func validateCAS3Ticket(ctx context.Context, serverURL, serviceURL, ticket string) (casValidatedIdentity, error) {
	var validated casValidatedIdentity
	validateURL, err := casProtocolURL(serverURL, "/p3/serviceValidate", url.Values{
		"service": []string{serviceURL},
		"ticket":  []string{ticket},
	})
	if err != nil {
		return validated, err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, validateURL, nil)
	if err != nil {
		return validated, err
	}
	request.Header.Set("Accept", "application/xml, text/xml")
	response, err := externalHTTPClient(true, casMaxResponseBody).Do(request)
	if err != nil {
		return validated, fmt.Errorf("CAS3 票据校验请求失败: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return validated, fmt.Errorf("CAS3 票据校验返回 HTTP %d", response.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, casMaxResponseBody+1))
	if err != nil {
		return validated, fmt.Errorf("读取 CAS3 XML 失败: %w", err)
	}
	if int64(len(body)) > casMaxResponseBody {
		return validated, errors.New("CAS3 XML 响应超过 1 MiB")
	}
	return parseCAS3ServiceResponse(body)
}

func parseCAS3ServiceResponse(body []byte) (casValidatedIdentity, error) {
	var validated casValidatedIdentity
	upperBody := bytes.ToUpper(body)
	if bytes.Contains(upperBody, []byte("<!DOCTYPE")) || bytes.Contains(upperBody, []byte("<!ENTITY")) {
		return validated, errors.New("CAS3 XML 不允许 DTD 或自定义实体")
	}
	decoder := xml.NewDecoder(bytes.NewReader(body))
	decoder.Strict = true
	var response casServiceResponse
	if err := decoder.Decode(&response); err != nil {
		return validated, fmt.Errorf("CAS3 XML 解析失败: %w", err)
	}
	for {
		token, err := decoder.Token()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return validated, fmt.Errorf("CAS3 XML 尾部数据无效: %w", err)
		}
		switch value := token.(type) {
		case xml.CharData:
			if strings.TrimSpace(string(value)) != "" {
				return validated, errors.New("CAS3 XML 包含额外尾部数据")
			}
		case xml.Comment:
			// XML 允许根元素后存在注释。
		default:
			return validated, errors.New("CAS3 XML 包含额外根元素")
		}
	}
	if response.XMLName.Local != "serviceResponse" || response.XMLName.Space != casXMLNamespace {
		return validated, errors.New("CAS3 XML serviceResponse 命名空间无效")
	}
	if response.Success != nil && response.Failure != nil {
		return validated, errors.New("CAS3 XML 同时包含成功与失败响应")
	}
	if response.Failure != nil {
		if response.Failure.XMLName.Space != casXMLNamespace {
			return validated, errors.New("CAS3 XML authenticationFailure 命名空间无效")
		}
		return validated, errors.New("CAS3 票据校验失败")
	}
	if response.Success == nil || response.Success.XMLName.Space != casXMLNamespace {
		return validated, errors.New("CAS3 XML 缺少有效 authenticationSuccess")
	}
	if response.Success.User.XMLName.Space != casXMLNamespace {
		return validated, errors.New("CAS3 XML user 命名空间无效")
	}
	user := strings.TrimSpace(response.Success.User.Value)
	if user == "" || len(user) > 2048 {
		return validated, errors.New("CAS3 XML 缺少有效 user")
	}
	validated.User = user
	validated.Attributes = response.Success.Attributes
	return validated, nil
}

func firstCASAttribute(attributes casAttributes, keys ...string) string {
	for _, wanted := range keys {
		for key, values := range attributes {
			if !strings.EqualFold(key, wanted) {
				continue
			}
			for _, value := range values {
				if trimmed := strings.TrimSpace(value); trimmed != "" {
					return trimmed
				}
			}
		}
	}
	return ""
}

func truncateASCII(value string, max int) string {
	if max <= 0 {
		return ""
	}
	if len(value) <= max {
		return value
	}
	return value[:max]
}

func truncateUTF8(value string, max int) string {
	runes := []rune(strings.TrimSpace(value))
	if len(runes) <= max {
		return string(runes)
	}
	return string(runes[:max])
}
