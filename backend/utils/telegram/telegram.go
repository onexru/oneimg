package telegram

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// Config Telegram Bot 配置
type Config struct {
	BotToken string        // Bot Token（从 @BotFather 获取）
	Timeout  time.Duration // 请求超时时间（默认10秒）
	Retry    int           // 失败重试次数（默认2次）
}

// Message 发送的消息结构体
type Message struct {
	ChatID                string `json:"chat_id"`              // 接收消息的聊天ID（用户/群组/频道ID）
	Text                  string `json:"text"`                 // 消息文本
	ParseMode             string `json:"parse_mode,omitempty"` // 解析模式（MarkdownV2/HTML）
	DisableWebPagePreview bool   `json:"disable_web_page_preview,omitempty"`
}

// Response Telegram API 响应结构体
type Response struct {
	OK          bool            `json:"ok"`
	Result      json.RawMessage `json:"result,omitempty"`      // 成功返回的结果
	Description string          `json:"description,omitempty"` // 错误描述
	ErrorCode   int             `json:"error_code,omitempty"`  // 错误码
}

// 占位符结构体（与业务字段对应）
type PlaceholderData struct {
	Username    string
	Date        string
	Filename    string
	StorageType string
	URL         string
}

// 默认配置
var defaultConfig = Config{
	Timeout: 10 * time.Second,
	Retry:   2,
}

// NewClient 创建Telegram Bot客户端
func NewClient(botToken string) *Config {
	return &Config{
		BotToken: botToken,
		Timeout:  defaultConfig.Timeout,
		Retry:    defaultConfig.Retry,
	}
}

func ReplacePlaceholders(template string, data PlaceholderData) string {
	result := template
	// 映射占位符与结构体字段
	replaceMap := map[string]string{
		"username":    data.Username,
		"date":        data.Date,
		"filename":    data.Filename,
		"StorageType": data.StorageType,
		"url":         data.URL,
	}
	// 遍历替换所有占位符
	for key, value := range replaceMap {
		placeholder := "{" + key + "}"
		result = strings.ReplaceAll(result, placeholder, value)
	}
	return result
}

// SendMsg 发送文本消息（核心方法，先替换占位符再发送）
func (c *Config) SendMsg(msg Message, placeholderData PlaceholderData) error {
	// 基础参数校验
	if c.BotToken == "" {
		return fmt.Errorf("bot token 不能为空")
	}
	if msg.ChatID == "" {
		return fmt.Errorf("chat_id 不能为空")
	}

	// 1. 处理默认模板（如果消息文本为空）
	messageText := msg.Text
	if messageText == "" {
		messageText = "{username} {date} 上传了图片 {filename}，存储容器[{StorageType}]"
	}
	// 拼接访问链接
	messageText += "\n\n访问链接:{url}"

	// 2. 替换占位符
	messageText = ReplacePlaceholders(messageText, placeholderData)
	if messageText == "" {
		return fmt.Errorf("替换占位符后消息文本为空")
	}

	// 3. 更新消息文本为替换后的值
	msg.Text = messageText

	// 构建 API 请求 URL
	apiURL := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", c.BotToken)

	// 序列化消息体
	msgBytes, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("消息序列化失败: %w", err)
	}

	// 带重试的请求逻辑
	var lastErr error
	for i := 0; i <= c.Retry; i++ {
		lastErr = c.sendRequest(apiURL, msgBytes)
		if lastErr == nil {
			return nil
		}

		// 非最后一次重试，等待后重试（指数退避）
		if i < c.Retry {
			waitTime := time.Duration(1<<i) * 500 * time.Millisecond
			time.Sleep(waitTime)
			continue
		}
	}

	return fmt.Errorf("重试%d次后仍发送失败: %w", c.Retry, lastErr)
}

// sendRequest 单次发送HTTP请求
func (c *Config) sendRequest(apiURL string, msgBytes []byte) error {
	// 创建 HTTP 请求
	req, err := http.NewRequest("POST", apiURL, bytes.NewBuffer(msgBytes))
	if err != nil {
		return fmt.Errorf("创建请求失败: %w", err)
	}
	req.Header.Set("Content-Type", "application/json; charset=utf-8")

	// 发送请求
	client := &http.Client{Timeout: c.Timeout}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("发送请求失败: %w", err)
	}
	defer resp.Body.Close()

	// 解析响应
	var tgResp Response
	if err := json.NewDecoder(resp.Body).Decode(&tgResp); err != nil {
		return fmt.Errorf("解析响应失败: %w", err)
	}

	// 处理 Telegram API 错误
	if !tgResp.OK {
		return fmt.Errorf("telegram API 错误 [code:%d]: %s", tgResp.ErrorCode, tgResp.Description)
	}

	return nil
}

func SendSimpleMsg(botToken, chatID, text string, placeholderData PlaceholderData) error {
	client := NewClient(botToken)
	return client.SendMsg(Message{
		ChatID: chatID,
		Text:   text,
	}, placeholderData)
}
