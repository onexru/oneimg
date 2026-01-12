package app

import (
	"log"

	"oneimg/backend/config"
	"oneimg/backend/database"
	"oneimg/backend/models"
	"oneimg/backend/utils/images"

	"golang.org/x/crypto/bcrypt"
)

type System struct {
	Config   *config.Config
	Database *database.Database
}

func Init() *System {
	// 生成默认.env文件
	if !config.EnvExists() {
		config.CreateDefaultEnv()
	}
	// 创建配置实例
	config.NewConfig()
	cfg := config.App
	// 初始化数据库
	database.InitDB(cfg)
	// 获取数据库实例
	db := database.GetDB()

	// 初始化图片服务
	images.InitImageService()

	// 初始化默认用户
	InitDefaultUser(cfg, db)

	// 初始化系统设置
	InitSettings(db)

	// 初始化默认存储配置
	InitDefaultStorage(db)

	r := &System{
		Config:   cfg,
		Database: db,
	}

	return r
}

// hashPassword 对密码进行加密
func hashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(bytes), err
}

// InitDefaultUser 初始化默认用户
func InitDefaultUser(cfg *config.Config, db *database.Database) {
	// 检查是否已存在用户
	var count int64
	db.DB.Model(&models.User{}).Count(&count)

	if count > 0 {
		log.Println("用户已存在，跳过默认用户初始化")
		return
	}

	// 从配置中读取默认用户信息
	defaultUsername := cfg.DefaultUser
	defaultPassword := cfg.DefaultPass

	// 加密密码
	hashedPassword, err := hashPassword(defaultPassword)
	if err != nil {
		log.Fatal("密码加密失败:", err)
	}

	// 创建默认用户
	defaultUser := models.User{
		Username: defaultUsername,
		Role:     1,
		Password: hashedPassword,
	}

	result := db.DB.Create(&defaultUser)
	if result.Error != nil {
		log.Fatal("创建默认用户失败:", result.Error)
	}

	log.Printf("默认用户创建成功 - 用户名: %s, 默认密码: %s", defaultUser.Username, defaultPassword)
}

// InitDefaultStorage 初始化默认存储配置
func InitDefaultStorage(db *database.Database) {
	// 默认使用本地存储
	storageType := "default"
	// 默认路径
	storagePath := "/uploads"

	// 查询是否已存在存储配置
	var count int64
	db.DB.Model(&models.Buckets{}).Where("type = ?", storageType).Count(&count)

	if count > 0 {
		log.Println("存储配置已存在，跳过默认存储初始化")
		return
	}

	Config := map[string]any{
		"storagePath": storagePath,
	}

	// 创建默认存储配置
	storage := models.Buckets{
		Id:       1,
		Name:     "本地默认存储",
		Type:     storageType,
		Capacity: 0, // 无限容量
		Config:   Config,
		Usage:    0,
	}

	result := db.DB.Create(&storage)
	if result.Error != nil {
		log.Fatal("创建默认存储配置失败:", result.Error)
	}

	log.Printf("默认存储配置创建成功 - 存储类型: %s, 存储路径: %s", storage.Type, storagePath)

	// 版本存储迁移
	Migrate(db)
}

func InitSettings(db *database.Database) {
	var count int64
	db.DB.Model(&models.Settings{}).Count(&count)
	if count > 0 {
		log.Println("系统配置已存在，跳过系统配置初始化")
		return
	}
	result := db.DB.Create(&models.Settings{})
	if result.Error != nil {
		log.Fatal("创建系统配置失败:", result.Error)
	}
	log.Printf("系统配置创建成功")
}
