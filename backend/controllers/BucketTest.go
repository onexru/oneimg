package controllers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awss3 "github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"

	"oneimg/backend/database"
	"oneimg/backend/models"
	utilsBuckets "oneimg/backend/utils/buckets"
	ftpclient "oneimg/backend/utils/ftp"
	"oneimg/backend/utils/result"
	s3client "oneimg/backend/utils/s3"
	"oneimg/backend/utils/secureconfig"
	webdavclient "oneimg/backend/utils/webdav"
)

const bucketConnectionTestTimeout = 25 * time.Second

var bucketConnectionTestSlots = make(chan struct{}, 4)

var bucketConfigKeys = map[string][]string{
	"s3":       {"s3_endpoint", "s3_access_key", "s3_secret_key", "s3_bucket"},
	"r2":       {"r2_endpoint", "r2_access_key", "r2_secret_key", "r2_bucket"},
	"ftp":      {"ftp_host", "ftp_port", "ftp_user", "ftp_pass"},
	"webdav":   {"webdav_url", "webdav_user", "webdav_pass"},
	"telegram": {"tg_bot_token", "tg_receivers"},
	"default":  {"storagePath"},
}

// TestBucketConnection 使用未保存的表单配置或已存储的存储桶配置执行连接测试。
func TestBucketConnection(c *gin.Context) {
	select {
	case bucketConnectionTestSlots <- struct{}{}:
		defer func() { <-bucketConnectionTestSlots }()
	default:
		c.JSON(http.StatusTooManyRequests, result.Error(429, "当前测试任务较多，请稍后重试"))
		return
	}

	var params map[string]any
	decoder := json.NewDecoder(io.LimitReader(c.Request.Body, 1<<20))
	if err := decoder.Decode(&params); err != nil {
		c.JSON(http.StatusBadRequest, result.Error(400, "请求参数无效"))
		return
	}

	bucket, err := buildBucketConnectionCandidate(params)
	if err != nil {
		status := http.StatusBadRequest
		if errors.Is(err, gorm.ErrRecordNotFound) {
			status = http.StatusNotFound
		}
		c.JSON(status, result.Error(status, err.Error()))
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), bucketConnectionTestTimeout)
	defer cancel()
	detail, err := testBucketConnection(ctx, bucket)
	if err != nil {
		c.JSON(http.StatusBadGateway, result.Error(502, "连接测试失败："+sanitizeBucketTestError(err, bucket.Config)))
		return
	}

	c.JSON(http.StatusOK, result.Success("连接测试成功", gin.H{
		"type":   bucket.Type,
		"detail": detail,
	}))
}

func buildBucketConnectionCandidate(params map[string]any) (models.Buckets, error) {
	var existing models.Buckets
	bucketID, err := bucketTestID(params["id"])
	if err != nil {
		return existing, err
	}
	if bucketID > 0 {
		db := database.GetDB()
		if db == nil {
			return existing, errors.New("数据库未初始化")
		}
		if err := db.DB.First(&existing, bucketID).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return existing, fmt.Errorf("存储桶不存在: %w", err)
			}
			return existing, fmt.Errorf("读取存储桶失败: %w", err)
		}
	}

	bucketType := strings.ToLower(strings.TrimSpace(fmt.Sprintf("%v", params["type"])))
	if bucketType == "<nil>" {
		bucketType = ""
	}
	if bucketType == "" {
		bucketType = existing.Type
	}
	if _, ok := bucketConfigKeys[bucketType]; !ok {
		return existing, errors.New("不支持的存储类型")
	}
	if existing.Id > 0 && existing.Type != bucketType {
		return existing, errors.New("已存储的存储桶类型不能修改")
	}

	incoming, err := extractBucketTestConfig(params, bucketType, existing.Id == 0)
	if err != nil {
		return existing, err
	}
	config := incoming
	if existing.Id > 0 {
		config, err = mergeBucketConfig(existing.Config, incoming)
		if err != nil {
			return existing, errors.New("解密已有存储配置失败")
		}
	}
	if err := validateBucketTestConfig(bucketType, config); err != nil {
		return existing, err
	}

	return models.Buckets{
		Id:     existing.Id,
		Name:   existing.Name,
		Type:   bucketType,
		Config: config,
	}, nil
}

func bucketTestID(value any) (int, error) {
	if value == nil {
		return 0, nil
	}
	switch typed := value.(type) {
	case float64:
		if typed < 0 || typed != float64(int(typed)) {
			return 0, errors.New("存储桶 ID 无效")
		}
		return int(typed), nil
	case int:
		if typed < 0 {
			return 0, errors.New("存储桶 ID 无效")
		}
		return typed, nil
	case string:
		if strings.TrimSpace(typed) == "" {
			return 0, nil
		}
		parsed, err := strconv.Atoi(typed)
		if err != nil || parsed < 0 {
			return 0, errors.New("存储桶 ID 无效")
		}
		return parsed, nil
	default:
		return 0, errors.New("存储桶 ID 无效")
	}
}

func extractBucketTestConfig(params map[string]any, bucketType string, isNew bool) (map[string]any, error) {
	config := make(map[string]any)
	for _, key := range bucketConfigKeys[bucketType] {
		value, exists := params[key]
		if !exists {
			continue
		}
		if key == "ftp_port" {
			port, err := bucketTestPort(value)
			if err != nil {
				return nil, err
			}
			config[key] = port
			continue
		}
		config[key] = value
	}
	if bucketType == "ftp" && isNew {
		if _, ok := config["ftp_port"]; !ok {
			config["ftp_port"] = 21
		}
	}
	return config, nil
}

func bucketTestPort(value any) (int, error) {
	var port int
	switch typed := value.(type) {
	case float64:
		if typed != float64(int(typed)) {
			return 0, errors.New("FTP 端口必须是整数")
		}
		port = int(typed)
	case int:
		port = typed
	case string:
		if strings.TrimSpace(typed) == "" {
			return 0, errors.New("FTP 端口不能为空")
		}
		parsed, err := strconv.Atoi(typed)
		if err != nil {
			return 0, errors.New("FTP 端口无效")
		}
		port = parsed
	default:
		return 0, errors.New("FTP 端口无效")
	}
	if port < 1 || port > 65535 {
		return 0, errors.New("FTP 端口必须在 1-65535 之间")
	}
	return port, nil
}

func validateBucketTestConfig(bucketType string, config map[string]any) error {
	if bucketType == "default" {
		return nil
	}
	for _, key := range bucketConfigKeys[bucketType] {
		if key == "ftp_port" {
			if secureconfig.GetInt(config, key) < 1 {
				return errors.New("ftp_port 为必填项")
			}
			continue
		}
		if strings.TrimSpace(secureconfig.GetString(config, key)) == "" {
			return fmt.Errorf("%s 为必填项", key)
		}
	}
	return nil
}

func testBucketConnection(ctx context.Context, bucket models.Buckets) (string, error) {
	switch bucket.Type {
	case "default":
		return testLocalStorage()
	case "s3", "r2":
		return testS3CompatibleStorage(ctx, bucket)
	case "ftp":
		return testFTPStorage(bucket)
	case "webdav":
		return testWebDAVStorage(ctx, bucket)
	case "telegram":
		return testTelegramStorage(ctx, bucket)
	default:
		return "", errors.New("不支持的存储类型")
	}
}

func testLocalStorage() (string, error) {
	file, err := os.CreateTemp(".", ".oneimg-storage-test-*")
	if err != nil {
		return "", fmt.Errorf("本地目录不可写: %w", err)
	}
	name := file.Name()
	defer os.Remove(name)
	if _, err := file.WriteString("oneimg storage connection test"); err != nil {
		file.Close()
		return "", fmt.Errorf("本地文件写入失败: %w", err)
	}
	if err := file.Close(); err != nil {
		return "", fmt.Errorf("本地文件关闭失败: %w", err)
	}
	if err := os.Remove(name); err != nil {
		return "", fmt.Errorf("本地测试文件清理失败: %w", err)
	}
	return "本地目录可读写", nil
}

func testS3CompatibleStorage(ctx context.Context, bucket models.Buckets) (string, error) {
	client, err := s3client.NewS3Client(models.Settings{}, bucket)
	if err != nil {
		return "", err
	}
	bucketName := ""
	if bucket.Type == "s3" {
		bucketName = utilsBuckets.ConvertToS3Bucket(bucket.Config).S3Bucket
	} else {
		bucketName = utilsBuckets.ConvertToR2Bucket(bucket.Config).R2Bucket
	}
	key := ".oneimg-connection-test/" + uuid.NewString() + ".txt"
	content := []byte("oneimg storage connection test")
	if _, err := client.PutObject(ctx, &awss3.PutObjectInput{
		Bucket: aws.String(bucketName),
		Key:    aws.String(key),
		Body:   bytes.NewReader(content),
	}); err != nil {
		return "", fmt.Errorf("测试对象写入失败: %w", err)
	}
	cleanupCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if _, err := client.DeleteObject(cleanupCtx, &awss3.DeleteObjectInput{
		Bucket: aws.String(bucketName),
		Key:    aws.String(key),
	}); err != nil {
		return "", fmt.Errorf("写入成功，但测试对象清理失败: %w", err)
	}
	return "已验证对象写入与删除权限", nil
}

func testFTPStorage(bucket models.Buckets) (string, error) {
	config := utilsBuckets.ConvertToFTPBucket(bucket.Config)
	client := ftpclient.NewFTPUtil(ftpclient.FTPConfig{
		Host: config.FTPHost, Port: config.FTPPort, User: config.FTPUser, Password: config.FTPPass, Timeout: 8,
	})
	defer client.Close()
	remotePath := ".oneimg-connection-test-" + uuid.NewString() + ".txt"
	if err := client.UploadImage(remotePath, []byte("oneimg storage connection test"), "text/plain"); err != nil {
		return "", err
	}
	if err := client.DeleteImage(remotePath); err != nil {
		return "", fmt.Errorf("写入成功，但测试文件清理失败: %w", err)
	}
	return "已验证 FTP 登录、写入与删除权限", nil
}

func testWebDAVStorage(ctx context.Context, bucket models.Buckets) (string, error) {
	config := utilsBuckets.ConvertToWebDavBucket(bucket.Config)
	client := webdavclient.Client(webdavclient.Config{
		BaseURL: config.WebdavURL, Username: config.WebdavUser, Password: config.WebdavPass, Timeout: 15 * time.Second,
	})
	remotePath := ".oneimg-connection-test-" + uuid.NewString() + ".txt"
	if err := client.WebDAVUpload(ctx, remotePath, strings.NewReader("oneimg storage connection test")); err != nil {
		return "", err
	}
	cleanupCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := client.WebDAVDelete(cleanupCtx, remotePath); err != nil {
		return "", fmt.Errorf("写入成功，但测试文件清理失败: %w", err)
	}
	return "已验证 WebDAV 认证、写入与删除权限", nil
}

func testTelegramStorage(ctx context.Context, bucket models.Buckets) (string, error) {
	config := utilsBuckets.ConvertToTelegramBucket(bucket.Config)
	if err := callTelegramTestAPI(ctx, config.TGBotToken, "getMe", nil); err != nil {
		return "", fmt.Errorf("Bot Token 校验失败: %w", err)
	}
	if err := callTelegramTestAPI(ctx, config.TGBotToken, "getChat", map[string]string{"chat_id": config.TGReceivers}); err != nil {
		return "", fmt.Errorf("Chat ID 校验失败: %w", err)
	}
	return "已验证 Bot Token 与 Chat ID 访问权限（未发送消息）", nil
}

func callTelegramTestAPI(ctx context.Context, token, method string, payload any) error {
	var body io.Reader
	if payload != nil {
		encoded, err := json.Marshal(payload)
		if err != nil {
			return err
		}
		body = bytes.NewReader(encoded)
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.telegram.org/bot"+token+"/"+method, body)
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", "application/json")
	response, err := (&http.Client{Timeout: 12 * time.Second}).Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	var apiResponse struct {
		OK          bool   `json:"ok"`
		ErrorCode   int    `json:"error_code"`
		Description string `json:"description"`
	}
	if err := json.NewDecoder(io.LimitReader(response.Body, 1<<20)).Decode(&apiResponse); err != nil {
		return fmt.Errorf("Telegram API 响应无效（HTTP %d）", response.StatusCode)
	}
	if response.StatusCode != http.StatusOK || !apiResponse.OK {
		return fmt.Errorf("Telegram API 错误 [%d]: %s", apiResponse.ErrorCode, apiResponse.Description)
	}
	return nil
}

func sanitizeBucketTestError(err error, config map[string]any) string {
	message := err.Error()
	for key, value := range config {
		if !secureconfig.IsBucketSensitiveKey(key) {
			continue
		}
		secret := strings.TrimSpace(fmt.Sprintf("%v", value))
		if len(secret) >= 3 {
			message = strings.ReplaceAll(message, secret, "***")
		}
	}
	runes := []rune(message)
	if len(runes) > 500 {
		message = string(runes[:500]) + "..."
	}
	return message
}
