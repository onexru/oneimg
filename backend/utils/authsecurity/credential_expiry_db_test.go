package authsecurity

import (
	"errors"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"oneimg/backend/models"
)

// newCredentialDB 建一个只含凭据相关表的内存库。这些用例要验证的是
// 「配置存进去之后还能不能按原来的方式用」，所以必须真的落库，
// 只测纯函数证明不了兼容性。
func newCredentialDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{Logger: logger.Discard})
	if err != nil {
		t.Fatalf("打开内存数据库失败: %v", err)
	}
	if err := db.AutoMigrate(&models.APICredential{}, &models.AuthState{}, &models.Settings{}); err != nil {
		t.Fatalf("迁移失败: %v", err)
	}
	if err := EnsureState(db); err != nil {
		t.Fatalf("初始化 AuthState 失败: %v", err)
	}
	// 每个用例都从空表开始，避免共享内存库互相污染。
	if err := db.Where("1 = 1").Delete(&models.APICredential{}).Error; err != nil {
		t.Fatalf("清理凭据失败: %v", err)
	}
	return db
}

func mustExpiry(t *testing.T, spec string) TokenExpiry {
	t.Helper()
	expiry, err := ParseTokenExpiry(spec)
	if err != nil {
		t.Fatalf("解析 %q 失败: %v", spec, err)
	}
	return expiry
}

// 不传有效期时行为必须和改动前完全一致：30 天、能正常校验通过。
func TestNewAPICredentialDefaultsToLegacyLifetime(t *testing.T) {
	db := newCredentialDB(t)
	raw, row, err := NewAPICredential(db, models.SuperAdminID, mustExpiry(t, ""))
	if err != nil {
		t.Fatalf("生成凭据失败: %v", err)
	}
	if row.NeverExpires {
		t.Error("默认配置不应是永久有效")
	}
	if row.ExpirySpec != "30d" {
		t.Errorf("默认 ExpirySpec = %q, 期望 30d", row.ExpirySpec)
	}
	if remaining := time.Until(row.ExpiresAt); remaining < 29*24*time.Hour || remaining > 31*24*time.Hour {
		t.Errorf("默认有效期 = %v, 期望约 30 天", remaining)
	}
	if _, err := ValidateAPICredential(db, models.Settings{StartAPI: true}, raw); err != nil {
		t.Errorf("默认凭据应当可用，实际 %v", err)
	}
}

func TestNewAPICredentialHonoursConfiguredExpiry(t *testing.T) {
	db := newCredentialDB(t)
	_, row, err := NewAPICredential(db, models.SuperAdminID, mustExpiry(t, "7d"))
	if err != nil {
		t.Fatalf("生成凭据失败: %v", err)
	}
	if row.ExpirySpec != "7d" {
		t.Errorf("ExpirySpec = %q, 期望 7d", row.ExpirySpec)
	}
	if remaining := time.Until(row.ExpiresAt); remaining < 6*24*time.Hour || remaining > 8*24*time.Hour {
		t.Errorf("有效期 = %v, 期望约 7 天", remaining)
	}
}

// 永久有效的凭据既要「不过期」，也不能破坏按 expires_at 做的历史查询。
func TestUpdateAPICredentialExpiryToPermanentKeepsCredentialUsable(t *testing.T) {
	db := newCredentialDB(t)
	raw, created, err := NewAPICredential(db, models.SuperAdminID, mustExpiry(t, "30d"))
	if err != nil {
		t.Fatalf("生成凭据失败: %v", err)
	}
	updated, err := UpdateAPICredentialExpiry(db, models.SuperAdminID, mustExpiry(t, "permanent"))
	if err != nil {
		t.Fatalf("更新有效期失败: %v", err)
	}
	if updated.ID != created.ID {
		t.Error("更新有效期不应更换凭据本身")
	}
	if !updated.NeverExpires || updated.ExpirySpec != "permanent" {
		t.Errorf("更新后 never_expires = %v, spec = %q", updated.NeverExpires, updated.ExpirySpec)
	}
	if !updated.ExpiresAt.Equal(PermanentExpiresAt) {
		t.Errorf("永久有效的 expires_at = %v, 期望 %v", updated.ExpiresAt, PermanentExpiresAt)
	}
	// 关键兼容点：凭据原文没变，仍然能通过中间件校验。
	if _, err := ValidateAPICredential(db, models.Settings{StartAPI: true}, raw); err != nil {
		t.Errorf("改成永久后凭据应当仍可用，实际 %v", err)
	}

	// 再改回有限时长同样生效，说明这不是单向开关。
	back, err := UpdateAPICredentialExpiry(db, models.SuperAdminID, mustExpiry(t, "7d"))
	if err != nil {
		t.Fatalf("改回 7d 失败: %v", err)
	}
	if back.NeverExpires || back.ExpirySpec != "7d" {
		t.Errorf("改回后 never_expires = %v, spec = %q", back.NeverExpires, back.ExpirySpec)
	}
}

// 已过期的凭据不允许「改配置复活」，否则撤销语义会被一次编辑绕过。
func TestUpdateAPICredentialExpiryRejectsExpiredCredential(t *testing.T) {
	db := newCredentialDB(t)
	_, created, err := NewAPICredential(db, models.SuperAdminID, mustExpiry(t, "7d"))
	if err != nil {
		t.Fatalf("生成凭据失败: %v", err)
	}
	// 直接把时间推到过去，模拟自然过期。
	if err := db.Model(&models.APICredential{}).Where("id = ?", created.ID).
		Update("expires_at", time.Now().Add(-time.Hour)).Error; err != nil {
		t.Fatalf("构造过期凭据失败: %v", err)
	}
	if _, err := UpdateAPICredentialExpiry(db, models.SuperAdminID, mustExpiry(t, "permanent")); !errors.Is(err, ErrCredentialExpired) {
		t.Errorf("过期凭据应拒绝续期，实际 %v", err)
	}
}

func TestUpdateAPICredentialExpiryWithoutCredential(t *testing.T) {
	db := newCredentialDB(t)
	if _, err := UpdateAPICredentialExpiry(db, 42, mustExpiry(t, "30d")); !errors.Is(err, ErrNoActiveCredential) {
		t.Errorf("无凭据时应返回 ErrNoActiveCredential，实际 %v", err)
	}
}

// 重新生成仍然只保留一条有效凭据，且新凭据使用新配置的有效期。
func TestRegenerateRevokesPreviousAndAppliesExpiry(t *testing.T) {
	db := newCredentialDB(t)
	oldRaw, _, err := NewAPICredential(db, models.SuperAdminID, mustExpiry(t, "30d"))
	if err != nil {
		t.Fatalf("生成旧凭据失败: %v", err)
	}
	_, fresh, err := NewAPICredential(db, models.SuperAdminID, mustExpiry(t, "permanent"))
	if err != nil {
		t.Fatalf("生成新凭据失败: %v", err)
	}
	if !fresh.NeverExpires {
		t.Error("新凭据应当是永久有效")
	}
	if _, err := ValidateAPICredential(db, models.Settings{StartAPI: true}, oldRaw); err == nil {
		t.Error("旧凭据应当已失效")
	}
	var active int64
	if err := db.Model(&models.APICredential{}).Where("revoked = ?", false).Count(&active).Error; err != nil {
		t.Fatalf("统计有效凭据失败: %v", err)
	}
	if active != 1 {
		t.Errorf("有效凭据数量 = %d, 期望 1", active)
	}
}

// 老库升级路径：本次新增了 never_expires / expiry_spec 两列，必须能直接
// AutoMigrate 补上，并且既有凭据升级后依然可用、依然保留原到期时间。
// 这是「不影响现有 API KEY 兼容性」最直接的证据。
func TestAutoMigrateUpgradesLegacyCredentialTable(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:legacy-upgrade?mode=memory&cache=shared"), &gorm.Config{Logger: logger.Discard})
	if err != nil {
		t.Fatalf("打开内存数据库失败: %v", err)
	}
	// 手写一张改动之前的表结构，模拟线上已存在的库。
	if err := db.Exec(`CREATE TABLE api_credentials (
		id text PRIMARY KEY,
		owner_id integer,
		scopes text,
		expires_at datetime,
		last_used_at datetime,
		revoked numeric,
		legacy numeric,
		created_at datetime
	)`).Error; err != nil {
		t.Fatalf("创建旧表失败: %v", err)
	}
	raw := "oi_legacycredentialvalue"
	if err := db.Exec(`INSERT INTO api_credentials (id, owner_id, scopes, expires_at, last_used_at, revoked, legacy, created_at)
		VALUES (?, ?, ?, ?, NULL, 0, 0, ?)`,
		Digest(raw), models.SuperAdminID, DefaultTokenScopes, time.Now().Add(24*time.Hour), time.Now()).Error; err != nil {
		t.Fatalf("写入旧凭据失败: %v", err)
	}

	if err := db.AutoMigrate(&models.APICredential{}); err != nil {
		t.Fatalf("升级迁移失败: %v", err)
	}

	var upgraded models.APICredential
	if err := db.First(&upgraded, "id = ?", Digest(raw)).Error; err != nil {
		t.Fatalf("读取升级后的凭据失败: %v", err)
	}
	if upgraded.NeverExpires {
		t.Error("旧凭据不应被自动升级成永久有效")
	}
	if upgraded.ExpirySpec != "" {
		t.Errorf("旧凭据的 expiry_spec 应为空，实际 %q", upgraded.ExpirySpec)
	}
	if _, err := ValidateAPICredential(db, models.Settings{StartAPI: true}, raw); err != nil {
		t.Errorf("升级后旧凭据应当仍可用，实际 %v", err)
	}
}
