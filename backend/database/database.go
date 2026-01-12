package database

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"log"
	"net/url"
	"os"
	"path/filepath"

	"oneimg/backend/config"
	"oneimg/backend/models"

	// MySQL底层驱动
	sqlmysql "github.com/go-sql-driver/mysql"
	// GORM驱动
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// Database 数据库操作类
type Database struct {
	DB *gorm.DB
}

var db *Database

// NewDB 创建新的数据库连接
func NewDB(dialector gorm.Dialector) (*Database, error) {
	gormConfig := &gorm.Config{
		SkipDefaultTransaction:                   true,
		DisableForeignKeyConstraintWhenMigrating: true,
	}

	gormDB, err := gorm.Open(dialector, gormConfig)
	if err != nil {
		return nil, fmt.Errorf("gorm连接失败: %w", err)
	}

	sqlDB, err := gormDB.DB()
	if err != nil {
		return nil, fmt.Errorf("获取SQL连接失败: %w", err)
	}

	// 验证连接
	if err := sqlDB.Ping(); err != nil {
		return nil, fmt.Errorf("连接验证失败: %w", err)
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

	switch cfg.DbType {
	case "mysql":
		dialector, err = initMysqlWithTLS(cfg)
		if err != nil {
			log.Fatalf("❌ MySQL初始化失败: %v", err)
		}
		log.Println("✅ MySQL 数据库连接成功")
	case "postgres":
		dialector, err = initPostgreSQLWithTLS(cfg)
		if err != nil {
			log.Fatalf("❌ PostgreSQL初始化失败: %v", err)
		}
		log.Println("✅ PostgreSQL 数据库连接成功")
	default:
		ensureDirExists(cfg.SqlitePath)
		dialector = sqlite.Open(cfg.SqlitePath)
		log.Printf("✅ SQLite 数据库连接成功: %s", cfg.SqlitePath)
	}

	// 创建数据库实例
	db, err = NewDB(dialector)
	if err != nil {
		log.Fatalf("❌ 数据库实例创建失败: %v", err)
	}

	// 自动迁移数据表
	err = db.DB.AutoMigrate(
		&models.Tags{},
		&models.User{},
		&models.Image{},
		&models.Settings{},
		&models.ImageTeleGram{},
		&models.ImageToTags{},
		&models.Buckets{},
	)
	if err != nil {
		log.Fatalf("❌ 数据库表迁移失败: %v", err)
	}
	log.Println("✅ 数据库表迁移完成")
}

// initMysqlWithTLS 初始化MySQL
func initMysqlWithTLS(cfg *config.Config) (gorm.Dialector, error) {
	dsnTemplate := "%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=True&loc=Local&timeout=10s"
	dsn := fmt.Sprintf(dsnTemplate, cfg.DbUser, cfg.DbPassword, cfg.DbHost, cfg.DbPort, cfg.DbName)

	tlsName := "custom_tls"
	tlsConfig, err := buildTLSConfig(cfg)
	if err != nil {
		return nil, err
	}

	if err := sqlmysql.RegisterTLSConfig(tlsName, tlsConfig); err != nil {
		return nil, err
	}
	dsn += "&tls=" + tlsName

	return mysql.New(mysql.Config{DSN: dsn}), nil
}

// initPostgreSQLWithTLS 初始化 PG 数据库
func initPostgreSQLWithTLS(cfg *config.Config) (gorm.Dialector, error) {
	// 对特殊字符进行编码
	user := url.QueryEscape(cfg.DbUser)
	pass := url.QueryEscape(cfg.DbPassword)

	// 构建基础 DSN
	dsn := fmt.Sprintf("postgres://%s:%s@%s:%d/%s", user, pass, cfg.DbHost, cfg.DbPort, cfg.DbName)

	// 处理 SSL/TLS 参数
	queryParams := url.Values{}
	queryParams.Add("timezone", "Asia/Shanghai")

	if cfg.DbCaCertPath != "" && fileExists(cfg.DbCaCertPath) {
		absPath, _ := filepath.Abs(cfg.DbCaCertPath)
		if cfg.DbSkipCertVerify {
			queryParams.Add("sslmode", "require")
		} else {
			queryParams.Add("sslmode", "verify-full")
			queryParams.Add("sslrootcert", absPath)
		}
	} else {
		queryParams.Add("sslmode", "require")
	}

	dsn = dsn + "?" + queryParams.Encode()

	return postgres.New(postgres.Config{
		DSN:                  dsn,
		PreferSimpleProtocol: true, // 增加兼容性
	}), nil
}

// buildTLSConfig 构建 TLS 配置
func buildTLSConfig(cfg *config.Config) (*tls.Config, error) {
	tlsConfig := &tls.Config{
		InsecureSkipVerify: cfg.DbSkipCertVerify,
	}

	if cfg.DbCaCertPath != "" && fileExists(cfg.DbCaCertPath) {
		caCert, err := os.ReadFile(cfg.DbCaCertPath)
		if err != nil {
			return nil, fmt.Errorf("读取CA证书失败: %w", err)
		}
		caCertPool := x509.NewCertPool()
		if !caCertPool.AppendCertsFromPEM(caCert) {
			return nil, fmt.Errorf("解析CA证书失败")
		}
		tlsConfig.RootCAs = caCertPool
	}
	return tlsConfig, nil
}

func ensureDirExists(path string) {
	dir := filepath.Dir(path)
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		os.MkdirAll(dir, 0755)
	}
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return !os.IsNotExist(err)
}
