package database

import (
	"crypto/x509"
	"database/sql"
	"fmt"
	"oneimg/backend/config"
	"os"
	"path/filepath"
	"time"
)

func configurePool(db *sql.DB, cfg *config.Config, dialect string) {
	open, idle, lifetime, idleTime := 10, 2, 30*time.Minute, 5*time.Minute
	if cfg != nil {
		if cfg.DbMaxOpenConns > 0 {
			open = cfg.DbMaxOpenConns
		}
		if cfg.DbMaxIdleConns >= 0 {
			idle = cfg.DbMaxIdleConns
		}
		if cfg.DbConnMaxLifetime > 0 {
			lifetime = cfg.DbConnMaxLifetime
		}
		if cfg.DbConnMaxIdleTime > 0 {
			idleTime = cfg.DbConnMaxIdleTime
		}
	}
	// SQLite serializes writers; :memory: connections must share the same DB.
	if dialect == "sqlite" {
		open, idle, lifetime, idleTime = 1, 1, 0, 0
	}
	if idle > open {
		idle = open
	}
	db.SetMaxOpenConns(open)
	db.SetMaxIdleConns(idle)
	db.SetConnMaxLifetime(lifetime)
	db.SetConnMaxIdleTime(idleTime)
}

func validateCAFile(path string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	data, err := os.ReadFile(abs)
	if err != nil {
		return "", fmt.Errorf("读取配置的数据库 CA 失败: %w", err)
	}
	if !x509.NewCertPool().AppendCertsFromPEM(data) {
		return "", fmt.Errorf("数据库 CA 不是有效 PEM 证书")
	}
	return abs, nil
}
