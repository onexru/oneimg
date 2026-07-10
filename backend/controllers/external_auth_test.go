package controllers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"

	"oneimg/backend/config"
	"oneimg/backend/database"
	"oneimg/backend/models"
)

func TestParseCAS3ServiceResponse(t *testing.T) {
	tests := []struct {
		name           string
		xml            string
		wantUser       string
		wantAttributes casAttributes
		wantError      string
	}{
		{
			name: "official namespace success with attributes",
			xml: `<?xml version="1.0" encoding="UTF-8"?>
<cas:serviceResponse xmlns:cas="http://www.yale.edu/tp/cas">
  <cas:authenticationSuccess>
    <cas:user> alice </cas:user>
    <cas:attributes>
      <cas:email>alice@example.com</cas:email>
      <cas:memberOf>staff</cas:memberOf>
      <cas:memberOf>developers</cas:memberOf>
      <cas:empty>   </cas:empty>
    </cas:attributes>
  </cas:authenticationSuccess>
</cas:serviceResponse>`,
			wantUser: "alice",
			wantAttributes: casAttributes{
				"email":    {"alice@example.com"},
				"memberOf": {"staff", "developers"},
			},
		},
		{
			name: "official default namespace success",
			xml: `<serviceResponse xmlns="http://www.yale.edu/tp/cas">
  <authenticationSuccess><user>bob</user></authenticationSuccess>
</serviceResponse>`,
			wantUser:       "bob",
			wantAttributes: nil,
		},
		{
			name: "foreign namespace attributes are ignored",
			xml: `<cas:serviceResponse xmlns:cas="http://www.yale.edu/tp/cas" xmlns:evil="https://attacker.invalid/cas">
  <cas:authenticationSuccess><cas:user>bob</cas:user><cas:attributes>
    <evil:uid>mallory</evil:uid><cas:email>bob@example.com</cas:email>
  </cas:attributes></cas:authenticationSuccess>
</cas:serviceResponse>`,
			wantUser: "bob",
			wantAttributes: casAttributes{
				"email": {"bob@example.com"},
			},
		},
		{
			name: "authentication failure",
			xml: `<cas:serviceResponse xmlns:cas="http://www.yale.edu/tp/cas">
  <cas:authenticationFailure code="INVALID_TICKET">ticket rejected</cas:authenticationFailure>
</cas:serviceResponse>`,
			wantError: "CAS3 票据校验失败",
		},
		{
			name: "wrong root namespace",
			xml: `<cas:serviceResponse xmlns:cas="https://attacker.invalid/cas">
  <cas:authenticationSuccess><cas:user>mallory</cas:user></cas:authenticationSuccess>
</cas:serviceResponse>`,
			wantError: "serviceResponse 命名空间无效",
		},
		{
			name: "wrong success namespace",
			xml: `<cas:serviceResponse xmlns:cas="http://www.yale.edu/tp/cas" xmlns:evil="https://attacker.invalid/cas">
  <evil:authenticationSuccess><evil:user>mallory</evil:user></evil:authenticationSuccess>
</cas:serviceResponse>`,
			wantError: "缺少有效 authenticationSuccess",
		},
		{
			name: "wrong user namespace",
			xml: `<cas:serviceResponse xmlns:cas="http://www.yale.edu/tp/cas" xmlns:evil="https://attacker.invalid/cas">
  <cas:authenticationSuccess><evil:user>mallory</evil:user></cas:authenticationSuccess>
</cas:serviceResponse>`,
			wantError: "user 命名空间无效",
		},
		{
			name: "empty user",
			xml: `<cas:serviceResponse xmlns:cas="http://www.yale.edu/tp/cas">
  <cas:authenticationSuccess><cas:user>   </cas:user></cas:authenticationSuccess>
</cas:serviceResponse>`,
			wantError: "缺少有效 user",
		},
		{
			name: "doctype is rejected",
			xml: `<?xml version="1.0"?>
<!DOCTYPE serviceResponse [<!ENTITY username "mallory">]>
<cas:serviceResponse xmlns:cas="http://www.yale.edu/tp/cas">
  <cas:authenticationSuccess><cas:user>&username;</cas:user></cas:authenticationSuccess>
</cas:serviceResponse>`,
			wantError: "不允许 DTD 或自定义实体",
		},
		{
			name: "trailing second root is rejected",
			xml: `<cas:serviceResponse xmlns:cas="http://www.yale.edu/tp/cas">
  <cas:authenticationSuccess><cas:user>alice</cas:user></cas:authenticationSuccess>
</cas:serviceResponse><cas:serviceResponse xmlns:cas="http://www.yale.edu/tp/cas"/>`,
			wantError: "额外根元素",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseCAS3ServiceResponse([]byte(tt.xml))
			if tt.wantError != "" {
				if err == nil {
					t.Fatalf("parseCAS3ServiceResponse() error = nil, want error containing %q; result=%+v", tt.wantError, got)
				}
				if !strings.Contains(err.Error(), tt.wantError) {
					t.Fatalf("parseCAS3ServiceResponse() error = %q, want substring %q", err, tt.wantError)
				}
				return
			}

			if err != nil {
				t.Fatalf("parseCAS3ServiceResponse() unexpected error: %v", err)
			}
			if got.User != tt.wantUser {
				t.Fatalf("parseCAS3ServiceResponse() user = %q, want %q", got.User, tt.wantUser)
			}
			if !reflect.DeepEqual(got.Attributes, tt.wantAttributes) {
				t.Fatalf("parseCAS3ServiceResponse() attributes = %#v, want %#v", got.Attributes, tt.wantAttributes)
			}
		})
	}
}

func TestValidateCAS3TicketUsesExactServiceAndDoesNotRedirect(t *testing.T) {
	serviceURL := "https://oneimg.example/api/auth/cas/callback?state=exact-state"
	ticket := "ST-1-secret-ticket"

	t.Run("success uses CAS3 endpoint and exact service", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			if request.URL.Path != "/cas/p3/serviceValidate" {
				t.Errorf("CAS validation path = %q", request.URL.Path)
			}
			if request.URL.Query().Get("service") != serviceURL || request.URL.Query().Get("ticket") != ticket {
				t.Errorf("CAS validation query = %v", request.URL.Query())
			}
			writer.Header().Set("Content-Type", "application/xml")
			_, _ = writer.Write([]byte(`<cas:serviceResponse xmlns:cas="http://www.yale.edu/tp/cas"><cas:authenticationSuccess><cas:user>alice</cas:user></cas:authenticationSuccess></cas:serviceResponse>`))
		}))
		defer server.Close()

		identity, err := validateCAS3Ticket(context.Background(), server.URL+"/cas", serviceURL, ticket)
		if err != nil {
			t.Fatalf("validateCAS3Ticket() error: %v", err)
		}
		if identity.User != "alice" {
			t.Fatalf("validateCAS3Ticket() user = %q", identity.User)
		}
	})

	t.Run("redirect is rejected without forwarding ticket", func(t *testing.T) {
		var targetCalls atomic.Int32
		target := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			targetCalls.Add(1)
			writer.WriteHeader(http.StatusOK)
		}))
		defer target.Close()
		server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			http.Redirect(writer, request, target.URL, http.StatusFound)
		}))
		defer server.Close()

		if _, err := validateCAS3Ticket(context.Background(), server.URL, serviceURL, ticket); err == nil {
			t.Fatal("validateCAS3Ticket() followed redirect")
		}
		if targetCalls.Load() != 0 {
			t.Fatalf("CAS ticket leaked to redirect target; calls=%d", targetCalls.Load())
		}
	})
}

func TestExternalAuthFlowCookieAndDatabaseAreConsumedOnce(t *testing.T) {
	initExternalAuthTestDB(t)
	state := strings.Repeat("a", 43)
	flow := models.ExternalAuthFlow{
		StateHash:  hashExternalAuthState(state),
		Provider:   "cas",
		Issuer:     "https://cas.example.com/cas",
		ServiceURL: "https://oneimg.example/api/auth/cas/callback?state=" + state,
		ExpiresAt:  time.Now().Add(5 * time.Minute),
	}

	saveRecorder, saveContext := newExternalAuthTestContext(http.MethodGet, "https://oneimg.example/api/auth/cas/login")
	if err := saveExternalAuthFlow(saveContext, state, &flow); err != nil {
		t.Fatalf("saveExternalAuthFlow() error: %v", err)
	}
	cookie := requireExternalAuthCookie(t, saveRecorder, externalAuthCookieName(state))
	if cookie.Value != externalAuthCookieSignature(state) {
		t.Fatalf("flow cookie value = %q, want signed state value", cookie.Value)
	}
	if cookie.Path != "/api/auth" || !cookie.HttpOnly || !cookie.Secure || cookie.SameSite != http.SameSiteLaxMode {
		t.Fatalf("unexpected flow cookie attributes: %+v", cookie)
	}
	if cookie.MaxAge != int(externalAuthFlowTTL.Seconds()) {
		t.Fatalf("flow cookie MaxAge = %d, want %d", cookie.MaxAge, int(externalAuthFlowTTL.Seconds()))
	}

	var stored models.ExternalAuthFlow
	if err := database.GetDB().DB.First(&stored, "state_hash = ?", hashExternalAuthState(state)).Error; err != nil {
		t.Fatalf("query saved flow: %v", err)
	}
	if stored.StateHash == state {
		t.Fatal("database stored raw state instead of its hash")
	}

	tamperedRecorder, tamperedContext := newExternalAuthTestContext(http.MethodGet, "https://oneimg.example/api/auth/cas/callback?state="+state)
	tamperedCookie := *cookie
	tamperedCookie.Value = "tampered-cookie-signature"
	tamperedContext.Request.AddCookie(&tamperedCookie)
	if _, err := consumeExternalAuthFlow(tamperedContext, state, "cas"); err == nil || !strings.Contains(err.Error(), "Cookie 校验失败") {
		t.Fatalf("tampered cookie consume error = %v, want cookie validation failure", err)
	}
	if got := tamperedRecorder.Header().Values("Set-Cookie"); len(got) != 0 {
		t.Fatalf("tampered cookie unexpectedly changed response cookies: %q", got)
	}
	var count int64
	if err := database.GetDB().DB.Model(&models.ExternalAuthFlow{}).Where("state_hash = ?", flow.StateHash).Count(&count).Error; err != nil {
		t.Fatalf("count flow after tampered cookie: %v", err)
	}
	if count != 1 {
		t.Fatalf("tampered cookie consumed database flow; count=%d", count)
	}

	consumeRecorder, consumeContext := newExternalAuthTestContext(http.MethodGet, "https://oneimg.example/api/auth/cas/callback?state="+state)
	consumeContext.Request.AddCookie(cookie)
	consumed, err := consumeExternalAuthFlow(consumeContext, state, "cas")
	if err != nil {
		t.Fatalf("first consumeExternalAuthFlow() error: %v", err)
	}
	if consumed.StateHash != flow.StateHash || consumed.Provider != "cas" {
		t.Fatalf("first consumeExternalAuthFlow() = %+v, want saved flow", consumed)
	}
	cleared := requireExternalAuthCookie(t, consumeRecorder, externalAuthCookieName(state))
	if cleared.MaxAge >= 0 || cleared.Value != "" {
		t.Fatalf("consumed flow cookie was not cleared: %+v", cleared)
	}

	count = 0
	if err := database.GetDB().DB.Model(&models.ExternalAuthFlow{}).Where("state_hash = ?", flow.StateHash).Count(&count).Error; err != nil {
		t.Fatalf("count consumed flow: %v", err)
	}
	if count != 0 {
		t.Fatalf("consumed flow remains in database; count=%d", count)
	}

	replayRecorder, replayContext := newExternalAuthTestContext(http.MethodGet, "https://oneimg.example/api/auth/cas/callback?state="+state)
	replayContext.Request.AddCookie(cookie)
	if _, err := consumeExternalAuthFlow(replayContext, state, "cas"); err == nil {
		t.Fatal("replayed flow unexpectedly succeeded")
	}
	// A replay with a formerly valid cookie still receives a deletion cookie.
	requireExternalAuthCookie(t, replayRecorder, externalAuthCookieName(state))
}

func TestExternalAuthFlowExpiryIsRejectedAndConsumed(t *testing.T) {
	initExternalAuthTestDB(t)
	state := strings.Repeat("b", 43)
	flow := models.ExternalAuthFlow{
		StateHash: hashExternalAuthState(state),
		Provider:  "oidc",
		Issuer:    "https://idp.example.com",
		ExpiresAt: time.Now().Add(-time.Minute),
	}

	saveRecorder, saveContext := newExternalAuthTestContext(http.MethodGet, "https://oneimg.example/api/auth/oidc/login")
	if err := saveExternalAuthFlow(saveContext, state, &flow); err != nil {
		t.Fatalf("saveExternalAuthFlow() error: %v", err)
	}
	cookie := requireExternalAuthCookie(t, saveRecorder, externalAuthCookieName(state))

	consumeRecorder, consumeContext := newExternalAuthTestContext(http.MethodGet, "https://oneimg.example/api/auth/oidc/callback?state="+state)
	consumeContext.Request.AddCookie(cookie)
	if _, err := consumeExternalAuthFlow(consumeContext, state, "oidc"); err == nil || !strings.Contains(err.Error(), "已过期") {
		t.Fatalf("expired consumeExternalAuthFlow() error = %v, want expiration error", err)
	}
	cleared := requireExternalAuthCookie(t, consumeRecorder, externalAuthCookieName(state))
	if cleared.MaxAge >= 0 {
		t.Fatalf("expired flow cookie was not cleared: %+v", cleared)
	}

	var count int64
	if err := database.GetDB().DB.Model(&models.ExternalAuthFlow{}).Where("state_hash = ?", flow.StateHash).Count(&count).Error; err != nil {
		t.Fatalf("count expired flow: %v", err)
	}
	if count != 0 {
		t.Fatalf("expired flow should be consumed; count=%d", count)
	}
}

func TestResolveOrProvisionExternalIdentityCreatesRoleUserWithoutBindingSameUsername(t *testing.T) {
	initExternalAuthTestDB(t)
	db := database.GetDB().DB
	local := models.User{
		Role:       models.RoleAdmin,
		Username:   "alice",
		Password:   "local-password-hash",
		Permission: models.Permission{Buckets: []int{}},
	}
	if err := db.Create(&local).Error; err != nil {
		t.Fatalf("create local user: %v", err)
	}

	profile := externalIdentityProfile{
		Provider:    "oidc",
		Issuer:      "https://idp.example.com",
		Subject:     "subject-123",
		Username:    "alice",
		Email:       "alice@example.com",
		DisplayName: "Alice External",
	}
	external, err := resolveOrProvisionExternalIdentity(profile)
	if err != nil {
		t.Fatalf("resolveOrProvisionExternalIdentity() error: %v", err)
	}
	if external.ID == local.ID {
		t.Fatal("external identity was automatically bound to same-named local account")
	}
	if external.Role != models.RoleUser {
		t.Fatalf("external user role = %d, want RoleUser=%d", external.Role, models.RoleUser)
	}
	if strings.EqualFold(external.Username, local.Username) {
		t.Fatalf("external username %q collides with local username %q", external.Username, local.Username)
	}
	if err := bcrypt.CompareHashAndPassword([]byte(external.Password), []byte("")); err == nil {
		t.Fatal("external account unexpectedly has an empty usable password")
	}

	var identity models.ExternalIdentity
	if err := db.Where("identity_key = ?", externalIdentityKey(profile.Provider, profile.Issuer, profile.Subject)).First(&identity).Error; err != nil {
		t.Fatalf("query external identity: %v", err)
	}
	if identity.UserID != external.ID {
		t.Fatalf("identity user_id = %d, want %d", identity.UserID, external.ID)
	}

	profile.Username = "renamed-upstream-user"
	again, err := resolveOrProvisionExternalIdentity(profile)
	if err != nil {
		t.Fatalf("resolve existing external identity: %v", err)
	}
	if again.ID != external.ID {
		t.Fatalf("existing external identity resolved user %d, want %d", again.ID, external.ID)
	}
	if again.Username != external.Username {
		t.Fatalf("upstream username change rewrote local username: got %q, want %q", again.Username, external.Username)
	}
}

func TestResolveExternalIdentityHonorsProvisioningAndDisabledTombstone(t *testing.T) {
	initExternalAuthTestDB(t)
	profile := externalIdentityProfile{
		Provider: "cas",
		Issuer:   "https://cas.example.com/cas",
		Subject:  "blocked-user",
		Username: "blocked-user",
	}

	if _, err := resolveExternalIdentity(profile, false); err == nil || !strings.Contains(err.Error(), "未允许") {
		t.Fatalf("resolveExternalIdentity(auto=false) error = %v, want provisioning rejection", err)
	}
	var userCount int64
	if err := database.GetDB().DB.Model(&models.User{}).Count(&userCount).Error; err != nil || userCount != 0 {
		t.Fatalf("disabled provisioning created users: count=%d error=%v", userCount, err)
	}

	user, err := resolveExternalIdentity(profile, true)
	if err != nil {
		t.Fatalf("resolveExternalIdentity(auto=true) error: %v", err)
	}
	identityKey := externalIdentityKey(profile.Provider, profile.Issuer, profile.Subject)
	if err := database.GetDB().DB.Model(&models.ExternalIdentity{}).
		Where("identity_key = ?", identityKey).
		Updates(map[string]any{"disabled": true, "user_id": 0}).Error; err != nil {
		t.Fatalf("disable external identity: %v", err)
	}
	if err := database.GetDB().DB.Delete(&models.User{}, user.ID).Error; err != nil {
		t.Fatalf("delete provisioned user: %v", err)
	}

	if _, err := resolveExternalIdentity(profile, true); err == nil || !strings.Contains(err.Error(), "已被禁用") {
		t.Fatalf("disabled tombstone login error = %v, want disabled rejection", err)
	}
	userCount = 0
	if err := database.GetDB().DB.Model(&models.User{}).Count(&userCount).Error; err != nil || userCount != 0 {
		t.Fatalf("disabled identity recreated user: count=%d error=%v", userCount, err)
	}
}

func TestResolveExternalCallbackUserMapsExactUsernameToSuperAdmin(t *testing.T) {
	initExternalAuthTestDB(t)
	db := database.GetDB().DB
	superAdmin := models.User{
		ID:         models.SuperAdminID,
		Role:       models.RoleAdmin,
		Username:   "local-root",
		Password:   "unused-password-hash",
		Permission: models.Permission{Buckets: []int{}},
	}
	if err := db.Create(&superAdmin).Error; err != nil {
		t.Fatalf("create super admin: %v", err)
	}
	profile := externalIdentityProfile{
		Provider: "cas",
		Issuer:   "https://cas.example.com/cas",
		Subject:  "external-root",
		Username: "external-root",
	}

	mapped, err := resolveExternalCallbackUser(profile, "external-root", "external-root", false)
	if err != nil {
		t.Fatalf("resolveExternalCallbackUser() mapping error: %v", err)
	}
	if mapped.ID != models.SuperAdminID || mapped.Role != models.RoleAdmin || mapped.Username != "local-root" {
		t.Fatalf("mapped user = %+v, want local super admin", mapped)
	}
	var identityCount int64
	if err := db.Model(&models.ExternalIdentity{}).Count(&identityCount).Error; err != nil || identityCount != 0 {
		t.Fatalf("super admin mapping created external identity: count=%d error=%v", identityCount, err)
	}

	if _, err := resolveExternalCallbackUser(profile, "External-Root", "external-root", false); err == nil || !strings.Contains(err.Error(), "未允许") {
		t.Fatalf("case-mismatched username error = %v, want normal provisioning rejection", err)
	}
}

func initExternalAuthTestDB(t *testing.T) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	oldConfig := config.App
	config.App = &config.Config{
		AppURL:        "https://oneimg.example",
		SessionSecret: "external-auth-test-session-secret-32-bytes",
		ConfigSecret:  "external-auth-test-config-secret-32-bytes",
	}
	t.Cleanup(func() { config.App = oldConfig })
	database.InitDB(&config.Config{
		DbType:     "sqlite",
		SqlitePath: filepath.Join(t.TempDir(), "external-auth.db"),
	})
}

func newExternalAuthTestContext(method, target string) (*httptest.ResponseRecorder, *gin.Context) {
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(method, target, nil)
	return recorder, context
}

func requireExternalAuthCookie(t *testing.T, recorder *httptest.ResponseRecorder, name string) *http.Cookie {
	t.Helper()
	for _, cookie := range recorder.Result().Cookies() {
		if cookie.Name == name {
			return cookie
		}
	}
	t.Fatalf("response did not contain cookie %q; Set-Cookie=%q", name, recorder.Header().Values("Set-Cookie"))
	return nil
}
