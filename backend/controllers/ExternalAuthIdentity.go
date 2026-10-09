package controllers

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"fmt"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
	"oneimg/backend/database"
	"oneimg/backend/models"
	"strings"
	"time"
)

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
