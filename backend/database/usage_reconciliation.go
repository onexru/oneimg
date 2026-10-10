package database

import (
	"fmt"
	"math"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"oneimg/backend/models"
)

// BucketUsageReconciliation is an immutable audit record of the one-time
// migration from legacy disk usage to accounted image artifact bytes.
type BucketUsageReconciliation struct {
	BucketID       int       `gorm:"primaryKey;autoIncrement:false"`
	PreviousUsage  uint64    `gorm:"not null"`
	AccountedUsage uint64    `gorm:"not null"`
	ReconciledAt   time.Time `gorm:"not null"`
}

// ReconcileLegacyBucketUsage runs before workers start, after replica backfill.
// Successful replicas are charged; partial deletions retain only the remaining
// bytes. Failed unpublished cleanup manifests explicitly marked uncharged are
// excluded. A per-bucket audit marker prevents every restart from resetting usage.
func ReconcileLegacyBucketUsage(db *gorm.DB) error {
	if err := db.AutoMigrate(&BucketUsageReconciliation{}); err != nil {
		return err
	}
	return db.Transaction(func(tx *gorm.DB) error {
		var buckets []models.Buckets
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Order("id ASC").Find(&buckets).Error; err != nil {
			return err
		}
		for _, bucket := range buckets {
			var recorded int64
			if err := tx.Model(&BucketUsageReconciliation{}).Where("bucket_id = ?", bucket.Id).Count(&recorded).Error; err != nil {
				return err
			}
			if recorded > 0 {
				continue
			}
			var replicas []models.ImageStorage
			if err := tx.Where("bucket_id = ?", bucket.Id).Find(&replicas).Error; err != nil {
				return err
			}
			var usage uint64
			for _, r := range replicas {
				charged := r.Status == models.ImageStorageStatusSuccess || r.SyncedAt != nil
				if r.Status == models.ImageStorageStatusDeleting {
					charged, _ = r.Metadata["delete_charged"].(bool)
				}
				if !charged {
					continue
				}
				if r.FileSize < 0 || r.ThumbnailSize < 0 {
					return fmt.Errorf("negative artifact accounting in replica %d", r.ID)
				}
				for _, size := range []int64{r.FileSize, r.ThumbnailSize} {
					if uint64(size) > math.MaxInt64-usage {
						return fmt.Errorf("artifact accounting overflow in bucket %d", bucket.Id)
					}
					usage += uint64(size)
				}
			}
			if err := tx.Model(&models.Buckets{}).Where("id = ?", bucket.Id).UpdateColumn("usage", usage).Error; err != nil {
				return err
			}
			audit := BucketUsageReconciliation{BucketID: bucket.Id, PreviousUsage: bucket.Usage, AccountedUsage: usage, ReconciledAt: time.Now()}
			if err := tx.Create(&audit).Error; err != nil {
				return err
			}
		}
		return nil
	})
}
