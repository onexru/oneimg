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

// settingsCache 的锁纪律：临界区内绝不做数据库 IO。
//
// 这里曾经是「整段持 mu 查库」。那与 SQLite 的单连接池（backend/database/pool.go
// 把 MaxOpenConns 硬编码成 1）构成一对反向锁序：
//
//	读缓存：拿 mu → 等一条连接
//	写设置：握住事务的那条连接 → 提交后失效钩子要 mu（RegisterInvalidation）
//
// 两个 goroutine 各持对方要的东西，永久互等——database/sql 在
// context.Background() 下等连接没有超时，harness 上实测可稳定卡死。
// EnforceQuotaInTx 那处「事务内回池取连接」属于同一根因的另一种形状，已在
// controllers/users 里单独修掉；这里把持锁查库本身拆掉，才不会再长出第三种形状。
//
// 现在的形状：锁内只做内存操作（命中判断 / 写入快照），查库在锁外完成，
// 再用 gen 代际号防止「失效前读到的旧快照」覆盖「失效后应有的空缓存」。
type settingsCache struct {
	mu      sync.Mutex
	db      *gorm.DB
	row     models.Settings
	expires time.Time
	secrets map[string]string
	// gen 由 Invalidate 递增。查库在锁外进行，回填前必须确认期间没有失效过，
	// 否则会把失效之前的旧行重新写进缓存，让「改完设置立刻生效」退化成 5 秒 TTL。
	gen uint64
	// reload 非 nil 表示已有 goroutine 正在锁外查库：其余调用者等它结束再读缓存，
	// 把冷缓存瞬间的并发查询收敛成一次（池只有 1 条连接，多打并发没有收益）。
	reload chan struct{}
}

var cache settingsCache

// GetSettings caches the stored row, not plaintext secrets. Reads have no
// migration/write side effects. Return by value keeps callers from changing it.
func GetSettings() (models.Settings, error) {
	db := database.GetDB()
	if db == nil || db.DB == nil {
		return models.Settings{}, fmt.Errorf("database is not initialized")
	}
	row, _, err := cache.snapshot(db.DB, time.Now())
	return row, err
}

// snapshot 返回缓存行，以及这一行所属的缓存代际（供解密回填时判断是否过期）。
// 命中时只有内存操作；未命中时把查库放到锁外。
//
// 注意：本函数自己获取 c.mu，调用方不得持锁调用。
func (c *settingsCache) snapshot(db *gorm.DB, now time.Time) (models.Settings, uint64, error) {
	for {
		c.mu.Lock()
		if c.db == db && now.Before(c.expires) {
			row := c.row
			gen := c.gen
			c.mu.Unlock()
			return row, gen, nil
		}
		if c.reload != nil {
			// 已有 goroutine 在锁外查库：等它，而不是自己也去抢那唯一一条连接。
			// SQLite 池只有 1 条连接，冷缓存瞬间几十个请求各发一条 SELECT 会
			// 把连接排成长队，直接拖慢同一时刻的正常查询。
			wait := c.reload
			c.mu.Unlock()
			<-wait
			continue
		}
		done := make(chan struct{})
		c.reload = done
		gen := c.gen
		c.mu.Unlock()

		row, err := c.fetch(db, gen, now, done)
		if err != nil {
			return models.Settings{}, gen, err
		}
		return row, gen, nil
	}
}

// fetch 在锁外查库并回填缓存，无论成功、失败还是 panic 都要放出等待者。
//
// 放不出的后果是永久的：c.reload 一直非 nil，之后所有 GetSettings 都停在
// <-wait 上。所以这里用 defer 兜住，而不是在正常路径末尾手工收尾。
func (c *settingsCache) fetch(db *gorm.DB, gen uint64, now time.Time, done chan struct{}) (row models.Settings, err error) {
	defer func() {
		c.mu.Lock()
		if c.gen == gen {
			if err != nil {
				// 不缓存错误：下一次调用仍然重新查。
				c.resetLocked()
			} else {
				c.db = db
				c.row = row
				c.expires = now.Add(cacheTTL)
				c.secrets = make(map[string]string)
			}
		}
		// 期间被 Invalidate 过就不回填：这一行可能读于失效之前，写回去等于把
		// 刚提交的新设置又盖成旧值。语义与 TTL 一致：本次调用者拿到查到的行。
		c.reload = nil
		close(done)
		c.mu.Unlock()
	}()

	// 锁外查库。慢查询、连接池抖动、唯一那条连接被长事务占着，都只影响本次
	// 调用，不会再被放大成「全站等同一把锁」。
	err = db.Where("id = ?", 1).First(&row).Error
	return row, err
}

func (c *settingsCache) resetLocked() {
	c.db = nil
	c.row = models.Settings{}
	c.expires = time.Time{}
	c.secrets = nil
}

// Invalidate must run after the settings transaction COMMIT succeeds. The TTL
// also bounds staleness across processes/manual SQL, without caching DB errors.
//
// gen 递增会作废所有「已发出但尚未回填」的锁外查询，避免旧快照覆盖新状态。
func Invalidate() {
	cache.mu.Lock()
	defer cache.mu.Unlock()
	cache.gen++
	cache.resetLocked()
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

// decryptedSecret 取一个已解密的 secret，解密结果按缓存代际复用。
//
// 解密同样在锁外做：它是纯 CPU 操作，不该让其它 goroutine 陪着等。
// gen 是读到 row 时的代际：期间若发生 Invalidate，就不回填 —— 旧代际的明文
// 不该混进新一代缓存。本次调用者仍然拿到正确值。
func (c *settingsCache) decryptedSecret(key string, row models.Settings, gen uint64) (string, error) {
	c.mu.Lock()
	if c.secrets != nil {
		if value, ok := c.secrets[key]; ok {
			c.mu.Unlock()
			return value, nil
		}
	}
	c.mu.Unlock()

	encrypted, err := secretValue(row, key)
	if err != nil {
		return "", err
	}
	value, err := secureconfig.DecryptSettingValue(key, encrypted)
	if err != nil {
		return "", err
	}

	c.mu.Lock()
	if c.secrets != nil && c.gen == gen {
		c.secrets[key] = value
	}
	c.mu.Unlock()
	return value, nil
}

func (c *settingsCache) secret(db *gorm.DB, key string, now time.Time) (string, error) {
	row, gen, err := c.snapshot(db, now)
	if err != nil {
		return "", err
	}
	return c.decryptedSecret(key, row, gen)
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
	row, gen, err := cache.snapshot(db.DB, time.Now())
	if err != nil {
		return models.Settings{}, err
	}
	for _, key := range keys {
		value, err := cache.decryptedSecret(key, row, gen)
		if err != nil {
			return models.Settings{}, err
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
