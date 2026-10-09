package storage

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"oneimg/backend/utils/telegram"
	"path"
	"strings"
)

type telegramDriver struct {
	token, chat string
	client      *http.Client
}

// The supplied HTTP client/transport is used for every API and file request;
// fixtures can intercept api.telegram.org without any real outbound traffic.
func NewTelegram(token, chat string, client *http.Client) (Driver, error) {
	if token == "" || chat == "" || strings.ContainsAny(token, "/?#") || strings.ContainsRune(token, 92) || strings.ContainsRune(token, 13) || strings.ContainsRune(token, 10) {
		return nil, errors.New("missing Telegram configuration")
	}
	return &telegramDriver{token, chat, storageHTTPClient(client)}, nil
}

type telegramResponse struct {
	OK          bool            `json:"ok"`
	Result      json.RawMessage `json:"result"`
	Code        int             `json:"error_code"`
	Description string          `json:"description"`
}

func (d *telegramDriver) api(ctx context.Context, method string, body io.Reader, contentType string) (json.RawMessage, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.telegram.org/bot"+d.token+"/"+method, body)
	if err != nil {
		return nil, errors.New("Telegram request failed")
	}
	req.Header.Set("Content-Type", contentType)
	resp, err := d.client.Do(req)
	if err != nil {
		return nil, errors.New("Telegram transport failed")
	}
	defer resp.Body.Close()
	var result telegramResponse
	if err = json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&result); err != nil {
		return nil, errors.New("invalid Telegram response")
	}
	if !result.OK {
		description := strings.ToLower(result.Description)
		if (method == "deleteMessage" && strings.Contains(description, "message to delete not found")) || (method == "getFile" && (strings.Contains(description, "file not found") || strings.Contains(description, "invalid file_id") || strings.Contains(description, "wrong file identifier"))) {
			return nil, ErrNotFound
		}
		if result.Code == 401 || result.Code == 403 {
			return nil, ErrAccessDenied
		}
		// Don't echo an upstream body/URL that could contain bot credentials.
		return nil, fmt.Errorf("Telegram %s failed (code %d)", method, result.Code)
	}
	if resp.StatusCode != 200 {
		return nil, httpStorageError(resp.StatusCode)
	}
	return result.Result, nil
}
func (d *telegramDriver) Upload(ctx context.Context, ref Ref, r io.Reader, o UploadOptions) (Info, error) {
	key, err := Key(ref.Key)
	if err != nil {
		return Info{}, err
	}
	// sendPhoto recompresses bytes and cannot implement a raw artifact driver.
	// Store new plaintext and ciphertext as documents; old photo file IDs remain
	// readable through getFile. Bot API getFile caps downloadable files at 20 MiB.
	method, field, limit := "sendDocument", "document", int64(20<<20)
	encrypted := o.ContentType == "application/octet-stream"
	if o.Size <= 0 || o.Size > limit {
		return Info{}, errors.New("Telegram artifact exceeds provider limit")
	}
	data, err := io.ReadAll(io.LimitReader(&contextReader{ctx, r}, o.Size+1))
	if err != nil {
		return Info{}, err
	}
	if int64(len(data)) != o.Size {
		return Info{}, errors.New("stored artifact size mismatch")
	}
	filename := o.FileName
	if filename == "" {
		filename = path.Base(key)
	}
	filename = path.Base(filename)
	if encrypted {
		filename += ".oneimg"
	}
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	if err = writer.WriteField("chat_id", d.chat); err != nil {
		return Info{}, err
	}
	part, err := writer.CreateFormFile(field, filename)
	if err != nil {
		return Info{}, err
	}
	if _, err = part.Write(data); err != nil {
		return Info{}, err
	}
	if err = writer.Close(); err != nil {
		return Info{}, err
	}
	// Do not automatically retry ambiguous uploads: a timeout after acceptance
	// could create orphan Telegram messages. Durable queue owns whole-task retry.
	raw, err := d.api(ctx, method, &body, writer.FormDataContentType())
	if err != nil {
		return Info{}, err
	}
	var result struct {
		MessageID int `json:"message_id"`
		Document  struct {
			FileID string `json:"file_id"`
		} `json:"document"`
	}
	if err = json.Unmarshal(raw, &result); err != nil {
		return Info{}, errors.New("invalid Telegram upload result")
	}
	fileID := result.Document.FileID

	info := Info{Size: o.Size, ContentType: o.ContentType, Metadata: map[string]any{"tg_file_id": fileID, "tg_message_id": result.MessageID, "tg_chat_id": d.chat}}
	if fileID == "" || result.MessageID <= 0 {
		return info, errors.New("missing Telegram upload identifiers")
	}
	return info, nil
}

type telegramFile struct {
	FilePath string `json:"file_path"`
	FileSize int64  `json:"file_size"`
}

func (d *telegramDriver) file(ctx context.Context, ref Ref) (telegramFile, error) {
	if _, err := Key(ref.Key); err != nil {
		return telegramFile{}, err
	}
	id := telegram.ParseFileIdFromTelegramPath(MetadataString(ref.Metadata, "tg_file_id"))
	if id == "" {
		return telegramFile{}, ErrNotFound
	}
	body, _ := json.Marshal(map[string]string{"file_id": id})
	raw, err := d.api(ctx, "getFile", bytes.NewReader(body), "application/json")
	if err != nil {
		return telegramFile{}, err
	}
	var result telegramFile
	if err = json.Unmarshal(raw, &result); err != nil {
		return result, errors.New("invalid Telegram file result")
	}
	if _, err = Key(result.FilePath); err != nil {
		return result, errors.New("invalid Telegram download path")
	}
	return result, nil
}
func (d *telegramDriver) Get(ctx context.Context, ref Ref) (io.ReadCloser, error) {
	file, err := d.file(ctx, ref)
	if err != nil {
		return nil, err
	}
	u := url.URL{Scheme: "https", Host: "api.telegram.org", Path: "/file/bot" + d.token + "/" + file.FilePath}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, errors.New("Telegram download request failed")
	}
	resp, err := d.client.Do(req)
	if err != nil {
		return nil, errors.New("Telegram download transport failed")
	}
	if resp.StatusCode != 200 {
		resp.Body.Close()
		return nil, httpStorageError(resp.StatusCode)
	}
	return resp.Body, nil
}
func (d *telegramDriver) Stat(ctx context.Context, ref Ref) (Info, error) {
	file, err := d.file(ctx, ref)
	return Info{Size: file.FileSize}, err
}
func (d *telegramDriver) Delete(ctx context.Context, ref Ref) error {
	if _, err := Key(ref.Key); err != nil {
		return err
	}
	id := MetadataInt(ref.Metadata, "tg_message_id")
	if id <= 0 {
		if MetadataString(ref.Metadata, "tg_file_id") != "" {
			return errors.New("Telegram artifact has no deletion message ID")
		}
		return nil // An unpublished/pending artifact has no message to delete.
	}
	chat := MetadataString(ref.Metadata, "tg_chat_id")
	if chat == "" {
		chat = d.chat
	}
	body, _ := json.Marshal(map[string]any{"chat_id": chat, "message_id": id})
	_, err := d.api(ctx, "deleteMessage", bytes.NewReader(body), "application/json")
	if errors.Is(err, ErrNotFound) {
		return nil
	}
	return err
}
