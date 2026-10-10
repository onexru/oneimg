package services

import (
	"errors"
	"regexp"
	"sync"
	"time"
)

const DirectUploadMaxBytes int64 = 32 << 20

const directPutTTL = 15 * time.Minute

const directTaskTTL = 24 * time.Hour

const directMaxAttempts = 3

var directMu sync.Mutex

var directUUID = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

var directHash = regexp.MustCompile(`^[0-9a-f]{64}$`)

// DirectUploadError is safe to return to clients; no SDK error or signed URL is exposed.
type DirectUploadError struct {
	Status    int
	Code      string
	Message   string
	Retryable bool
}

func (e *DirectUploadError) Error() string { return e.Message }

func directError(status int, code, message string, retry bool) *DirectUploadError {
	return &DirectUploadError{status, code, message, retry}
}

func directDBError(err error) error {
	if err == nil {
		return nil
	}
	var folderErr *GalleryFolderError
	if errors.As(err, &folderErr) {
		return directError(folderErr.Status, folderErr.Code, folderErr.Message, false)
	}
	var e *DirectUploadError
	if errors.As(err, &e) {
		return e
	}
	return directError(503, "database_unavailable", "上传任务暂时不可用，请稍后重试", true)
}

type CreateDirectUploadRequest struct {
	ClientID    string   `json:"client_id"`
	BucketID    int      `json:"bucket_id"`
	Filename    string   `json:"filename"`
	Size        int64    `json:"size"`
	ContentType string   `json:"content_type"`
	SHA256      string   `json:"sha256"`
	Tags        []string `json:"tags"`
	FolderID    int      `json:"folder_id"`
}

type DirectUploadInstruction struct {
	URL       string            `json:"url"`
	Method    string            `json:"method"`
	Headers   map[string]string `json:"headers"`
	ExpiresAt time.Time         `json:"expires_at"`
}

type DirectUploadImage struct {
	ID           int    `json:"id"`
	URL          string `json:"url"`
	ThumbnailURL string `json:"thumbnail_url"`
	Filename     string `json:"filename"`
	FileSize     int64  `json:"file_size"`
	MimeType     string `json:"mimeType"`
	Width        int    `json:"width"`
	Height       int    `json:"height"`
	Storage      string `json:"storage"`
	BucketID     int    `json:"bucket_id"`
	FolderID     int    `json:"folder_id"`
}

type DirectUploadResponse struct {
	ID                string                   `json:"id"`
	ClientID          string                   `json:"client_id"`
	FolderID          int                      `json:"folder_id"`
	BucketID          int                      `json:"bucket_id"`
	Filename          string                   `json:"filename"`
	Status            string                   `json:"status"`
	Transport         string                   `json:"transport"`
	ThumbnailStatus   string                   `json:"thumbnail_status"`
	ErrorCode         string                   `json:"error_code"`
	Message           string                   `json:"message"`
	Retryable         bool                     `json:"retryable"`
	Attempts          int                      `json:"attempts"`
	ThumbnailAttempts int                      `json:"thumbnail_attempts"`
	ImageID           int                      `json:"image_id"`
	Image             *DirectUploadImage       `json:"image"`
	CreatedAt         time.Time                `json:"created_at"`
	UpdatedAt         time.Time                `json:"updated_at"`
	NextRetryAt       *time.Time               `json:"next_retry_at"`
	Upload            *DirectUploadInstruction `json:"upload,omitempty"`
}
