package services

import (
	"context"
	"errors"
	"fmt"
	"gorm.io/gorm"
	"oneimg/backend/database"
	"oneimg/backend/models"
)

// Deletion never transitions to the synchronization worker's retryable failed
// state. Persist the fence before touching bytes, including after a restart.
func deleteImageReplicasIncremental(ctx context.Context, db *gorm.DB, image models.Image, deleteArtifact func(context.Context, models.Image, models.Buckets, models.ImageStorage, bool) error) error {
	var replicas []models.ImageStorage
	if err := db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&models.Image{}).Where("id = ?", image.Id).Update("deleting", true).Error; err != nil {
			return err
		}
		if err := tx.Where("image_id = ?", image.Id).Order("id ASC").Find(&replicas).Error; err != nil {
			return err
		}
		if len(replicas) == 0 && !image.Deleting {
			replicas = []models.ImageStorage{{ImageID: image.Id, BucketID: image.BucketId, Storage: image.Storage, Status: models.ImageStorageStatusSuccess, URL: image.Url, Thumbnail: image.Thumbnail, FileSize: image.FileSize}}
			if replicas[0].BucketID == 0 {
				replicas[0].BucketID = 1
			}
			if replicas[0].Storage == "" {
				replicas[0].Storage = "default"
			}
			if err := tx.Create(&replicas[0]).Error; err != nil {
				return err
			}
		}
		for i := range replicas {
			r := &replicas[i]
			if r.Metadata == nil {
				r.Metadata = map[string]any{}
			}
			if r.Status != models.ImageStorageStatusDeleting {
				r.Metadata["delete_charged"] = r.Status == models.ImageStorageStatusSuccess || r.SyncedAt != nil
				r.Status = models.ImageStorageStatusDeleting
				r.NextRetryAt = nil
				if err := tx.Model(r).Select("Status", "Metadata", "NextRetryAt").Updates(r).Error; err != nil {
					return err
				}
			}
		}
		return nil
	}); err != nil {
		return err
	}
	var failures []error
	// Keep durable local source until all remote artifacts have been removed.
	for _, local := range []bool{false, true} {
		if local && len(failures) > 0 {
			break
		}
		for i := range replicas {
			r := &replicas[i]
			if (r.Storage == "default") != local {
				continue
			}
			var bucket models.Buckets
			if err := db.First(&bucket, r.BucketID).Error; err != nil {
				failures = append(failures, err)
				continue
			}
			failed := false
			for _, thumb := range []bool{false, true} {
				key := "delete_main_done"
				path := r.URL
				if thumb {
					key = "delete_thumb_done"
					path = r.Thumbnail
				}
				if r.Metadata[key] == true {
					continue
				}
				// Empty replica paths for pending jobs are resolved only once from the
				// canonical Image. Done markers must never fall back to deleted originals.
				if path == "" {
					if thumb {
						path = image.Thumbnail
					} else {
						path = image.Url
					}
				}
				if path != "" {
					if err := deleteArtifact(ctx, image, bucket, *r, thumb); err != nil {
						failures = append(failures, fmt.Errorf("delete replica %d: %w", r.ID, err))
						failed = true
						if err := db.Model(r).Update("error", "存储文件删除失败，请重试").Error; err != nil {
							failures = append(failures, err)
						}
						continue
					}
				}
				if err := recordDeletedArtifact(db, bucket, r, thumb); err != nil {
					failures = append(failures, err)
					failed = true
				}
			}
			if !failed && r.Metadata["delete_main_done"] == true && r.Metadata["delete_thumb_done"] == true {
				if err := removeDeletedReplicaManifest(db, image, *r); err != nil {
					failures = append(failures, err)
				}
			}
		}
	}
	return errors.Join(failures...)
}

func recordDeletedArtifact(db *gorm.DB, bucket models.Buckets, r *models.ImageStorage, thumb bool) error {
	key := "delete_main_done"
	size := r.FileSize
	if thumb {
		key = "delete_thumb_done"
		size = r.ThumbnailSize
	}
	return db.Transaction(func(tx *gorm.DB) error {
		var current models.ImageStorage
		if err := tx.First(&current, r.ID).Error; err != nil {
			return err
		}
		if current.Metadata[key] == true {
			r.Metadata = current.Metadata
			return nil
		}
		if current.Metadata == nil {
			current.Metadata = map[string]any{}
		}
		// Legacy deleting rows may predate the charged marker. SyncedAt is the
		// durable evidence used by startup reconciliation too; never infer from
		// the deleting status alone (unpublished cleanup rows are uncharged).
		if _, exists := current.Metadata["delete_charged"]; !exists {
			current.Metadata["delete_charged"] = current.SyncedAt != nil
		}
		if current.Metadata["delete_charged"] == true && size > 0 {
			col := database.UsageColumn(tx)
			if err := tx.Model(&models.Buckets{}).Where("id = ?", bucket.Id).UpdateColumn("usage", gorm.Expr("CASE WHEN "+col+" >= ? THEN "+col+" - ? ELSE 0 END", size, size)).Error; err != nil {
				return err
			}
		}
		current.Metadata[key] = true
		// Struct-based updates keep the json serializer on Metadata; a map update
		// would bypass field serializers and fail on the raw map value.
		current.Error = ""
		if thumb {
			current.ThumbnailSize = 0
		} else {
			current.FileSize = 0
		}
		sel := []string{"metadata", "error", "file_size"}
		if thumb {
			sel = []string{"metadata", "error", "thumbnail_size"}
		}
		if err := tx.Model(&current).Select(sel[0], sel[1], sel[2]).Updates(&current).Error; err != nil {
			return err
		}
		r.Metadata = current.Metadata
		if thumb {
			r.ThumbnailSize = 0
		} else {
			r.FileSize = 0
		}
		return nil
	})
}
