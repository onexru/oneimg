package services

import (
	"context"
	"errors"
	"fmt"
	"log"
	"oneimg/backend/config"
	"oneimg/backend/database"
	"oneimg/backend/models"
	"oneimg/backend/storage"
	"path/filepath"
	"time"
)

type localStorageArtifact struct {
	URL           string
	Thumbnail     string
	FileName      string
	MimeType      string
	FileSize      int64
	ThumbnailSize int64
}

func synchronizeReplica(ctx context.Context, replica *models.ImageStorage) (map[string]any, error) {
	db := database.GetDB().DB

	var image models.Image
	if err := db.First(&image, replica.ImageID).Error; err != nil {
		return nil, fmt.Errorf("load image %d: %w", replica.ImageID, err)
	}

	var bucket models.Buckets
	if err := db.First(&bucket, replica.BucketID).Error; err != nil {
		return nil, fmt.Errorf("load bucket %d: %w", replica.BucketID, err)
	}
	if bucket.Disabled {
		return nil, fmt.Errorf("storage source %d is temporarily disabled", bucket.Id)
	}

	artifact, err := buildLocalStorageArtifactContext(ctx, image)
	if err != nil {
		return nil, err
	}

	if err := checkStorageCapacity(bucket, artifact.FileSize+artifact.ThumbnailSize); err != nil {
		return nil, err
	}

	// A previous Telegram attempt may have accepted a main message before its
	// thumbnail failed. Clean its persisted IDs before overwriting metadata.
	if bucket.Type == "telegram" && replica.Metadata != nil {
		if err := deleteRemoteReplica(ctx, image, bucket, *replica); err != nil {
			return replica.Metadata, err
		}
	}
	metadata, err := uploadStorageArtifact(ctx, bucket, artifact)
	if err != nil {
		cleanupReplicaAfterFailedUpload(image, bucket, *replica, metadata, err)
		return metadata, err
	}

	if err := completeStorageSync(replica.ID, bucket, artifact, metadata); err != nil {
		var latest models.ImageStorage
		if loadErr := db.First(&latest, replica.ID).Error; loadErr == nil && latest.Status == models.ImageStorageStatusSuccess {
			return metadata, nil
		}
		cleanupReplicaAfterFailedUpload(image, bucket, *replica, metadata, err)
		return metadata, err
	}

	log.Printf("[storage-sync] image %d synchronized to bucket %d (%s)", image.Id, bucket.Id, bucket.Type)
	return metadata, nil
}

func cleanupReplicaAfterFailedUpload(image models.Image, bucket models.Buckets, replica models.ImageStorage, metadata map[string]any, uploadErr error) {
	if bucket.Type == "default" {
		return
	}
	replica.URL = image.Url
	replica.Thumbnail = image.Thumbnail
	replica.Metadata = metadata
	cleanupContext, cancelCleanup := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancelCleanup()
	if cleanupErr := deleteRemoteReplica(cleanupContext, image, bucket, replica); cleanupErr != nil {
		log.Printf("[storage-sync] cleanup after failed upload also failed (task=%d, upload=%v): %v", replica.ID, uploadErr, cleanupErr)
	}
}

func buildLocalStorageArtifact(image models.Image) (localStorageArtifact, error) {
	return buildLocalStorageArtifactContext(context.Background(), image)
}
func buildLocalStorageArtifactContext(ctx context.Context, image models.Image) (localStorageArtifact, error) {
	source := storage.NewLocal(config.UploadRoot())
	mainInfo, err := source.Stat(ctx, storage.Ref{Key: image.Url})
	if err != nil {
		return localStorageArtifact{}, fmt.Errorf("stat local image %q: %w", image.Url, err)
	}

	artifact := localStorageArtifact{
		URL:      image.Url,
		FileName: image.FileName,
		MimeType: image.MimeType,
		FileSize: mainInfo.Size,
	}
	if artifact.FileName == "" {
		artifact.FileName = filepath.Base(image.Url)
	}
	if artifact.MimeType == "" {
		artifact.MimeType = "application/octet-stream"
	}

	if image.Thumbnail != "" {
		thumbnailInfo, statErr := source.Stat(ctx, storage.Ref{Key: image.Thumbnail})
		if statErr != nil {
			return localStorageArtifact{}, fmt.Errorf("stat local thumbnail %q: %w", image.Thumbnail, statErr)
		}
		artifact.Thumbnail = image.Thumbnail
		artifact.ThumbnailSize = thumbnailInfo.Size
	}

	return artifact, nil
}

func checkStorageCapacity(bucket models.Buckets, size int64) error {
	if size < 0 {
		return errors.New("replica size cannot be negative")
	}
	if bucket.Type == "default" || bucket.Type == "telegram" || bucket.Capacity == 0 {
		return nil
	}
	required := uint64(size)
	if required > bucket.Capacity || bucket.Usage > bucket.Capacity-required {
		return fmt.Errorf("bucket %d has insufficient capacity", bucket.Id)
	}
	return nil
}
