package services

import (
	"context"
	"errors"
	"fmt"
	"gorm.io/gorm"
	"oneimg/backend/database"
	"oneimg/backend/models"
	"path/filepath"
)

// DeleteImageReplicas keeps local canonical bytes until all remote deletes have
// succeeded. Operation serialization is shared with the durable sync worker.
func DeleteImageReplicas(ctx context.Context, image models.Image) error {
	if err := acquireStorageOperation(ctx, &storageReplicaOperationMu); err != nil {
		return err
	}
	defer storageReplicaOperationMu.Unlock()
	db := database.GetDB()
	if db == nil || db.DB == nil {
		return errors.New("database is not initialized")
	}
	return deleteImageReplicasIncremental(ctx, db.DB, image, deleteReplicaArtifact)
}

func DeleteBucketReplicas(ctx context.Context, bucket models.Buckets) error {
	storageReplicaOperationMu.Lock()
	defer storageReplicaOperationMu.Unlock()
	if bucket.Type == "default" {
		return errors.New("the default local bucket cannot be deleted")
	}
	db := database.GetDB()
	if db == nil || db.DB == nil {
		return errors.New("database is not initialized")
	}
	var replicas []models.ImageStorage
	if err := db.DB.Where("bucket_id = ?", bucket.Id).Order("id ASC").Find(&replicas).Error; err != nil {
		return err
	}
	var failures []error
	for i := range replicas {
		r := &replicas[i]
		var image models.Image
		err := db.DB.First(&image, r.ImageID).Error
		if err != nil {
			if !errors.Is(err, gorm.ErrRecordNotFound) {
				failures = append(failures, err)
				continue
			}
			image = models.Image{Id: r.ImageID, Url: r.URL, Thumbnail: r.Thumbnail, FileName: filepath.Base(r.URL)}
		}
		if r.Metadata == nil {
			r.Metadata = map[string]any{}
		}
		if r.Status != models.ImageStorageStatusDeleting {
			r.Metadata["delete_charged"] = r.Status == models.ImageStorageStatusSuccess || r.SyncedAt != nil
			r.Status = models.ImageStorageStatusDeleting
			r.NextRetryAt = nil
			if err := db.DB.Model(r).Select("Status", "Metadata", "NextRetryAt").Updates(r).Error; err != nil {
				failures = append(failures, err)
				continue
			}
		}
		failed := false
		for _, thumb := range []bool{false, true} {
			key := "delete_main_done"
			if thumb {
				key = "delete_thumb_done"
			}
			if r.Metadata[key] == true {
				continue
			}
			if err := deleteReplicaArtifact(ctx, image, bucket, *r, thumb); err != nil {
				failures = append(failures, fmt.Errorf("delete replica %d: %w", r.ID, err))
				failed = true
				continue
			}
			if err := recordDeletedArtifact(db.DB, bucket, r, thumb); err != nil {
				failures = append(failures, err)
				failed = true
			}
		}
		if !failed {
			if err := db.DB.Transaction(func(tx *gorm.DB) error {
				if err := tx.Model(&models.Image{}).Where("id = ? AND access_bucket_id = ?", r.ImageID, bucket.Id).Update("access_bucket_id", 0).Error; err != nil {
					return err
				}
				return removeDeletedReplicaManifest(tx, image, *r)
			}); err != nil {
				failures = append(failures, err)
			}
		}
	}
	return errors.Join(failures...)
}
