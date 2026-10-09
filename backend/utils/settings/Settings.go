package settings

import (
	"fmt"
	"gorm.io/gorm"
	"oneimg/backend/database"
	"oneimg/backend/models"
	"oneimg/backend/utils/secureconfig"
	"sync"
	"time"
)

const cacheTTL = 5 * time.Second

type settingsCache struct {
	mu      sync.Mutex
	db      *gorm.DB
	row     models.Settings
	expires time.Time
	secrets map[string]string
}

var cache settingsCache

// GetSettings caches the stored row, not plaintext secrets. Reads have no
// migration/write side effects. Return by value keeps callers from changing it.
func GetSettings() (models.Settings, error) {
	db := database.GetDB()
	if db == nil || db.DB == nil {
		return models.Settings{}, fmt.Errorf("database is not initialized")
	}
	return cache.get(db.DB, time.Now())
}
func (c *settingsCache) get(db *gorm.DB, now time.Time) (models.Settings, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.loadLocked(db, now)
}
func (c *settingsCache) loadLocked(db *gorm.DB, now time.Time) (models.Settings, error) {
	if c.db == db && now.Before(c.expires) {
		return c.row, nil
	}
	var row models.Settings
	if err := db.Where("id = ?", 1).First(&row).Error; err != nil {
		c.db = nil
		c.expires = time.Time{}
		c.secrets = nil
		return models.Settings{}, err
	}
	c.db = db
	c.row = row
	c.expires = now.Add(cacheTTL)
	c.secrets = make(map[string]string)
	return row, nil
}

// Invalidate must run after the settings transaction COMMIT succeeds. The TTL
// also bounds staleness across processes/manual SQL, without caching DB errors.
func Invalidate() {
	cache.mu.Lock()
	defer cache.mu.Unlock()
	cache.db = nil
	cache.row = models.Settings{}
	cache.expires = time.Time{}
	cache.secrets = nil
}

func secretValue(row models.Settings, key string) (string, error) {
	switch key {
	case "tg_bot_token":
		return row.TGBotToken, nil
	case "oidc_client_secret":
		return row.OIDCClientSecret, nil
	case "turnstile_secret_key":
		return row.TurnstileSecret, nil
	case "cloudflare_api_token":
		return row.CloudflareAPIToken, nil
	default:
		return "", fmt.Errorf("unsupported settings secret %q", key)
	}
}
func (c *settingsCache) secret(db *gorm.DB, key string, now time.Time) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	row, err := c.loadLocked(db, now)
	if err != nil {
		return "", err
	}
	if value, ok := c.secrets[key]; ok {
		return value, nil
	}
	encrypted, err := secretValue(row, key)
	if err != nil {
		return "", err
	}
	value, err := secureconfig.DecryptSettingValue(key, encrypted)
	if err != nil {
		return "", err
	}
	c.secrets[key] = value
	return value, nil
}

// GetSecret decrypts only the requested key, once per cache generation.
func GetSecret(key string) (string, error) {
	db := database.GetDB()
	if db == nil || db.DB == nil {
		return "", fmt.Errorf("database is not initialized")
	}
	return cache.secret(db.DB, key, time.Now())
}

// GetSettingsWithSecrets explicitly opts selected consumers into decryption.
func GetSettingsWithSecrets(keys ...string) (models.Settings, error) {
	db := database.GetDB()
	if db == nil || db.DB == nil {
		return models.Settings{}, fmt.Errorf("database is not initialized")
	}
	cache.mu.Lock()
	defer cache.mu.Unlock()
	row, err := cache.loadLocked(db.DB, time.Now())
	if err != nil {
		return row, err
	}
	for _, key := range keys {
		value, ok := cache.secrets[key]
		if !ok {
			encrypted, e := secretValue(row, key)
			if e != nil {
				return models.Settings{}, e
			}
			value, e = secureconfig.DecryptSettingValue(key, encrypted)
			if e != nil {
				return models.Settings{}, e
			}
			cache.secrets[key] = value
		}
		switch key {
		case "tg_bot_token":
			row.TGBotToken = value
		case "oidc_client_secret":
			row.OIDCClientSecret = value
		case "turnstile_secret_key":
			row.TurnstileSecret = value
		case "cloudflare_api_token":
			row.CloudflareAPIToken = value
		}
	}
	return row, nil
}

// MigrateSecrets is an explicit startup operation, never a GET side effect.
func MigrateSecrets(db *gorm.DB) error {
	err := db.Transaction(func(tx *gorm.DB) error {
		var row models.Settings
		if err := tx.Where("id = ?", 1).First(&row).Error; err != nil {
			return err
		}
		changed, err := secureconfig.TryMigrateSettingsSecrets(&row)
		if err != nil {
			return err
		}
		if !changed {
			return nil
		}
		return tx.Model(&models.Settings{}).Where("id = ?", 1).Updates(map[string]interface{}{
			"tg_bot_token": row.TGBotToken, "api_token": row.APIToken, "api_token_hash": row.APITokenHash, "oidc_client_secret": row.OIDCClientSecret, "turnstile_secret_key": row.TurnstileSecret, "cloudflare_api_token": row.CloudflareAPIToken,
		}).Error
	})
	if err == nil {
		Invalidate()
	}
	return err
}

// RegisterInvalidation covers ordinary GORM writes as a safety net. Transaction
// users still invalidate after commit to fence an in-flight pre-commit reload.
func RegisterInvalidation(db *gorm.DB) error {
	hook := func(tx *gorm.DB) {
		if tx.Error == nil && tx.Statement != nil && (tx.Statement.Table == "settings" || (tx.Statement.Schema != nil && tx.Statement.Schema.Table == "settings")) {
			Invalidate()
		}
	}
	if err := db.Callback().Update().After("gorm:commit_or_rollback_transaction").Register("settings:invalidate", hook); err != nil {
		return err
	}
	if err := db.Callback().Create().After("gorm:commit_or_rollback_transaction").Register("settings:invalidate", hook); err != nil {
		return err
	}
	return db.Callback().Delete().After("gorm:commit_or_rollback_transaction").Register("settings:invalidate", hook)
}
