package controllers

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"testing"
	"time"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"

	"oneimg/backend/config"
	"oneimg/backend/database"
	"oneimg/backend/middlewares"
	"oneimg/backend/models"
)

const (
	testOIDCClientID     = "oneimg-test-client"
	testOIDCClientSecret = "oneimg-test-client-secret"
	testOIDCKeyID        = "oneimg-test-key"
)

func TestOIDCHandlersAuthorizationCodeFlowAndReplayProtection(t *testing.T) {
	initExternalAuthTestDB(t)
	provider := newTestOIDCProvider(t)
	createTestOIDCSettings(t, provider.URL())
	router := newExternalAuthOIDCTestRouter()

	startRecorder := httptest.NewRecorder()
	startRequest := httptest.NewRequest(http.MethodGet, "https://oneimg.example/api/auth/oidc/login", nil)
	router.ServeHTTP(startRecorder, startRequest)
	if startRecorder.Code != http.StatusFound {
		t.Fatalf("StartOIDCLogin status = %d, want %d; body=%s", startRecorder.Code, http.StatusFound, startRecorder.Body.String())
	}
	authorizationURL, err := url.Parse(startRecorder.Header().Get("Location"))
	if err != nil {
		t.Fatalf("parse OIDC authorization redirect: %v", err)
	}
	if authorizationURL.Scheme+"://"+authorizationURL.Host != provider.URL() || authorizationURL.Path != "/authorize" {
		t.Fatalf("authorization endpoint = %q, want %s/authorize", authorizationURL.String(), provider.URL())
	}
	authorizationQuery := authorizationURL.Query()
	state := authorizationQuery.Get("state")
	nonce := authorizationQuery.Get("nonce")
	challenge := authorizationQuery.Get("code_challenge")
	if len(state) < 32 || len(nonce) < 32 {
		t.Fatalf("state/nonce lack entropy: state length=%d nonce length=%d", len(state), len(nonce))
	}
	if authorizationQuery.Get("response_type") != "code" || authorizationQuery.Get("client_id") != testOIDCClientID {
		t.Fatalf("unexpected authorization request parameters: %v", authorizationQuery)
	}
	if authorizationQuery.Get("redirect_uri") != "https://oneimg.example/api/auth/oidc/callback" {
		t.Fatalf("redirect_uri = %q", authorizationQuery.Get("redirect_uri"))
	}
	if authorizationQuery.Get("scope") != "openid profile email" {
		t.Fatalf("scope = %q, want openid profile email", authorizationQuery.Get("scope"))
	}
	if authorizationQuery.Get("code_challenge_method") != "S256" || challenge == "" {
		t.Fatalf("PKCE parameters missing or invalid: %v", authorizationQuery)
	}

	flowCookie := requireExternalAuthCookie(t, startRecorder, externalAuthCookieName(state))
	if flowCookie.Path != "/api/auth" || !flowCookie.HttpOnly || !flowCookie.Secure || flowCookie.SameSite != http.SameSiteLaxMode {
		t.Fatalf("unexpected OIDC flow cookie attributes: %+v", flowCookie)
	}
	if flowCookie.MaxAge != int(externalAuthFlowTTL.Seconds()) {
		t.Fatalf("OIDC flow cookie MaxAge = %d, want %d", flowCookie.MaxAge, int(externalAuthFlowTTL.Seconds()))
	}

	var flow models.ExternalAuthFlow
	if err := database.GetDB().DB.Where("state_hash = ?", hashExternalAuthState(state)).First(&flow).Error; err != nil {
		t.Fatalf("query OIDC auth flow: %v", err)
	}
	if flow.Provider != "oidc" || flow.Nonce != nonce || flow.ClientID != testOIDCClientID {
		t.Fatalf("unexpected persisted OIDC flow: %+v", flow)
	}
	if flow.CallbackURL != authorizationQuery.Get("redirect_uri") {
		t.Fatalf("persisted callback URL = %q, authorization redirect_uri = %q", flow.CallbackURL, authorizationQuery.Get("redirect_uri"))
	}
	if remaining := time.Until(flow.ExpiresAt); remaining < 9*time.Minute || remaining > externalAuthFlowTTL+5*time.Second {
		t.Fatalf("OIDC flow expiration remaining = %v, want about %v", remaining, externalAuthFlowTTL)
	}
	wantChallenge := pkceS256Challenge(flow.CodeVerifier)
	if challenge != wantChallenge {
		t.Fatalf("code_challenge = %q, want S256(%q) = %q", challenge, flow.CodeVerifier, wantChallenge)
	}
	provider.ConfigureFlow(nonce, challenge)

	callbackRecorder := httptest.NewRecorder()
	callbackRequest := httptest.NewRequest(
		http.MethodGet,
		"https://oneimg.example/api/auth/oidc/callback?state="+url.QueryEscape(state)+"&code=valid-code",
		nil,
	)
	callbackRequest.AddCookie(flowCookie)
	router.ServeHTTP(callbackRecorder, callbackRequest)
	if callbackRecorder.Code != http.StatusFound {
		t.Fatalf("OIDCCallback status = %d, want %d; body=%s", callbackRecorder.Code, http.StatusFound, callbackRecorder.Body.String())
	}
	successLocation := parseTestLocation(t, callbackRecorder.Header().Get("Location"))
	if successLocation.Path != "/login" || successLocation.Query().Get("external_login") != "success" || successLocation.Query().Get("provider") != "oidc" {
		t.Fatalf("OIDCCallback success redirect = %q", successLocation.String())
	}

	tokenCalls, tokenForm, tokenClientID, tokenClientSecret := provider.TokenRequestSnapshot()
	if tokenCalls != 1 {
		t.Fatalf("token endpoint calls = %d, want 1", tokenCalls)
	}
	if tokenForm.Get("grant_type") != "authorization_code" || tokenForm.Get("code") != "valid-code" {
		t.Fatalf("unexpected token request form: %v", tokenForm)
	}
	if tokenForm.Get("redirect_uri") != flow.CallbackURL || tokenForm.Get("code_verifier") != flow.CodeVerifier {
		t.Fatalf("token request did not use persisted callback/verifier: %v", tokenForm)
	}
	if tokenClientID != testOIDCClientID || tokenClientSecret != testOIDCClientSecret {
		t.Fatalf("token client credentials = (%q, %q), want configured credentials", tokenClientID, tokenClientSecret)
	}

	sessionCookie := requireExternalAuthCookie(t, callbackRecorder, "oneimg-session")
	if !sessionCookie.HttpOnly || !sessionCookie.Secure || sessionCookie.SameSite != http.SameSiteStrictMode {
		t.Fatalf("unexpected authenticated session cookie: %+v", sessionCookie)
	}
	probeRecorder := httptest.NewRecorder()
	probeRequest := httptest.NewRequest(http.MethodGet, "https://oneimg.example/test/session", nil)
	probeRequest.AddCookie(sessionCookie)
	router.ServeHTTP(probeRecorder, probeRequest)
	var sessionState struct {
		LoggedIn bool   `json:"logged_in"`
		UserID   int    `json:"user_id"`
		Role     int    `json:"role"`
		Username string `json:"username"`
	}
	if err := json.Unmarshal(probeRecorder.Body.Bytes(), &sessionState); err != nil {
		t.Fatalf("decode authenticated session probe: %v; body=%s", err, probeRecorder.Body.String())
	}
	if !sessionState.LoggedIn || sessionState.UserID <= 0 || sessionState.Role != models.RoleUser || sessionState.Username != "oidc-user" {
		t.Fatalf("unexpected authenticated session: %+v", sessionState)
	}

	var identity models.ExternalIdentity
	if err := database.GetDB().DB.Where("provider = ? AND issuer = ? AND subject = ?", "oidc", provider.URL(), "subject-123").First(&identity).Error; err != nil {
		t.Fatalf("query provisioned OIDC identity: %v", err)
	}
	if identity.UserID != sessionState.UserID {
		t.Fatalf("OIDC identity user_id = %d, session user_id = %d", identity.UserID, sessionState.UserID)
	}
	var flowCount int64
	if err := database.GetDB().DB.Model(&models.ExternalAuthFlow{}).Where("state_hash = ?", flow.StateHash).Count(&flowCount).Error; err != nil {
		t.Fatalf("count consumed OIDC flow: %v", err)
	}
	if flowCount != 0 {
		t.Fatalf("successful OIDC flow remains in database; count=%d", flowCount)
	}

	// Reusing the original state and its still-valid signed browser cookie must
	// fail before another authorization code is sent to the token endpoint.
	replayRecorder := httptest.NewRecorder()
	replayRequest := httptest.NewRequest(
		http.MethodGet,
		"https://oneimg.example/api/auth/oidc/callback?state="+url.QueryEscape(state)+"&code=replayed-code",
		nil,
	)
	replayRequest.AddCookie(flowCookie)
	router.ServeHTTP(replayRecorder, replayRequest)
	if replayRecorder.Code != http.StatusFound {
		t.Fatalf("OIDC replay status = %d, want %d", replayRecorder.Code, http.StatusFound)
	}
	replayLocation := parseTestLocation(t, replayRecorder.Header().Get("Location"))
	if replayLocation.Query().Get("external_login") != "error" || replayLocation.Query().Get("error_code") != "invalid_state" {
		t.Fatalf("OIDC replay redirect = %q, want invalid_state", replayLocation.String())
	}
	afterReplayCalls, _, _, _ := provider.TokenRequestSnapshot()
	if afterReplayCalls != tokenCalls {
		t.Fatalf("replayed state reached token endpoint: calls before=%d after=%d", tokenCalls, afterReplayCalls)
	}
}

type testOIDCProvider struct {
	server *httptest.Server
	signer *rsa.PrivateKey
	jwks   []byte

	mu                    sync.Mutex
	expectedNonce         string
	expectedPKCE          string
	tokenCalls            int
	lastTokenForm         url.Values
	lastTokenClientID     string
	lastTokenClientSecret string
}

func newTestOIDCProvider(t *testing.T) *testOIDCProvider {
	t.Helper()
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate OIDC RSA key: %v", err)
	}
	provider := &testOIDCProvider{signer: privateKey}
	provider.jwks, err = json.Marshal(map[string]any{
		"keys": []map[string]any{{
			"kty": "RSA",
			"kid": testOIDCKeyID,
			"use": "sig",
			"alg": "RS256",
			"n":   base64.RawURLEncoding.EncodeToString(privateKey.PublicKey.N.Bytes()),
			"e":   base64.RawURLEncoding.EncodeToString(rsaExponentBytes(privateKey.PublicKey.E)),
		}},
	})
	if err != nil {
		t.Fatalf("marshal OIDC JWKS: %v", err)
	}
	provider.server = httptest.NewServer(http.HandlerFunc(provider.serveHTTP))
	t.Cleanup(provider.server.Close)
	return provider
}

func (provider *testOIDCProvider) URL() string { return provider.server.URL }

func (provider *testOIDCProvider) ConfigureFlow(nonce, challenge string) {
	provider.mu.Lock()
	defer provider.mu.Unlock()
	provider.expectedNonce = nonce
	provider.expectedPKCE = challenge
}

func (provider *testOIDCProvider) TokenRequestSnapshot() (int, url.Values, string, string) {
	provider.mu.Lock()
	defer provider.mu.Unlock()
	return provider.tokenCalls, cloneTestURLValues(provider.lastTokenForm), provider.lastTokenClientID, provider.lastTokenClientSecret
}

func (provider *testOIDCProvider) serveHTTP(writer http.ResponseWriter, request *http.Request) {
	switch request.URL.Path {
	case "/.well-known/openid-configuration":
		writer.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(writer).Encode(map[string]any{
			"issuer":                                provider.URL(),
			"authorization_endpoint":                provider.URL() + "/authorize",
			"token_endpoint":                        provider.URL() + "/token",
			"jwks_uri":                              provider.URL() + "/jwks",
			"response_types_supported":              []string{"code"},
			"subject_types_supported":               []string{"public"},
			"id_token_signing_alg_values_supported": []string{"RS256"},
		})
	case "/jwks":
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write(provider.jwks)
	case "/token":
		provider.serveToken(writer, request)
	default:
		http.NotFound(writer, request)
	}
}

func (provider *testOIDCProvider) serveToken(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost {
		writer.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	if err := request.ParseForm(); err != nil {
		http.Error(writer, "invalid form", http.StatusBadRequest)
		return
	}
	clientID, clientSecret, hasBasicAuth := request.BasicAuth()
	if !hasBasicAuth {
		clientID = request.PostForm.Get("client_id")
		clientSecret = request.PostForm.Get("client_secret")
	}

	provider.mu.Lock()
	provider.tokenCalls++
	provider.lastTokenForm = cloneTestURLValues(request.PostForm)
	provider.lastTokenClientID = clientID
	provider.lastTokenClientSecret = clientSecret
	expectedNonce := provider.expectedNonce
	expectedChallenge := provider.expectedPKCE
	provider.mu.Unlock()

	if request.PostForm.Get("code") != "valid-code" || clientID != testOIDCClientID || clientSecret != testOIDCClientSecret {
		http.Error(writer, "invalid authorization code or client", http.StatusBadRequest)
		return
	}
	if pkceS256Challenge(request.PostForm.Get("code_verifier")) != expectedChallenge {
		http.Error(writer, "invalid code verifier", http.StatusBadRequest)
		return
	}
	idToken, err := provider.signIDToken(expectedNonce)
	if err != nil {
		http.Error(writer, "failed to sign token", http.StatusInternalServerError)
		return
	}
	writer.Header().Set("Content-Type", "application/json")
	writer.Header().Set("Cache-Control", "no-store")
	_ = json.NewEncoder(writer).Encode(map[string]any{
		"access_token": "test-access-token",
		"token_type":   "Bearer",
		"expires_in":   3600,
		"id_token":     idToken,
	})
}

func (provider *testOIDCProvider) signIDToken(nonce string) (string, error) {
	now := time.Now()
	header, err := json.Marshal(map[string]any{"alg": "RS256", "kid": testOIDCKeyID, "typ": "JWT"})
	if err != nil {
		return "", err
	}
	payload, err := json.Marshal(map[string]any{
		"iss":                provider.URL(),
		"sub":                "subject-123",
		"aud":                testOIDCClientID,
		"exp":                now.Add(5 * time.Minute).Unix(),
		"iat":                now.Unix(),
		"nonce":              nonce,
		"preferred_username": "oidc-user",
		"email":              "oidc-user@example.com",
		"name":               "OIDC Test User",
	})
	if err != nil {
		return "", err
	}
	unsigned := base64.RawURLEncoding.EncodeToString(header) + "." + base64.RawURLEncoding.EncodeToString(payload)
	digest := sha256.Sum256([]byte(unsigned))
	signature, err := rsa.SignPKCS1v15(rand.Reader, provider.signer, crypto.SHA256, digest[:])
	if err != nil {
		return "", err
	}
	return unsigned + "." + base64.RawURLEncoding.EncodeToString(signature), nil
}

func createTestOIDCSettings(t *testing.T, issuer string) {
	t.Helper()
	setting := models.Settings{
		OIDCEnable:        true,
		OIDCIssuer:        issuer,
		OIDCClientID:      testOIDCClientID,
		OIDCClientSecret:  testOIDCClientSecret,
		OIDCRedirectURL:   "https://oneimg.example/api/auth/oidc/callback",
		OIDCScopes:        "openid profile email",
		OIDCUsernameClaim: "preferred_username",
	}
	if err := database.GetDB().DB.Create(&setting).Error; err != nil {
		t.Fatalf("create OIDC settings: %v", err)
	}
}

func newExternalAuthOIDCTestRouter() *gin.Engine {
	router := gin.New()
	router.Use(middlewares.SessionMiddleware(config.App))
	router.GET("/api/auth/oidc/login", StartOIDCLogin)
	router.GET("/api/auth/oidc/callback", OIDCCallback)
	router.GET("/test/session", func(context *gin.Context) {
		session := sessions.Default(context)
		context.JSON(http.StatusOK, gin.H{
			"logged_in": session.Get("logged_in"),
			"user_id":   session.Get("user_id"),
			"role":      session.Get("user_role"),
			"username":  session.Get("username"),
		})
	})
	return router
}

func parseTestLocation(t *testing.T, raw string) *url.URL {
	t.Helper()
	parsed, err := url.Parse(raw)
	if err != nil {
		t.Fatalf("parse redirect location %q: %v", raw, err)
	}
	return parsed
}

func pkceS256Challenge(verifier string) string {
	digest := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(digest[:])
}

func rsaExponentBytes(exponent int) []byte {
	if exponent == 0 {
		return []byte{0}
	}
	value := make([]byte, 0, 4)
	for exponent > 0 {
		value = append([]byte{byte(exponent)}, value...)
		exponent >>= 8
	}
	return value
}

func cloneTestURLValues(values url.Values) url.Values {
	cloned := make(url.Values, len(values))
	for key, entries := range values {
		cloned[key] = append([]string(nil), entries...)
	}
	return cloned
}
