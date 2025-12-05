package database

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"oneimg/backend/config"
	"oneimg/backend/models"

	"gorm.io/driver/mysql"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// 数据库操作类，支持Mysql+SQLite3
type Database struct {
	DB *gorm.DB
}

var db *Database

// NewDB 创建新的数据库连接
func NewDB(dialector gorm.Dialector) (*Database, error) {
	gormDB, err := gorm.Open(dialector, &gorm.Config{})
	if err != nil {
		return nil, err
	}

	return &Database{DB: gormDB}, nil
}

// GetDB 获取数据库实例
func GetDB() *Database {
	return db
}

// InitDB 初始化数据库连接
func InitDB(cfg *config.Config) {
	var err error
	var dialector gorm.Dialector

	// 根据配置选择数据库类型
	if cfg.IsMysql {
		// 使用 MySQL
		dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=True&loc=Local",
			cfg.DbUser,
			cfg.DbPassword,
			cfg.DbHost,
			cfg.DbPort,
			cfg.DbName)
		dialector = mysql.Open(dsn)
		log.Println("使用 MySQL 数据库")
	} else {
		// 使用 SQLite
		// 检查路径是否存在
		ensureDirExists(cfg.SqlitePath)
		dialector = sqlite.Open(cfg.SqlitePath)
		log.Printf("使用 SQLite 数据库: %s", cfg.SqlitePath)
	}

	db, err = NewDB(dialector)
	if err != nil {
		log.Fatal("数据库连接失败:", err)
	}

	log.Println("数据库连接成功")

	// 自动迁移数据表
	err = db.DB.AutoMigrate(&models.User{}, &models.Image{}, &models.Settings{})
	if err != nil {
		log.Fatal("数据库迁移失败:", err)
	}

	log.Println("数据库表迁移完成")
}

// 辅助函数，如果数据库目录不存在则创建
func ensureDirExists(path string) {
	dir := filepath.Dir(path)
	// 检查目录状态
	_, err := os.Stat(dir)
	if os.IsNotExist(err) {
		// 创建目录（0755：生产环境安全权限）
		if err := os.MkdirAll(dir, 0755); err != nil {
			log.Fatalf("创建数据库目录失败 [路径: %s]: %v", dir, err)
		}
		return
	}

	// 处理其他错误（如权限不足）
	if err != nil {
		log.Fatalf("检查数据库目录失败 [路径: %s]: %v", dir, err)
	}
}
