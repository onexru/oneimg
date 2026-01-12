package config

import (
	"crypto/rand"
	"encoding/base64"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

type Config struct {
	// 服务器配置
	Port string

	// Sqlite3数据库
	SqlitePath string

	// 数据库配置
	DbType           string
	DbHost           string
	DbPort           int
	DbUser           string
	DbPassword       string
	DbName           string
	DbCaCertPath     string
	DbSkipCertVerify bool

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

// 检查.env文件/目录状态
func EnvExists() bool {
	info, err := os.Stat(".env")
	if os.IsNotExist(err) {
		return false
	}
	// 如果存在但不是文件，先删除目录
	if err == nil && info.IsDir() {
		log.Printf("发现.env是目录，正在删除...")
		if err := os.RemoveAll(".env"); err != nil {
			log.Fatalf("删除.env目录失败：%v", err)
		}
		return false
	}
	return true
}

// 创建默认.env文件
func CreateDefaultEnv() {
	// 创建data目录（避免SQLite路径报错）
	if err := os.MkdirAll("./data", 0755); err != nil {
		log.Fatalf("创建data目录失败：%v", err)
	}

	// 生成随机的SESSION_SECRET（32位base64编码）
	sessionSecret := generateRandomSecret(32)

	// 直接定义.env模板内容
	envTemplate := `# 服务器配置
SERVER_PORT=8080

# 数据库配置
DB_TYPE=sqlite
# 类型可选：sqlite | mysql | postgres
SQLITE_PATH=./data/data.db
DB_HOST=localhost
DB_PORT=3306
DB_USER=root
DB_PASSWORD=
DB_NAME=oneimgxru
# CA证书路径，如果不需要TLS加密连接则将其注释
DB_CA_CERT_PATH=./ca/isrgrootx1.pem
DB_SKIP_CERT_VERIFY=false

# 文件上传配置
MAX_FILE_SIZE=10485760
ALLOWED_TYPES=image/jpeg,image/png,image/gif,image/webp,image/svg+xml

# 默认用户配置
DEFAULT_USER=admin
DEFAULT_PASS=123456

# Session配置
SESSION_SECRET=
`

	// 替换模板中的SESSION_SECRET占位符
	envContent := strings.Replace(envTemplate, "SESSION_SECRET=", "SESSION_SECRET="+sessionSecret, 1)

	// 写入.env文件
	wd, err := os.Getwd()
	if err != nil {
		log.Fatalf("获取当前工作目录失败：%v", err)
	}
	envPath := filepath.Join(wd, ".env")

	// 确保目标路径不是目录
	if info, err := os.Stat(envPath); err == nil && info.IsDir() {
		log.Fatalf("无法写入.env文件：%s 是一个目录", envPath)
	}

	if err := os.WriteFile(envPath, []byte(envContent), 0644); err != nil {
		log.Fatalf("生成默认.env文件失败：%v", err)
	}

	log.Printf("✅ 首次启动：自动生成.env文件（路径：%s）", envPath)
}

// 生成指定长度的随机密钥（base64编码）
func generateRandomSecret(length int) string {
	bytes := make([]byte, length)
	_, err := rand.Read(bytes)
	if err != nil {
		log.Fatalf("生成随机密钥失败：%v", err)
	}
	return base64.URLEncoding.EncodeToString(bytes)
}

// 初始化配置（优先加载外部.env，无则生成默认）
func NewConfig() {
	// 1. 检查.env文件，不存在则生成
	if !EnvExists() {
		CreateDefaultEnv()
	}

	// 2. 加载.env文件（此时必存在）
	err := godotenv.Load()
	if err != nil {
		log.Fatalf("加载.env文件失败：%v", err)
	}

	// 3. 解析配置项
	maxFileSize, _ := strconv.ParseInt(getEnv("MAX_FILE_SIZE", "10485760"), 10, 64)
	allowedTypes := strings.Split(getEnv("ALLOWED_TYPES", "image/jpeg,image/png,image/gif,image/webp"), ",")
	port := getEnv("SERVER_PORT", getEnv("PORT", "8080"))

	// Sqlite3配置
	sqlitePath := getEnv("SQLITE_PATH", "./data/data.db")
	// 确保SQLite目录存在
	if err := os.MkdirAll(filepath.Dir(sqlitePath), 0755); err != nil {
		log.Printf("警告：创建SQLite目录失败：%v", err)
	}

	// Mysql配置
	dbType := getEnv("DB_TYPE", "sqlite3")
	dbHost := getEnv("DB_HOST", "localhost")
	dbPort, _ := strconv.Atoi(getEnv("DB_PORT", "3306"))
	dbUser := getEnv("DB_USER", "root")
	dbPassword := getEnv("DB_PASSWORD", "")
	dbName := getEnv("DB_NAME", "oneimgxru")
	dbCaCertPath := getEnv("DB_CA_CERT_PATH", "")
	dbSkipCertVerify := getEnv("DB_SKIP_CERT_VERIFY", "false") == "true"

	// 默认用户配置
	defaultUser := getEnv("DEFAULT_USER", "admin")
	defaultPass := getEnv("DEFAULT_PASS", "123456")

	// JWT配置（默认生成随机密钥，避免硬编码）
	jwtSecret := getEnv("JWT_SECRET", generateRandomSecret(32))

	// Session配置（读取.env中的值，无则生成）
	sessionSecret := getEnv("SESSION_SECRET", generateRandomSecret(32))

	// 初始化全局配置
	App = &Config{
		Port:             port,
		SqlitePath:       sqlitePath,
		DbType:           dbType,
		DbHost:           dbHost,
		DbPort:           dbPort,
		DbUser:           dbUser,
		DbPassword:       dbPassword,
		DbName:           dbName,
		MaxFileSize:      maxFileSize,
		AllowedTypes:     allowedTypes,
		DefaultUser:      defaultUser,
		DefaultPass:      defaultPass,
		JWTSecret:        jwtSecret,
		SessionSecret:    sessionSecret,
		DbCaCertPath:     dbCaCertPath,
		DbSkipCertVerify: dbSkipCertVerify,
	}

	log.Println("✅ 配置初始化完成")
}

// 获取环境变量
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
