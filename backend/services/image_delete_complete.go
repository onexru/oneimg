package services

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"oneimg/backend/database"
	"oneimg/backend/models"

	"gorm.io/gorm"
)

// acquireStorageOperation respects a request deadline while a worker holds the
// shared mutex across network I/O. This also keeps folder deletion leases bounded.
func acquireStorageOperation(ctx context.Context, mu *sync.Mutex) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if mu.TryLock() {
		return nil
	}
	ticker := time.NewTicker(20 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if mu.TryLock() {
				return nil
			}
		}
	}
}

// DeleteImageCompletely reuses the durable replica lifecycle. Authorization and
// any folder fence must already be applied by the caller. No SQL-only cascade.
func DeleteImageCompletely(ctx context.Context, imageID int) error {
	if err := CancelDirectUploadsForImage(ctx, imageID); err != nil {
		return err
	}
	if err := acquireStorageOperation(ctx, &storageReplicaOperationMu); err != nil {
		return err
	}
	defer storageReplicaOperationMu.Unlock()
	db := database.GetDB().DB.WithContext(ctx)
	var image models.Image
	if err := db.First(&image, imageID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		return err
	}
	if err := deleteImageReplicasIncremental(ctx, db, image, deleteReplicaArtifact); err != nil {
		return err
	}
	return db.Transaction(func(tx *gorm.DB) error {
		var remaining int64
		if err := tx.Model(&models.ImageStorage{}).Where("image_id = ?", imageID).Count(&remaining).Error; err != nil {
			return err
		}
		if remaining != 0 {
			return fmt.Errorf("replica cleanup incomplete")
		}
		if err := tx.Where("image_id = ?", imageID).Delete(&models.ImageToTags{}).Error; err != nil {
			return err
		}
		return tx.Delete(&image).Error
	})
}
