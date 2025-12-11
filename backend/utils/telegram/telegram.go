// telegram/telegram.go
package telegram

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"path/filepath"
	"strings"
	"time"
)

// 【关键修改】导出方法（首字母大写），供Proxy.go调用
func ParseFileIdFromTelegramPath(path string) string {
	if path == "" {
		return ""
	}
	if after, ok := strings.CutPrefix(path, "tg://file?id="); ok {
		return after
	}
	if after, ok := strings.CutPrefix(path, "/tg/"); ok {
		return after
	}
	return path
}

// 【关键修改】导出方法（首字母大写），供Proxy.go调用
func GetTelegramFileStreamReader(client *Config, fileId string) (io.ReadCloser, error) {
	var lastErr error
	for i := 0; i <= client.Retry; i++ {
		reader, err := getTelegramFileStreamReaderOnce(client, fileId)
		if err == nil {
			return reader, nil
		}
		lastErr = err
		if i < client.Retry {
			waitTime := time.Duration(1<<i) * 500 * time.Millisecond
			time.Sleep(waitTime)
			continue
		}
	}
	return nil, fmt.Errorf("重试%d次后仍获取文件流失败: %w", client.Retry, lastErr)
}

// 内部方法（小写，不导出）
func getTelegramFileStreamReaderOnce(client *Config, fileId string) (io.ReadCloser, error) {
	fileURL := fmt.Sprintf("https://api.telegram.org/bot%s/getFile", client.BotToken)
	reqBody := []byte(fmt.Sprintf(`{"file_id":"%s"}`, fileId))

	req, err := http.NewRequest("POST", fileURL, strings.NewReader(string(reqBody)))
	if err != nil {
		return nil, fmt.Errorf("创建getFile请求失败: %w", err)
	}
	req.Header.Set("Content-Type", "application/json; charset=utf-8")

	httpClient := &http.Client{Timeout: client.Timeout}
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("调用getFile接口失败: %w", err)
	}
	defer resp.Body.Close()

	var fileResp FileResponse
	if err := json.NewDecoder(resp.Body).Decode(&fileResp); err != nil {
		return nil, fmt.Errorf("解析getFile响应失败: %w", err)
	}
	if !fileResp.OK {
		return nil, fmt.Errorf("telegram API 错误 [code:%d]: %s", fileResp.ErrorCode, fileResp.Description)
	}
	if fileResp.Result.FilePath == "" {
		return nil, errors.New("未获取到文件下载路径")
	}

	downloadURL := fmt.Sprintf("https://api.telegram.org/file/bot%s/%s", client.BotToken, fileResp.Result.FilePath)
	downloadResp, err := httpClient.Get(downloadURL)
	if err != nil {
		return nil, fmt.Errorf("创建下载请求失败: %w", err)
	}
	if downloadResp.StatusCode != http.StatusOK {
		downloadResp.Body.Close()
		return nil, fmt.Errorf("下载请求失败，HTTP状态码: %d", downloadResp.StatusCode)
	}

	return downloadResp.Body, nil
}

// 其他核心方法（NewClient、UploadPhotoByBytes、SendMsg等）保持不变...
type Config struct {
	BotToken string        // Bot Token（从 @BotFather 获取）
	Timeout  time.Duration // 请求超时时间（默认10秒）
	Retry    int           // 失败重试次数（默认2次）
}

type Message struct {
	ChatID                string `json:"chat_id"`              // 接收消息的聊天ID（用户/群组/频道ID）
	Text                  string `json:"text"`                 // 消息文本
	ParseMode             string `json:"parse_mode,omitempty"` // 解析模式（MarkdownV2/HTML）
	DisableWebPagePreview bool   `json:"disable_web_page_preview,omitempty"`
}

type Response struct {
	OK          bool            `json:"ok"`
	Result      json.RawMessage `json:"result,omitempty"`      // 成功返回的结果
	Description string          `json:"description,omitempty"` // 错误描述
	ErrorCode   int             `json:"error_code,omitempty"`  // 错误码
}

type PhotoResponse struct {
	OK     bool `json:"ok"`
	Result struct {
		Photo []struct {
			FileID       string `json:"file_id"`
			FileUniqueID string `json:"file_unique_id"`
			Width        int    `json:"width"`
			Height       int    `json:"height"`
			FileSize     int64  `json:"file_size,omitempty"`
		} `json:"photo"`
		MessageID int `json:"message_id"`
	} `json:"result"`
	Description string `json:"description,omitempty"`
	ErrorCode   int    `json:"error_code,omitempty"`
}

type FileResponse struct {
	OK     bool `json:"ok"`
	Result struct {
		FileID       string `json:"file_id"`
		FileUniqueID string `json:"file_unique_id"`
		FileSize     int64  `json:"file_size"`
		FilePath     string `json:"file_path"`
	} `json:"result"`
	Description string `json:"description,omitempty"`
	ErrorCode   int    `json:"error_code,omitempty"`
}

type PlaceholderData struct {
	Username    string
	Date        string
	Filename    string
	StorageType string
	URL         string
}

var defaultConfig = Config{
	Timeout: 10 * time.Second,
	Retry:   2,
}

type TelegramUploader struct {
	client *Config
}

func NewTelegramUploader(cfg *Config) *TelegramUploader {
	return &TelegramUploader{
		client: cfg,
	}
}

func NewClient(botToken string) *Config {
	return &Config{
		BotToken: botToken,
		Timeout:  defaultConfig.Timeout,
		Retry:    defaultConfig.Retry,
	}
}

// 删除图片
func (t *TelegramUploader) DeletePhoto(chatID string, messageID int) error {
	// 1. 基础参数校验
	if t.client.BotToken == "" {
		return fmt.Errorf("bot token 不能为空")
	}
	if chatID == "" {
		return fmt.Errorf("chat id 不能为空")
	}
	if messageID <= 0 {
		return fmt.Errorf("message id 无效（需大于0）: %d", messageID)
	}

	// 2. 构建删除请求（复用重试逻辑）
	var lastErr error
	for i := 0; i <= t.client.Retry; i++ {
		err := t.deletePhotoOnce(chatID, messageID)
		if err == nil {
			return nil
		}
		lastErr = err

		// 指数退避重试
		if i < t.client.Retry {
			waitTime := time.Duration(1<<i) * 500 * time.Millisecond
			time.Sleep(waitTime)
			continue
		}
	}

	return fmt.Errorf("重试%d次后仍删除图片失败: %w", t.client.Retry, lastErr)
}

// deletePhotoOnce 单次执行删除操作（内部方法，不导出）
func (t *TelegramUploader) deletePhotoOnce(chatID string, messageID int) error {
	apiURL := fmt.Sprintf("https://api.telegram.org/bot%s/deleteMessage", t.client.BotToken)

	// 3. 构造请求体（messageID为int类型，避免字符串类型错误）
	reqBody := map[string]interface{}{
		"chat_id":    chatID,
		"message_id": messageID,
	}
	reqBodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("序列化deleteMessage请求体失败: %w", err)
	}

	// 4. 创建HTTP请求
	req, err := http.NewRequest("POST", apiURL, bytes.NewBuffer(reqBodyBytes))
	if err != nil {
		return fmt.Errorf("创建deleteMessage请求失败: %w", err)
	}
	req.Header.Set("Content-Type", "application/json; charset=utf-8")

	// 5. 发送请求
	httpClient := &http.Client{Timeout: t.client.Timeout}
	resp, err := httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("调用deleteMessage接口失败: %w", err)
	}
	defer resp.Body.Close()

	// 6. 解析响应
	var deleteResp Response
	if err := json.NewDecoder(resp.Body).Decode(&deleteResp); err != nil {
		return fmt.Errorf("解析deleteMessage响应失败: %w", err)
	}
	if !deleteResp.OK {
		return fmt.Errorf("telegram API 错误 [code:%d]: %s", deleteResp.ErrorCode, deleteResp.Description)
	}

	return nil
}

// 获取图片字节流
func ReplacePlaceholders(template string, data PlaceholderData) string {
	result := template
	replaceMap := map[string]string{
		"username":    data.Username,
		"date":        data.Date,
		"filename":    data.Filename,
		"StorageType": data.StorageType,
		"url":         data.URL,
	}
	for key, value := range replaceMap {
		placeholder := "{" + key + "}"
		result = strings.ReplaceAll(result, placeholder, value)
	}
	return result
}

func (c *Config) SendMsg(msg Message, placeholderData PlaceholderData) error {
	if c.BotToken == "" {
		return fmt.Errorf("bot token 不能为空")
	}
	if msg.ChatID == "" {
		return fmt.Errorf("chat_id 不能为空")
	}

	messageText := msg.Text
	if messageText == "" {
		messageText = "{username} {date} 上传了图片 {filename}，存储容器[{StorageType}]"
	}
	messageText += "\n\n访问链接:{url}"

	messageText = ReplacePlaceholders(messageText, placeholderData)
	if messageText == "" {
		return fmt.Errorf("替换占位符后消息文本为空")
	}

	msg.Text = messageText

	apiURL := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", c.BotToken)

	msgBytes, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("消息序列化失败: %w", err)
	}

	var lastErr error
	for i := 0; i <= c.Retry; i++ {
		lastErr = c.sendRequest(apiURL, msgBytes)
		if lastErr == nil {
			return nil
		}

		if i < c.Retry {
			waitTime := time.Duration(1<<i) * 500 * time.Millisecond
			time.Sleep(waitTime)
			continue
		}
	}

	return fmt.Errorf("重试%d次后仍发送失败: %w", c.Retry, lastErr)
}

func (c *Config) sendRequest(apiURL string, msgBytes []byte) error {
	req, err := http.NewRequest("POST", apiURL, bytes.NewBuffer(msgBytes))
	if err != nil {
		return fmt.Errorf("创建请求失败: %w", err)
	}
	req.Header.Set("Content-Type", "application/json; charset=utf-8")

	client := &http.Client{Timeout: c.Timeout}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("发送请求失败: %w", err)
	}
	defer resp.Body.Close()

	var tgResp Response
	if err := json.NewDecoder(resp.Body).Decode(&tgResp); err != nil {
		return fmt.Errorf("解析响应失败: %w", err)
	}

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

func (c *Config) UploadPhotoByBytes(chatID string, photoBytes []byte, filename string, caption string) (fileID string, messageID int, err error) {
	if c.BotToken == "" {
		return "", 0, fmt.Errorf("bot token 不能为空")
	}
	if chatID == "" {
		return "", 0, fmt.Errorf("chat_id 不能为空")
	}
	if len(photoBytes) == 0 {
		return "", 0, fmt.Errorf("图片字节流不能为空")
	}
	if filename == "" {
		return "", 0, fmt.Errorf("图片文件名不能为空")
	}

	if len(photoBytes) > 10*1024*1024 {
		return "", 0, fmt.Errorf("图片字节流超过10MB限制（当前：%d字节）", len(photoBytes))
	}

	apiURL := fmt.Sprintf("https://api.telegram.org/bot%s/sendPhoto", c.BotToken)

	var requestBody bytes.Buffer
	writer := multipart.NewWriter(&requestBody)

	if err := writer.WriteField("chat_id", chatID); err != nil {
		return "", 0, fmt.Errorf("写入chat_id失败: %w", err)
	}

	if caption != "" {
		if err := writer.WriteField("caption", caption); err != nil {
			return "", 0, fmt.Errorf("写入caption失败: %w", err)
		}
	}

	photoPart, err := writer.CreateFormFile("photo", filepath.Base(filename))
	if err != nil {
		return "", 0, fmt.Errorf("创建图片表单字段失败: %w", err)
	}
	if _, err := photoPart.Write(photoBytes); err != nil {
		return "", 0, fmt.Errorf("写入图片字节流失败: %w", err)
	}

	if err := writer.Close(); err != nil {
		return "", 0, fmt.Errorf("关闭multipart writer失败: %w", err)
	}

	var lastErr error
	for i := 0; i <= c.Retry; i++ {
		fileID, messageID, lastErr = c.uploadPhotoByBytesRequest(apiURL, &requestBody, writer.FormDataContentType())
		if lastErr == nil {
			return fileID, messageID, nil
		}

		if i < c.Retry {
			waitTime := time.Duration(1<<i) * 500 * time.Millisecond
			time.Sleep(waitTime)
			continue
		}
	}

	return "", 0, fmt.Errorf("重试%d次后仍上传图片失败: %w", c.Retry, lastErr)
}

func (c *Config) uploadPhotoByBytesRequest(apiURL string, requestBody *bytes.Buffer, contentType string) (fileID string, messageID int, err error) {
	req, err := http.NewRequest("POST", apiURL, requestBody)
	if err != nil {
		return "", 0, fmt.Errorf("创建请求失败: %w", err)
	}
	req.Header.Set("Content-Type", contentType)

	client := &http.Client{Timeout: c.Timeout}
	resp, err := client.Do(req)
	if err != nil {
		return "", 0, fmt.Errorf("发送请求失败: %w", err)
	}
	defer resp.Body.Close()

	var photoResp PhotoResponse
	if err := json.NewDecoder(resp.Body).Decode(&photoResp); err != nil {
		return "", 0, fmt.Errorf("解析响应失败: %w", err)
	}

	if !photoResp.OK {
		return "", 0, fmt.Errorf("telegram API 错误 [code:%d]: %s", photoResp.ErrorCode, photoResp.Description)
	}

	if len(photoResp.Result.Photo) == 0 {
		return "", 0, fmt.Errorf("未返回图片信息")
	}
	return photoResp.Result.Photo[len(photoResp.Result.Photo)-1].FileID, photoResp.Result.MessageID, nil
}
