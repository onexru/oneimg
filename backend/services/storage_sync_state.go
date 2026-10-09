package services

import (
	"fmt"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"oneimg/backend/database"
	"oneimg/backend/models"
	"oneimg/backend/storage"
	"time"
)

func completeStorageSync(replicaID int, bucket models.Buckets, artifact localStorageArtifact, metadata map[string]any) error {
	db := database.GetDB().DB
	now := time.Now()
	totalSize := artifact.FileSize + artifact.ThumbnailSize

	return db.Transaction(func(tx *gorm.DB) error {
		var current models.ImageStorage
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&current, replicaID).Error; err != nil {
			return err
		}
		if current.Status == models.ImageStorageStatusSuccess {
			return nil
		}
		if current.Status != models.ImageStorageStatusUploading {
			return fmt.Errorf("task %d is no longer uploading (status=%s)", replicaID, current.Status)
		}

		if bucket.Type != "default" && totalSize > 0 {
			totalSizeUint := uint64(totalSize)
			usageColumn := database.UsageColumn(tx)
			usageUpdate := tx.Model(&models.Buckets{}).
				Where("id = ? AND (capacity = 0 OR type IN ('telegram','default') OR "+usageColumn+" + ? <= capacity)", bucket.Id, totalSizeUint).
				UpdateColumn("usage", gorm.Expr(usageColumn+" + ?", totalSizeUint))
			if usageUpdate.Error != nil {
				return usageUpdate.Error
			}
			if usageUpdate.RowsAffected == 0 {
				return fmt.Errorf("bucket %d has insufficient capacity", bucket.Id)
			}
		}

		current.Storage = bucket.Type
		current.Status = models.ImageStorageStatusSuccess
		current.URL = artifact.URL
		current.Thumbnail = artifact.Thumbnail
		current.FileSize = artifact.FileSize
		current.ThumbnailSize = artifact.ThumbnailSize
		current.Error = ""
		current.Metadata = metadata
		current.StartedAt = nil
		current.NextRetryAt = nil
		current.SyncedAt = &now
		result := tx.Model(&current).Where("id = ? AND status = ?", replicaID, models.ImageStorageStatusUploading).
			Select("Storage", "Status", "URL", "Thumbnail", "FileSize", "ThumbnailSize", "Error", "Metadata", "StartedAt", "NextRetryAt", "SyncedAt").Updates(&current)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return fmt.Errorf("task %d was modified before completion", replicaID)
		}
		return nil
	})
}

func markStorageSyncFailed(replicaID int, syncErr error, metadata map[string]any) error {
	db := database.GetDB().DB
	return db.Transaction(func(tx *gorm.DB) error {
		var current models.ImageStorage
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&current, replicaID).Error; err != nil {
			return err
		}
		if current.Status != models.ImageStorageStatusUploading {
			return nil
		}

		attempts := current.RetryCount + 1
		status := models.ImageStorageStatusFailed
		var nextRetryAt *time.Time
		if attempts < storageSyncMaxAttempts {
			status = models.ImageStorageStatusPending
			retryTime := time.Now().Add(time.Duration(1<<(attempts-1)) * 5 * time.Second)
			nextRetryAt = &retryTime
		}
		current.Status = status
		current.Error = syncErr.Error()
		current.RetryCount = attempts
		current.StartedAt = nil
		current.NextRetryAt = nextRetryAt
		if metadata != nil {
			current.Metadata = metadata
		}
		return tx.Model(&current).Where("id = ? AND status = ?", replicaID, models.ImageStorageStatusUploading).
			Select("Status", "Error", "RetryCount", "StartedAt", "NextRetryAt", "Metadata").Updates(&current).Error

	})
}

func metadataInt(metadata map[string]any, key string) int { return storage.MetadataInt(metadata, key) }
