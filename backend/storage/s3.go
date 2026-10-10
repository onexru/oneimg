package storage

import (
	"bufio"
	"context"
	"errors"
	"github.com/minio/minio-go/v7"
	"io"
)

type s3Driver struct {
	client *minio.Client
	bucket string
}

func NewS3(client *minio.Client, bucket string) (Driver, error) {
	if client == nil || bucket == "" {
		return nil, errors.New("missing object storage configuration")
	}
	return &s3Driver{client, bucket}, nil
}
func (d *s3Driver) Upload(ctx context.Context, ref Ref, r io.Reader, o UploadOptions) (Info, error) {
	key, err := Key(ref.Key)
	if err != nil {
		return Info{}, err
	}
	if o.Size < 0 {
		return Info{}, errors.New("unknown artifact size")
	}
	result, err := d.client.PutObject(ctx, d.bucket, key, r, o.Size, minio.PutObjectOptions{
		ContentType: o.ContentType, DisableMultipart: true, DisableContentSha256: true, SendContentMd5: true,
	})
	return Info{Size: result.Size, ContentType: o.ContentType}, s3Error(err)
}
func (d *s3Driver) Get(ctx context.Context, ref Ref) (io.ReadCloser, error) {
	key, err := Key(ref.Key)
	if err != nil {
		return nil, err
	}
	obj, err := d.client.GetObject(ctx, d.bucket, key, minio.GetObjectOptions{})
	if err != nil {
		return nil, s3Error(err)
	}
	// MinIO is lazy. Force its initial request so missing/denied errors reach the
	// caller before headers, while retaining the peeked byte (no second GET).
	reader := bufio.NewReader(obj)
	if _, err = reader.Peek(1); err != nil && !errors.Is(err, io.EOF) {
		obj.Close()
		return nil, s3Error(err)
	}
	return &objectReader{Reader: reader, closer: obj}, nil
}

type objectReader struct {
	io.Reader
	closer io.Closer
}

func (r *objectReader) Close() error { return r.closer.Close() }
func (d *s3Driver) Stat(ctx context.Context, ref Ref) (Info, error) {
	key, err := Key(ref.Key)
	if err != nil {
		return Info{}, err
	}
	info, err := d.client.StatObject(ctx, d.bucket, key, minio.StatObjectOptions{})
	return Info{Size: info.Size, ContentType: info.ContentType, Modified: info.LastModified}, s3Error(err)
}
func (d *s3Driver) Delete(ctx context.Context, ref Ref) error {
	key, err := Key(ref.Key)
	if err != nil {
		return err
	}
	err = s3Error(d.client.RemoveObject(ctx, d.bucket, key, minio.RemoveObjectOptions{}))
	if errors.Is(err, ErrNotFound) {
		return nil
	}
	return err
}
func s3Error(err error) error {
	if err == nil {
		return nil
	}
	var e minio.ErrorResponse
	if errors.As(err, &e) {
		if e.Code == "NoSuchKey" || e.Code == "NoSuchObject" || e.Code == "NotFound" {
			return ErrNotFound
		}
		if e.Code == "AccessDenied" || e.StatusCode == 403 {
			return ErrAccessDenied
		}
	}
	return err
}
