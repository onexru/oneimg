package services

import (
	"context"
	"errors"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"oneimg/backend/config"
	"oneimg/backend/database"
	"oneimg/backend/models"
	"oneimg/backend/storage"
	"time"
)

// BackfillImageStorages creates one successful replica record for the current
// singular storage fields on every legacy Image. It is safe to run repeatedly.
func BackfillImageStorages() error {
	db := database.GetDB()
	if db == nil || db.DB == nil {
		return errors.New("database is not initialized")
	}

	var bucketList []models.Buckets
	if err := db.DB.Find(&bucketList).Error; err != nil {
		return err
	}
	bucketByID := make(map[int]models.Buckets, len(bucketList))
	for _, bucket := range bucketList {
		bucketByID[bucket.Id] = bucket
	}

	var telegramRows []models.ImageTeleGram
	if err := db.DB.Find(&telegramRows).Error; err != nil {
		return err
	}
	telegramByFilename := make(map[string]models.ImageTeleGram, len(telegramRows))
	for _, row := range telegramRows {
		if _, exists := telegramByFilename[row.FileName]; !exists {
			telegramByFilename[row.FileName] = row
		}
	}

	var images []models.Image
	result := db.DB.Order("id ASC").FindInBatches(&images, 200, func(tx *gorm.DB, _ int) error {
		for _, image := range images {
			bucketID := image.BucketId
			if bucketID == 0 {
				bucketID = 1
			}
			storageType := image.Storage
			if storageType == "" {
				storageType = bucketByID[bucketID].Type
			}
			if storageType == "" {
				storageType = "default"
			}

			thumbnailSize := int64(0)
			if storageType == "default" && image.Thumbnail != "" {
				if info, err := storage.NewLocal(config.UploadRoot()).Stat(context.Background(), storage.Ref{Key: image.Thumbnail}); err == nil {
					thumbnailSize = info.Size
				}
			}

			var metadata map[string]any
			if storageType == "telegram" {
				if legacy, ok := telegramByFilename[image.FileName]; ok {
					metadata = telegramMetadata(legacy)
				}
			}

			replica := models.ImageStorage{
				ImageID:       image.Id,
				BucketID:      bucketID,
				Storage:       storageType,
				Status:        models.ImageStorageStatusSuccess,
				URL:           image.Url,
				Thumbnail:     image.Thumbnail,
				FileSize:      image.FileSize,
				ThumbnailSize: thumbnailSize,
				Metadata:      metadata,
				SyncedAt:      timePointer(image.CreatedAt),
			}
			if err := tx.Clauses(clause.OnConflict{
				Columns:   []clause.Column{{Name: "image_id"}, {Name: "bucket_id"}},
				DoNothing: true,
			}).Create(&replica).Error; err != nil {
				return err
			}
		}
		return nil
	})
	return result.Error
}

func telegramMetadata(row models.ImageTeleGram) map[string]any { return storage.TelegramMetadata(row) }
func timePointer(value time.Time) *time.Time {
	if value.IsZero() {
		return nil
	}
	copy := value
	return &copy
}
