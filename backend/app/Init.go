package app

import (
	"log"

	"oneimg/backend/config"
	"oneimg/backend/database"
	"oneimg/backend/models"
	"oneimg/backend/services"
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

	images.InitImageService()
	InitDefaultUser(cfg, db)
	InitSettings(db)
	InitDefaultStorage(db)

	// 为旧图片补齐存储副本记录（幂等），再启动同步 worker。
	if err := services.BackfillImageStorages(); err != nil {
		log.Printf("图片存储副本回填失败: %v", err)
	}
	services.StartStorageSyncWorker()

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

// InitDefaultUser 在空库时创建默认管理员。
func InitDefaultUser(cfg *config.Config, db *database.Database) {
	var count int64
	db.DB.Model(&models.User{}).Count(&count)
	if count > 0 {
		log.Println("用户已存在，跳过默认用户初始化")
		return
	}

	hashedPassword, err := hashPassword(cfg.DefaultPass)
	if err != nil {
		log.Fatal("密码加密失败:", err)
	}

	defaultUser := models.User{
		Username: cfg.DefaultUser,
		Role:     models.RoleAdmin,
		Password: hashedPassword,
	}
	if result := db.DB.Create(&defaultUser); result.Error != nil {
		log.Fatal("创建默认用户失败:", result.Error)
	}
	log.Printf("默认用户创建成功 - 用户名: %s", defaultUser.Username)
}

// InitDefaultStorage 在空库时创建本地默认存储桶，并执行版本迁移。
func InitDefaultStorage(db *database.Database) {
	const storageType = "default"
	const storagePath = "/uploads"

	var count int64
	db.DB.Model(&models.Buckets{}).Where("type = ?", storageType).Count(&count)
	if count > 0 {
		log.Println("存储配置已存在，跳过默认存储初始化")
		return
	}

	storage := models.Buckets{
		Id:       1,
		Name:     "本地默认存储",
		Type:     storageType,
		Capacity: 0,
		Config: map[string]any{
			"storagePath": storagePath,
		},
		Usage: 0,
	}
	if result := db.DB.Create(&storage); result.Error != nil {
		log.Fatal("创建默认存储配置失败:", result.Error)
	}
	log.Printf("默认存储配置创建成功 - 类型: %s, 路径: %s", storage.Type, storagePath)

	Migrate(db)
}

// InitSettings 在空库时创建默认系统设置行。
func InitSettings(db *database.Database) {
	var count int64
	db.DB.Model(&models.Settings{}).Count(&count)
	if count > 0 {
		log.Println("系统配置已存在，跳过系统配置初始化")
		return
	}
	if result := db.DB.Create(&models.Settings{}); result.Error != nil {
		log.Fatal("创建系统配置失败:", result.Error)
	}
	log.Printf("系统配置创建成功")
}
