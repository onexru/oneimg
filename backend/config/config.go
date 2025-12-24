package config

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

// Config 应用配置结构体
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

// 全局配置实例
var App *Config

// EnvExists 检查.env文件是否存在（若为目录则清理并返回false）
func EnvExists() bool {
	info, err := os.Stat(".env")
	// 场景1：文件不存在
	if os.IsNotExist(err) {
		return false
	}
	// 场景2：存在但不是文件（是目录），清理目录后返回false
	if err == nil && info.IsDir() {
		log.Println("发现.env是目录，正在自动清理...")
		if rmErr := os.RemoveAll(".env"); rmErr != nil {
			log.Fatalf("清理.env目录失败：%v", rmErr)
		}
		return false
	}
	// 场景3：正常文件
	return true
}

// CreateDefaultEnv 创建默认.env文件（仅当文件不存在时执行）
func CreateDefaultEnv() error {
	// 双重检查：避免并发场景下重复创建
	if EnvExists() {
		return nil
	}

	// 创建data目录（SQLite存储目录）
	if err := os.MkdirAll("./data", 0755); err != nil {
		return fmt.Errorf("创建data目录失败：%w", err)
	}

	// 生成32位随机Session密钥（base64编码）
	sessionSecret := generateRandomSecret(32)

	// 定义.env模板
	envTemplate := `# 服务器配置
SERVER_PORT=8080

# 数据库配置 - 二选一（IS_MYSQL=true则使用MySQL，否则使用SQLite）
SQLITE_PATH=./data/data.db
IS_MYSQL=false
DB_HOST=localhost
DB_PORT=3306
DB_USER=root
DB_PASSWORD=
DB_NAME=oneimgxru

# 文件上传配置
# 最大文件大小（字节）：10485760 = 10MB
MAX_FILE_SIZE=10485760
# 允许上传的文件类型（逗号分隔）
ALLOWED_TYPES=image/jpeg,image/png,image/gif,image/webp

# 默认管理员账户（首次启动初始化）
DEFAULT_USER=admin
DEFAULT_PASS=123456

# Session配置（自动生成随机值，请勿手动修改）
SESSION_SECRET=
`

	// 替换Session密钥占位符
	envContent := strings.Replace(envTemplate, "SESSION_SECRET=", "SESSION_SECRET="+sessionSecret, 1)

	// 获取当前工作目录，拼接.env路径
	wd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("获取当前工作目录失败：%w", err)
	}
	envPath := filepath.Join(wd, ".env")

	// 写入.env文件
	if err := os.WriteFile(envPath, []byte(envContent), 0644); err != nil {
		return fmt.Errorf("写入.env文件失败：%w", err)
	}

	log.Printf("✅ 首次启动：自动生成默认.env文件（路径：%s）", envPath)
	return nil
}

// generateRandomSecret 生成指定长度的随机密钥（base64 URL安全编码）
func generateRandomSecret(length int) string {
	bytes := make([]byte, length)
	// 读取加密安全的随机数
	n, err := rand.Read(bytes)
	if err != nil {
		log.Fatalf("生成随机密钥失败：%v", err)
	}
	if n != length {
		log.Fatalf("生成随机密钥长度不足：期望%d字节，实际%d字节", length, n)
	}
	return base64.URLEncoding.EncodeToString(bytes)
}

// NewConfig 初始化全局配置
func NewConfig() {
	// 检查.env文件，不存在则创建
	if !EnvExists() {
		if err := CreateDefaultEnv(); err != nil {
			log.Fatalf("创建默认.env文件失败：%v", err)
		}
	}

	// 加载.env文件到环境变量
	if err := godotenv.Load(); err != nil {
		log.Fatalf("加载.env文件失败：%v", err)
	}

	// 服务器端口：优先读SERVER_PORT，其次PORT，默认8080
	port := getEnv("SERVER_PORT", getEnv("PORT", "8080"))

	// SQLite配置
	sqlitePath := getEnv("SQLITE_PATH", "./data/data.db")
	// 确保SQLite目录存在（防止路径不存在导致数据库初始化失败）
	if err := os.MkdirAll(filepath.Dir(sqlitePath), 0755); err != nil {
		log.Printf("⚠️ 警告：创建SQLite目录失败（%s）：%v", filepath.Dir(sqlitePath), err)
	}

	// MySQL配置
	isMysql := getEnv("IS_MYSQL", "false") == "true"
	dbHost := getEnv("DB_HOST", "localhost")
	dbPort, _ := strconv.Atoi(getEnv("DB_PORT", "3306"))
	dbUser := getEnv("DB_USER", "root")
	dbPassword := getEnv("DB_PASSWORD", "")
	dbName := getEnv("DB_NAME", "oneimgxru")

	// 文件上传配置
	maxFileSize, err := strconv.ParseInt(getEnv("MAX_FILE_SIZE", "10485760"), 10, 64)
	if err != nil || maxFileSize <= 0 {
		log.Printf("MAX_FILE_SIZE配置无效（%s），使用默认值10MB", getEnv("MAX_FILE_SIZE", "10485760"))
		maxFileSize = 10485760
	}
	allowedTypes := strings.Split(getEnv("ALLOWED_TYPES", "image/jpeg,image/png,image/gif,image/webp"), ",")
	// 清理空的文件类型（防止配置错误）
	allowedTypes = cleanEmptyStrings(allowedTypes)

	// 默认用户配置
	defaultUser := getEnv("DEFAULT_USER", "admin")
	defaultPass := getEnv("DEFAULT_PASS", "123456")

	// JWT配置：优先读环境变量，无则生成随机密钥（每次启动无配置时都会生成新的，生产环境建议手动配置）
	jwtSecret := getEnv("JWT_SECRET", generateRandomSecret(32))

	// Session配置：优先读环境变量，无则生成随机密钥
	sessionSecret := getEnv("SESSION_SECRET", generateRandomSecret(32))

	// 初始化全局配置实例
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

	// 日志提示当前使用的数据库类型
	if App.IsMysql {
		log.Printf("数据库配置：使用MySQL（%s:%d/%s）", App.DbHost, App.DbPort, App.DbName)
	} else {
		log.Printf("数据库配置：使用SQLite（路径：%s）", App.SqlitePath)
	}
	log.Println("✅ 应用配置初始化完成")
}

// getEnv 获取环境变量，若不存在则返回默认值
func getEnv(key, defaultValue string) string {
	value := os.Getenv(key)
	if strings.TrimSpace(value) == "" {
		return defaultValue
	}
	return value
}

func cleanEmptyStrings(slice []string) []string {
	var result []string
	for _, s := range slice {
		trimmed := strings.TrimSpace(s)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}
