package database

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"oneimg/backend/config"
	"oneimg/backend/models"

	// MySQL底层驱动
	sqlmysql "github.com/go-sql-driver/mysql"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
	// GORM驱动
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// Database 封装 GORM 连接。
type Database struct {
	DB *gorm.DB
}

var db *Database

// NewDB 使用给定 dialector 打开并校验数据库连接。
func NewDB(dialector gorm.Dialector, configs ...*config.Config) (*Database, error) {
	gormConfig := &gorm.Config{
		SkipDefaultTransaction:                   true,
		DisableAutomaticPing:                     true,
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

	var cfg *config.Config
	if len(configs) > 0 {
		cfg = configs[0]
	}
	configurePool(sqlDB, cfg, dialector.Name())
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := sqlDB.PingContext(ctx); err != nil {
		_ = sqlDB.Close()
		return nil, fmt.Errorf("连接验证失败: %w", err)
	}

	return &Database{DB: gormDB}, nil
}

// UsageColumn 返回按当前方言正确引用的 usage 列名。
// usage 在 MySQL 中是保留字（GRANT USAGE），裸写在 SQL 表达式中会触发 1064 语法错误，
// 因此所有原生 SQL 片段都必须对 usage 做方言化引用。
func UsageColumn(db *gorm.DB) string {
	switch db.Dialector.Name() {
	case "postgres":
		return `"usage"`
	default: // mysql、sqlite 均接受反引号
		return "`usage`"
	}
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
	case "mysql", "postgres":
		mode, reason := resolveTLSMode(cfg)
		log.Printf("数据库 TLS 模式: %s（%s）", mode, reason)
		if cfg.DbType == "mysql" {
			dialector, err = initMysqlWithTLS(cfg, mode)
		} else {
			dialector, err = initPostgreSQLWithTLS(cfg, mode)
		}
		if err != nil {
			log.Fatalf("❌ 数据库初始化失败: %v", err)
		}
	default:
		ensureDirExists(cfg.SqlitePath)
		dialector = sqlite.Open(cfg.SqlitePath)
	}

	// 创建数据库实例（含连接验证）
	db, err = NewDB(dialector, cfg)
	if err != nil {
		log.Fatalf("❌ 数据库实例创建失败: %v", err)
	}
	switch cfg.DbType {
	case "mysql":
		log.Println("✅ MySQL 数据库连接成功")
	case "postgres":
		log.Println("✅ PostgreSQL 数据库连接成功")
	default:
		log.Printf("✅ SQLite 数据库连接成功: %s", cfg.SqlitePath)
	}

	// 自动迁移数据表
	err = db.DB.AutoMigrate(
		&models.Tags{},
		&models.User{},
		&models.AuthSession{},
		&models.AuthLoginEvent{},
		&models.AuthState{},
		&models.GuestIdentity{},
		&models.APICredential{},
		&models.Folder{},
		&models.Image{},
		&models.ImageStorage{},
		&models.DirectUploadTask{},
		&models.Settings{},
		&models.ExternalAuthFlow{},
		&models.ExternalIdentity{},
		&models.ImageTeleGram{},
		&models.ImageToTags{},
		&models.Buckets{},
		&models.RoleStoragePolicy{},
		&models.RandomGraph{},
	)
	if err != nil {
		log.Fatalf("❌ 数据库表迁移失败: %v", err)
	}
	if err := EnsureSettingsSingleton(db.DB); err != nil {
		log.Fatalf("❌ 系统配置单行迁移失败: %v", err)
	}
	if err := EnsureQueryIndexes(db.DB); err != nil {
		log.Fatalf("❌ 查询索引迁移失败: %v", err)
	}
	log.Println("✅ 数据库表迁移完成")
}

// initMysqlWithTLS 初始化MySQL
func initMysqlWithTLS(cfg *config.Config, mode string) (gorm.Dialector, error) {

	mysqlConfig := sqlmysql.NewConfig()
	mysqlConfig.User = cfg.DbUser
	mysqlConfig.Passwd = cfg.DbPassword
	mysqlConfig.Net = "tcp"
	mysqlConfig.Addr = net.JoinHostPort(cfg.DbHost, fmt.Sprint(cfg.DbPort))
	mysqlConfig.DBName = cfg.DbName
	mysqlConfig.ParseTime = true
	mysqlConfig.Loc = time.Local
	mysqlConfig.Timeout = 10 * time.Second
	mysqlConfig.Params = map[string]string{"charset": "utf8mb4"}

	tlsParam, err := mysqlTLSParam(cfg, mode)
	if err != nil {
		return nil, err
	}
	mysqlConfig.TLSConfig = tlsParam
	return mysql.New(mysql.Config{DSN: mysqlConfig.FormatDSN()}), nil
}

// initPostgreSQLWithTLS 初始化 PG 数据库
func initPostgreSQLWithTLS(cfg *config.Config, mode string) (gorm.Dialector, error) {
	u := &url.URL{Scheme: "postgres", User: url.UserPassword(cfg.DbUser, cfg.DbPassword), Host: net.JoinHostPort(cfg.DbHost, fmt.Sprint(cfg.DbPort)), Path: "/" + cfg.DbName}
	params := url.Values{"timezone": {"Asia/Shanghai"}, "connect_timeout": {"10"}}
	sslParams, err := postgresSSLParams(cfg, mode)
	if err != nil {
		return nil, err
	}
	for k, v := range sslParams {
		params.Set(k, v)
	}
	// GORM's timezone matcher reads the raw query before URL decoding. Keep
	// the zone's slash literal (valid in a query), without unescaping secrets.
	u.RawQuery = strings.Replace(params.Encode(), "timezone=Asia%2FShanghai", "timezone=Asia/Shanghai", 1)

	// sslmode 无法指定校验主机名，需要时改由 pgx 配置覆盖 ServerName。
	if mode == TLSModeVerifyFull && cfg.DbTLSServerName != "" && cfg.DbTLSServerName != cfg.DbHost {
		pgxConfig, err := pgx.ParseConfig(u.String())
		if err != nil {
			return nil, err
		}
		pgxConfig.DefaultQueryExecMode = pgx.QueryExecModeSimpleProtocol
		if pgxConfig.TLSConfig != nil {
			pgxConfig.TLSConfig.ServerName = cfg.DbTLSServerName
		}
		for _, fb := range pgxConfig.Fallbacks {
			if fb.TLSConfig != nil {
				fb.TLSConfig.ServerName = cfg.DbTLSServerName
			}
		}
		return postgres.New(postgres.Config{Conn: stdlib.OpenDB(*pgxConfig)}), nil
	}
	return postgres.New(postgres.Config{DSN: u.String(), PreferSimpleProtocol: true}), nil
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
