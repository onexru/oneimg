package services

import (
	"errors"
	"path/filepath"
	"testing"

	"oneimg/backend/config"
	"oneimg/backend/database"
	"oneimg/backend/models"
)

func TestBackfillImageStoragesIsIdempotent(t *testing.T) {
	initStorageSyncTestDB(t)
	db := database.GetDB().DB

	localBucket := models.Buckets{
		Id:       1,
		Name:     "local",
		Type:     "default",
		Capacity: 0,
		Config:   map[string]any{"storagePath": "/uploads"},
	}
	if err := db.Create(&localBucket).Error; err != nil {
		t.Fatalf("create bucket: %v", err)
	}
	image := models.Image{
		Url:      "/uploads/example.webp",
		FileName: "example.webp",
		FileSize: 42,
		Storage:  "default",
		BucketId: 1,
		UserId:   1,
	}
	if err := db.Create(&image).Error; err != nil {
		t.Fatalf("create image: %v", err)
	}

	if err := BackfillImageStorages(); err != nil {
		t.Fatalf("first backfill: %v", err)
	}
	if err := BackfillImageStorages(); err != nil {
		t.Fatalf("second backfill: %v", err)
	}

	var replicas []models.ImageStorage
	if err := db.Where("image_id = ?", image.Id).Find(&replicas).Error; err != nil {
		t.Fatalf("query replicas: %v", err)
	}
	if len(replicas) != 1 {
		t.Fatalf("expected one replica after idempotent backfill, got %d", len(replicas))
	}
	if replicas[0].BucketID != 1 || replicas[0].Status != models.ImageStorageStatusSuccess {
		t.Fatalf("unexpected backfilled replica: %+v", replicas[0])
	}
}

func TestStorageWorkerHonorsDisabledSwitch(t *testing.T) {
	initStorageSyncTestDB(t)
	db := database.GetDB().DB
	if err := db.Create(&models.Settings{MultiStorageSync: false}).Error; err != nil {
		t.Fatalf("create settings: %v", err)
	}
	remoteBucket := models.Buckets{
		Id:       2,
		Name:     "remote",
		Type:     "s3",
		Capacity: 1024,
		Config:   map[string]any{},
	}
	if err := db.Create(&remoteBucket).Error; err != nil {
		t.Fatalf("create remote bucket: %v", err)
	}
	replica := models.ImageStorage{
		ImageID:  1,
		BucketID: 2,
		Storage:  "s3",
		Status:   models.ImageStorageStatusPending,
	}
	if err := db.Create(&replica).Error; err != nil {
		t.Fatalf("create pending replica: %v", err)
	}

	if processNextStorageSyncTask() {
		t.Fatal("worker reported work while multi-storage synchronization was disabled")
	}
	var stored models.ImageStorage
	if err := db.First(&stored, replica.ID).Error; err != nil {
		t.Fatalf("reload pending replica: %v", err)
	}
	if stored.Status != models.ImageStorageStatusPending {
		t.Fatalf("disabled worker changed status to %q", stored.Status)
	}
}

func TestStorageSyncFailureRetriesBeforeTerminalFailure(t *testing.T) {
	initStorageSyncTestDB(t)
	db := database.GetDB().DB
	replica := models.ImageStorage{
		ImageID:  1,
		BucketID: 2,
		Storage:  "s3",
		Status:   models.ImageStorageStatusUploading,
	}
	if err := db.Create(&replica).Error; err != nil {
		t.Fatalf("create uploading replica: %v", err)
	}

	for attempt := 1; attempt <= storageSyncMaxAttempts; attempt++ {
		if err := markStorageSyncFailed(replica.ID, errors.New("temporary failure"), nil); err != nil {
			t.Fatalf("mark attempt %d failed: %v", attempt, err)
		}
		var stored models.ImageStorage
		if err := db.First(&stored, replica.ID).Error; err != nil {
			t.Fatalf("reload attempt %d: %v", attempt, err)
		}
		if stored.RetryCount != attempt {
			t.Fatalf("attempt %d stored retry_count=%d", attempt, stored.RetryCount)
		}
		if attempt < storageSyncMaxAttempts {
			if stored.Status != models.ImageStorageStatusPending || stored.NextRetryAt == nil {
				t.Fatalf("attempt %d should be scheduled for retry: %+v", attempt, stored)
			}
			if err := db.Model(&stored).Updates(map[string]any{
				"status":        models.ImageStorageStatusUploading,
				"next_retry_at": nil,
			}).Error; err != nil {
				t.Fatalf("prepare attempt %d: %v", attempt+1, err)
			}
		} else if stored.Status != models.ImageStorageStatusFailed || stored.NextRetryAt != nil {
			t.Fatalf("final attempt should be terminal failure: %+v", stored)
		}
	}
}

func TestCompleteStorageSyncPersistsMetadataAndUsageOnce(t *testing.T) {
	initStorageSyncTestDB(t)
	db := database.GetDB().DB
	bucket := models.Buckets{
		Id:       2,
		Name:     "telegram",
		Type:     "telegram",
		Capacity: 0,
		Config:   map[string]any{},
	}
	if err := db.Create(&bucket).Error; err != nil {
		t.Fatalf("create bucket: %v", err)
	}
	replica := models.ImageStorage{
		ImageID:  1,
		BucketID: bucket.Id,
		Storage:  bucket.Type,
		Status:   models.ImageStorageStatusUploading,
	}
	if err := db.Create(&replica).Error; err != nil {
		t.Fatalf("create replica: %v", err)
	}
	artifact := localStorageArtifact{
		URL:           "/uploads/example.webp",
		Thumbnail:     "/uploads/thumbnails/example.webp",
		FileSize:      10,
		ThumbnailSize: 2,
	}
	metadata := map[string]any{"tg_message_id": 123}
	if err := completeStorageSync(replica.ID, bucket, artifact, metadata); err != nil {
		t.Fatalf("complete sync: %v", err)
	}
	// A duplicate completion must not charge usage twice.
	if err := completeStorageSync(replica.ID, bucket, artifact, metadata); err != nil {
		t.Fatalf("repeat completion: %v", err)
	}

	var stored models.ImageStorage
	if err := db.First(&stored, replica.ID).Error; err != nil {
		t.Fatalf("reload replica: %v", err)
	}
	if stored.Status != models.ImageStorageStatusSuccess || metadataInt(stored.Metadata, "tg_message_id") != 123 {
		t.Fatalf("unexpected completed replica: %+v", stored)
	}
	var storedBucket models.Buckets
	if err := db.First(&storedBucket, bucket.Id).Error; err != nil {
		t.Fatalf("reload bucket: %v", err)
	}
	if storedBucket.Usage != 12 {
		t.Fatalf("usage should be charged once; got %d", storedBucket.Usage)
	}
}

func initStorageSyncTestDB(t *testing.T) {
	t.Helper()
	database.InitDB(&config.Config{
		DbType:     "sqlite",
		SqlitePath: filepath.Join(t.TempDir(), "storage-sync.db"),
	})
}
