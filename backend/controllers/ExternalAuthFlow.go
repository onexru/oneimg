package controllers

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"log"
	"net/http"
	"net/url"
	"oneimg/backend/database"
	"oneimg/backend/models"
	"strings"
	"time"
)

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
	var db *gorm.DB
	if current := database.GetDB(); current != nil {
		db = current.DB
	}
	externalAuthSuccessDB(c, db, provider)
}

func externalAuthSuccessDB(c *gin.Context, db *gorm.DB, provider string) {
	if userID, ok := sessions.Default(c).Get("user_id").(int); ok {
		recordAccountLogin(c, db, userID, provider)
	}
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
