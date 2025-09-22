package app

import (
	"log"

	"oneimg/backend/config"
	"oneimg/backend/database"
	"oneimg/backend/models"
	"oneimg/backend/services"

	"golang.org/x/crypto/bcrypt"
)

type System struct {
	Config   *config.Config
	Database *database.Database
}

func Init() *System {
	// 创建配置实例
	config.NewConfig()
	cfg := config.App
	// 初始化数据库
	database.InitDB(cfg)
	// 获取数据库实例
	db := database.GetDB()

	// 初始化图片服务
	services.InitImageService()

	// 初始化默认用户
	InitDefaultUser(cfg, db)

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
		Password: hashedPassword,
	}

	result := db.DB.Create(&defaultUser)
	if result.Error != nil {
		log.Fatal("创建默认用户失败:", result.Error)
	}

	log.Printf("默认用户创建成功 - 用户名: %s, 默认密码: %s", defaultUser.Username, defaultPassword)
}
