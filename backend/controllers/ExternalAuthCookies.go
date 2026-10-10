package controllers

import (
 "crypto/hmac"
 "crypto/sha256"
 "encoding/base64"
 "encoding/hex"
 "net/http"
 "strings"
 "time"
 "oneimg/backend/config"
 "github.com/gin-gonic/gin"
)

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
