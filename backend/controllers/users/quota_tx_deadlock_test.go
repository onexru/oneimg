package users

import (
	"path/filepath"
	"testing"
	"time"

	"gorm.io/gorm"

	"oneimg/backend/config"
	"oneimg/backend/database"
	"oneimg/backend/models"
)

// TestEnforceQuotaInTxNoSelfDeadlock 复现并锁死 v4.0.0 的连接池自锁事故。
//
// 事故形状（三处叠加，缺一不可）：
//  1. SQLite 下连接池被硬编码成 1 条连接（backend/database/pool.go）；
//  2. 上传流程先把文件同步直传目标桶（Telegram 桶要等 api.telegram.org，数秒）
//     再开写库事务，事务体里调 EnforceQuotaInTx；
//  3. 整条请求耗时跨过 settings 的 5 秒缓存 TTL 后缓存已冷，于是
//     EnforceQuotaInTx 里的 settings.GetSettings() 会**再向连接池申请一条连接** ——
//     而池里唯一那条正被调用者自己的事务攥着 → 永久自锁（database/sql 在
//     context.Background() 下等连接没有超时）。自锁期间还持有 settingsCache.mu，
//     middlewares.buildCSP 会把每一个请求（含静态资源）一起堵死，全站表现为
//     Cloudflare 524，而容器 CPU 为 0。
//
// 本测试用生产同一套初始化（database.InitDB，SQLite 落在 t.TempDir）拿到同一个
// 「池只有 1 条连接」的环境，因此：
//   - 修复前（EnforceQuotaInTx 内调 settings.GetSettings()）：3 秒看门狗处失败；
//   - 修复后（改用事务自己的连接 tx 读 settings）：毫秒级返回。
func TestEnforceQuotaInTxNoSelfDeadlock(t *testing.T) {
	database.InitDB(&config.Config{
		DbType:     "sqlite",
		SqlitePath: filepath.Join(t.TempDir(), "quota_deadlock_test.db"),
	})
	db := database.GetDB().DB

	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("取 *sql.DB 失败: %v", err)
	}
	// 先确认「池只有 1 条连接」这个前提确实成立，否则本测试证明不了任何东西。
	if got := sqlDB.Stats().MaxOpenConnections; got != 1 {
		t.Fatalf("前提不成立：SQLite 下连接池应为 1 条，实际 %d 条", got)
	}

	// settings 单行必须存在（InitDB 里的 EnsureSettingsSingleton 会建，这里兜底）。
	var setting models.Settings
	if err := db.First(&setting, 1).Error; err != nil {
		if err := db.Create(&models.Settings{ID: 1}).Error; err != nil {
			t.Fatalf("创建 settings 单行失败: %v", err)
		}
	}

	user := models.User{Username: "quota-deadlock-test", Password: "x", Role: models.RoleUser}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("创建测试用户失败: %v", err)
	}

	// 调用形状与 LegacyUpload.go 一致：在写库事务体内调用。
	done := make(chan error, 1)
	go func() {
		done <- db.Transaction(func(tx *gorm.DB) error {
			return EnforceQuotaInTx(tx, user.ID, 10)
		})
	}()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("EnforceQuotaInTx 返回错误: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("自锁：EnforceQuotaInTx 在事务内 3 秒仍未返回（池里唯一连接已被本事务占用，它却回池里再要一条）")
	}
}
