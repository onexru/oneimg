package app

import (
	"fmt"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"log"
	"oneimg/backend/utils/settings"

	"oneimg/backend/config"
	"oneimg/backend/database"
	"oneimg/backend/models"
	"oneimg/backend/services"
	"oneimg/backend/utils/authsecurity"
	"oneimg/backend/utils/images"

	"golang.org/x/crypto/bcrypt"
)

// System 应用运行时核心依赖。
type System struct {
	Config   *config.Config
	Database *database.Database
}

// Init 加载配置、数据库、默认数据与后台任务。
func Init() *System {
	if !config.EnvExists() {
		config.CreateDefaultEnv()
	}
	config.NewConfig()
	cfg := config.App

	database.InitDB(cfg)
	db := database.GetDB()

	if err := InitializeDefaults(cfg, db.DB); err != nil {
		log.Fatalf("默认数据初始化失败: %v", err)
	}
	if err := settings.MigrateSecrets(db.DB); err != nil {
		log.Fatalf("敏感配置迁移失败: %v", err)
	}
	if err := authsecurity.EnsureState(db.DB); err != nil {
		log.Fatalf("认证状态初始化失败: %v", err)
	}
	var storedSettings models.Settings
	if err := db.DB.First(&storedSettings, 1).Error; err != nil {
		log.Fatalf("认证配置读取失败: %v", err)
	}
	if err := authsecurity.EnrollLegacyToken(db.DB, storedSettings); err != nil {
		log.Fatalf("旧 API 凭据迁移失败: %v", err)
	}
	if err := settings.RegisterInvalidation(db.DB); err != nil {
		log.Fatalf("系统配置缓存初始化失败: %v", err)
	}
	images.InitImageService()

	// 为旧图片补齐存储副本记录（幂等），再启动同步 worker。
	if err := services.BackfillImageStorages(); err != nil {
		log.Fatalf("图片存储副本回填失败: %v", err)
	}
	if err := database.ReconcileLegacyBucketUsage(db.DB); err != nil {
		log.Fatalf("历史存储容量核对失败: %v", err)
	}
	services.StartStorageSyncWorker()
	services.StartDirectUploadWorker()

	return &System{
		Config:   cfg,
		Database: db,
	}
}

// hashPassword 使用 bcrypt 加密密码。
func hashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(bytes), err
}

// InitializeDefaults seeds only missing defaults in one transaction. It never
// resets an existing settings row, storage configuration or image ownership.
func InitializeDefaults(cfg *config.Config, db *gorm.DB) error {
	return db.Transaction(func(tx *gorm.DB) error {
		if err := initDefaultUser(cfg, tx); err != nil {
			return err
		}
		if err := initSettings(tx); err != nil {
			return err
		}
		if err := initDefaultStorage(tx); err != nil {
			return err
		}
		return migrateLegacyData(tx)
	})
}

func initDefaultUser(cfg *config.Config, tx *gorm.DB) error {
	var count int64
	if err := tx.Model(&models.User{}).Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return nil
	}
	initialPassword, err := config.InitialAdminPassword(cfg)
	if err != nil {
		return err
	}
	password, err := hashPassword(initialPassword)
	if err != nil {
		return err
	}
	return tx.Create(&models.User{Username: cfg.DefaultUser, Role: models.RoleAdmin, Password: password}).Error
}
func initSettings(tx *gorm.DB) error {
	return tx.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "id"}}, DoNothing: true}).Create(&models.Settings{ID: 1}).Error
}
func initDefaultStorage(tx *gorm.DB) error {
	var count int64
	if err := tx.Model(&models.Buckets{}).Where("type = ?", "default").Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return nil
	}
	var occupied int64
	if err := tx.Model(&models.Buckets{}).Where("id = ?", 1).Count(&occupied).Error; err != nil {
		return err
	}
	if occupied > 0 {
		return fmt.Errorf("bucket ID=1 is occupied by nonlocal storage; refusing to overwrite")
	}
	storage := models.Buckets{Id: 1, Name: "本地默认存储", Type: "default", Capacity: 0, Config: map[string]any{"storagePath": config.UploadRoot()}, Usage: 0}
	if err := tx.Create(&storage).Error; err != nil {
		return err
	}
	return database.AdvanceBucketSequence(tx)
}

// Compatibility entrypoints retain their names, but errors are never ignored.
func InitDefaultUser(cfg *config.Config, db *database.Database) {
	if err := db.DB.Transaction(func(tx *gorm.DB) error { return initDefaultUser(cfg, tx) }); err != nil {
		log.Fatal("默认用户初始化失败: ", err)
	}
}
func InitDefaultStorage(db *database.Database) {
	if err := db.DB.Transaction(func(tx *gorm.DB) error {
		if err := initDefaultStorage(tx); err != nil {
			return err
		}
		return migrateLegacyData(tx)
	}); err != nil {
		log.Fatal("默认存储初始化失败: ", err)
	}
}
func InitSettings(db *database.Database) {
	if err := db.DB.Transaction(initSettings); err != nil {
		log.Fatal("系统设置初始化失败: ", err)
	}
	settings.Invalidate()
}
