package models

import "time"

// DirectUploadSettings intentionally contains only image-processing options.
// Never persist Settings, credentials, signed URLs, or arbitrary bucket config here.
type DirectUploadSettings struct {
	CompressImage    bool
	SaveWebp         bool
	Thumbnail        bool
	EncryptedStorage bool
	WatermarkEnable  bool
	WatermarkText    string
	WatermarkPos     string
	WatermarkSize    int
	WatermarkColor   string
	WatermarkOpac    float64
}

// DirectUploadTask is both the durable queue and the quota reservation ledger.
// Staging objects are never Image rows. FinalKey is assigned before object writes.
type DirectUploadTask struct {
	ID       string `gorm:"size:36;primaryKey"`
	OwnerID  int    `gorm:"not null;uniqueIndex:idx_direct_owner_client,priority:1;index"`
	ClientID string `gorm:"size:36;not null;uniqueIndex:idx_direct_owner_client,priority:2"`
	BucketID int    `gorm:"not null;index"`
	FolderId int    `gorm:"not null;default:0;index"`
	// RequestedFolderId is immutable idempotency input; FolderId is reset when
	// a folder is deleted, without changing the originally submitted request.
	RequestedFolderId int `gorm:"not null;default:0"`
	Role              int
	Filename          string `gorm:"size:255"`
	Size              int64
	ContentType       string               `gorm:"size:64"`
	SHA256            string               `gorm:"size:64"`
	Tags              []string             `gorm:"type:text;serializer:json"`
	Settings          DirectUploadSettings `gorm:"type:text;serializer:json"`
	Status            string               `gorm:"size:24;index"`
	Transport         string               `gorm:"size:16"`
	ThumbnailStatus   string               `gorm:"size:16;index"`
	ErrorCode         string               `gorm:"size:64"`
	Message           string               `gorm:"size:255"`
	Retryable         bool
	Attempts          int
	ThumbnailAttempts int
	ImageID           int    `gorm:"index"`
	DirectKey         string `gorm:"size:512"`
	ProxyKey          string `gorm:"size:512"`
	SourceKey         string `gorm:"size:512"`
	FinalKey          string `gorm:"size:512"`
	ThumbnailKey      string `gorm:"size:512"`
	ReservedBytes     int64  `gorm:"not null;default:0"`
	FinalSize         int64
	MainSHA256        string `gorm:"size:64"`
	LastPutExpiresAt  *time.Time
	SignCount         int
	NextRetryAt       *time.Time `gorm:"index"`
	ExpiresAt         time.Time  `gorm:"index"`
	CleanedAt         *time.Time `gorm:"index"`
	CreatedAt         time.Time
	UpdatedAt         time.Time
}
