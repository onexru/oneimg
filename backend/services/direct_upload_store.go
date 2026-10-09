package services

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"oneimg/backend/storage"
	"strconv"
	"time"

	"github.com/minio/minio-go/v7"
	"oneimg/backend/models"
	"oneimg/backend/utils/buckets"
	storageS3 "oneimg/backend/utils/s3"
)

// Direct upload adds browser PUT signing to the shared raw storage driver.
// Staging/final keys and verification remain the direct-upload service policy.
type directObjectStore interface {
	Sign(context.Context, string, string, int64, time.Duration) (string, error)
	Head(context.Context, string) (int64, error)
	Read(context.Context, string) (io.ReadCloser, error)
	Put(context.Context, string, []byte, string) error
	Delete(context.Context, string) error
}
type directR2Store struct {
	client *minio.Client
	bucket string
	driver storage.Driver
}

var directOpenStore = func(b models.Buckets) (directObjectStore, error) {
	client, err := storageS3.NewS3Client(models.Settings{}, b)
	if err != nil {
		return nil, err
	}
	name := buckets.ConvertToR2Bucket(b.Config).R2Bucket
	driver, err := storage.NewS3(client, name)
	if err != nil {
		return nil, err
	}
	return &directR2Store{client: client, bucket: name, driver: driver}, nil
}

func (s *directR2Store) Sign(ctx context.Context, key, mime string, size int64, ttl time.Duration) (string, error) {
	if _, err := storage.Key(key); err != nil {
		return "", err
	}
	// Browser sets Content-Length itself from the File body. Signing it prevents
	// a stolen URL from being used to allocate an arbitrarily large R2 object.
	headers := http.Header{"Content-Type": {mime}, "Content-Length": {strconv.FormatInt(size, 10)}}
	u, err := s.client.PresignHeader(ctx, http.MethodPut, s.bucket, key, ttl, nil, headers)
	if err != nil {
		return "", err
	}
	return u.String(), nil
}
func (s *directR2Store) Head(ctx context.Context, key string) (int64, error) {
	v, err := s.driver.Stat(ctx, storage.Ref{Key: key})
	return v.Size, err
}
func (s *directR2Store) Read(ctx context.Context, key string) (io.ReadCloser, error) {
	return s.driver.Get(ctx, storage.Ref{Key: key})
}
func (s *directR2Store) Put(ctx context.Context, key string, data []byte, mime string) error {
	_, err := s.driver.Upload(ctx, storage.Ref{Key: key}, bytes.NewReader(data), storage.UploadOptions{Size: int64(len(data)), ContentType: mime})
	return err
}
func (s *directR2Store) Delete(ctx context.Context, key string) error {
	return s.driver.Delete(ctx, storage.Ref{Key: key})
}
func directStoreError(err error) *DirectUploadError {
	r := minio.ToErrorResponse(err)
	if errors.Is(err, storage.ErrNotFound) || r.StatusCode == 404 || r.Code == "NoSuchKey" || r.Code == "NoSuchObject" {
		return directError(409, "object_missing", "尚未收到完整文件，请重试传输或选择原文件", true)
	}
	if errors.Is(err, storage.ErrAccessDenied) || r.StatusCode == 403 || r.Code == "AccessDenied" {
		return directError(502, "storage_permission", "存储源拒绝访问，请联系管理员检查权限", false)
	}
	return directError(502, "storage_unavailable", "存储源暂时不可用，请稍后重试", true)
}
