// Package storage owns raw stored artifacts. Encryption/decryption, image
// processing, quota accounting and database publication stay with the callers.
package storage

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"oneimg/backend/config"
	"oneimg/backend/models"
	"oneimg/backend/utils/buckets"
	"oneimg/backend/utils/s3"
)

var (
	ErrNotFound     = errors.New("storage artifact not found")
	ErrInvalidKey   = errors.New("invalid storage object key")
	ErrAccessDenied = errors.New("storage access denied")
)

// Ref addresses exactly one artifact; Telegram needs its persisted message and
// file IDs as well as the stable public key. Main and thumbnail refs are separate.
type Ref struct {
	Key      string
	Metadata map[string]any
}
type UploadOptions struct {
	Size        int64
	ContentType string
	FileName    string
	Private     bool // encrypted local artifacts require owner-only permissions
}
type Info struct {
	Size        int64
	ContentType string
	Modified    time.Time
	Metadata    map[string]any
}
type Driver interface {
	Upload(context.Context, Ref, io.Reader, UploadOptions) (Info, error)
	Get(context.Context, Ref) (io.ReadCloser, error)
	Stat(context.Context, Ref) (Info, error)
	// Delete is idempotent only for confirmed absence, never permission errors.
	Delete(context.Context, Ref) error
}

// Key accepts stable /uploads/... paths and remote relative keys without
// silently cleaning traversal, escaping, query fragments or Windows paths.
func Key(raw string) (string, error) {
	if raw == "" || strings.ContainsAny(raw, "\\\x00\r\n?#") || strings.HasPrefix(raw, "//") {
		return "", ErrInvalidKey
	}
	key := strings.TrimPrefix(raw, "/")
	for _, part := range strings.Split(key, "/") {
		if part == "" || part == "." || part == ".." || strings.Contains(part, ":") {
			return "", ErrInvalidKey
		}
	}
	return key, nil
}

// New uses decrypted source config, but does not fetch global settings or mutate
// configuration. Provider-specific constructors permit isolated transport tests.
func New(setting models.Settings, bucket models.Buckets) (Driver, error) {
	switch bucket.Type {
	case "default":
		return NewLocal(config.UploadRoot()), nil
	case "s3", "r2":
		client, err := s3.NewS3Client(setting, bucket)
		if err != nil {
			return nil, err
		}
		name := buckets.ConvertToS3Bucket(bucket.Config).S3Bucket
		if bucket.Type == "r2" {
			name = buckets.ConvertToR2Bucket(bucket.Config).R2Bucket
		}
		return NewS3(client, name)
	case "webdav":
		c := buckets.ConvertToWebDavBucket(bucket.Config)
		return NewWebDAV(c.WebdavURL, c.WebdavUser, c.WebdavPass, nil)
	case "ftp":
		return NewFTP(buckets.ConvertToFTPBucket(bucket.Config)), nil
	case "telegram":
		c := buckets.ConvertToTelegramBucket(bucket.Config)
		return NewTelegram(c.TGBotToken, c.TGReceivers, nil)
	default:
		return nil, fmt.Errorf("unsupported storage type %q", bucket.Type)
	}
}
