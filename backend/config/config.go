package config

import (
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

type Config struct {
	// 服务器配置
	Port string

	// Sqlite3数据库
	SqlitePath string

	// Mysql数据库
	IsMysql    bool
	DbHost     string
	DbPort     int
	DbUser     string
	DbPassword string
	DbName     string

	// 上传文件配置
	MaxFileSize  int64
	AllowedTypes []string

	// 默认用户
	DefaultUser string
	DefaultPass string

	// JWT配置
	JWTSecret string

	// Session配置
	SessionSecret string
}

// 设置全局
var App *Config

func NewConfig() {

	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}

	maxFileSize, _ := strconv.ParseInt(getEnv("MAX_FILE_SIZE", "10485760"), 10, 64)
	allowedTypes := strings.Split(getEnv("ALLOWED_TYPES", "image/jpeg,image/png,image/gif"), ",")
	// 端口
	port := getEnv("SERVER_PORT", getEnv("PORT", "8080"))

	// Sqlite3数据库
	sqlitePath := getEnv("SQLITE_PATH", "./data/data.db")

	// Mysql数据库
	isMysql := getEnv("IS_MYSQL", "false") == "true"
	dbHost := getEnv("DB_HOST", "localhost")
	dbPort, _ := strconv.Atoi(getEnv("DB_PORT", "3306"))
	dbUser := getEnv("DB_USER", "root")
	dbPassword := getEnv("DB_PASSWORD", "")
	dbName := getEnv("DB_NAME", "oneimgxru")

	// 默认用户
	defaultUser := getEnv("DEFAULT_USER", "admin")
	defaultPass := getEnv("DEFAULT_PASS", "123456")

	// JWT配置
	jwtSecret := getEnv("JWT_SECRET", "your-secret-key-change-this-in-production")

	// Session配置
	sessionSecret := getEnv("SESSION_SECRET", "your-session-secret-key-change-this-in-production")

	App = &Config{
		Port:          port,
		SqlitePath:    sqlitePath,
		IsMysql:       isMysql,
		DbHost:        dbHost,
		DbPort:        dbPort,
		DbUser:        dbUser,
		DbPassword:    dbPassword,
		DbName:        dbName,
		MaxFileSize:   maxFileSize,
		AllowedTypes:  allowedTypes,
		DefaultUser:   defaultUser,
		DefaultPass:   defaultPass,
		JWTSecret:     jwtSecret,
		SessionSecret: sessionSecret,
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
