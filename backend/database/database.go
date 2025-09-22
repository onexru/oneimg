package database

import (
	"fmt"
	"log"

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
		dialector = sqlite.Open(cfg.SqlitePath)
		log.Printf("使用 SQLite 数据库: %s", cfg.SqlitePath)
	}

	db, err = NewDB(dialector)
	if err != nil {
		log.Fatal("数据库连接失败:", err)
	}

	log.Println("数据库连接成功")

	// 自动迁移数据表
	err = db.DB.AutoMigrate(&models.User{}, &models.Image{})
	if err != nil {
		log.Fatal("数据库迁移失败:", err)
	}

	log.Println("数据库表迁移完成")
}
