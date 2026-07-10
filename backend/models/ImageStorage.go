package models

import "time"

const (
	ImageStorageStatusPending   = "pending"
	ImageStorageStatusUploading = "uploading"
	ImageStorageStatusSuccess   = "success"
	ImageStorageStatusFailed    = "failed"
)

// ImageStorage records one physical copy of an image in a storage bucket.
// The (image_id, bucket_id) pair is unique so the row can also act as a
// durable synchronization job for that copy.
type ImageStorage struct {
	ID            int            `json:"id" gorm:"type:integer;primaryKey;autoIncrement"`
	ImageID       int            `json:"image_id" gorm:"column:image_id;not null;uniqueIndex:idx_image_storage_image_bucket,priority:1;index:idx_image_storages_image"`
	BucketID      int            `json:"bucket_id" gorm:"column:bucket_id;not null;uniqueIndex:idx_image_storage_image_bucket,priority:2;index:idx_image_storages_bucket"`
	Storage       string         `json:"storage" gorm:"column:storage;not null"`
	Status        string         `json:"status" gorm:"column:status;size:16;not null;default:pending;index:idx_image_storages_status"`
	URL           string         `json:"url" gorm:"column:url"`
	Thumbnail     string         `json:"thumbnail" gorm:"column:thumbnail"`
	FileSize      int64          `json:"file_size" gorm:"column:file_size;not null;default:0"`
	ThumbnailSize int64          `json:"thumbnail_size" gorm:"column:thumbnail_size;not null;default:0"`
	Error         string         `json:"error" gorm:"column:error;type:text"`
	RetryCount    int            `json:"retry_count" gorm:"column:retry_count;not null;default:0"`
	Metadata      map[string]any `json:"metadata" gorm:"column:metadata;type:text;serializer:json"`
	StartedAt     *time.Time     `json:"started_at" gorm:"column:started_at"`
	NextRetryAt   *time.Time     `json:"next_retry_at" gorm:"column:next_retry_at;index:idx_image_storages_retry"`
	SyncedAt      *time.Time     `json:"synced_at" gorm:"column:synced_at"`
	CreatedAt     time.Time      `json:"created_at" gorm:"column:created_at;autoCreateTime"`
	UpdatedAt     time.Time      `json:"updated_at" gorm:"column:updated_at;autoUpdateTime"`
}

func (ImageStorage) TableName() string {
	return "image_storages"
}
