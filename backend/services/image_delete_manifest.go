package services

import (
	"gorm.io/gorm"
	"oneimg/backend/models"
)

// Retain legacy IDs until both artifact completion markers have been committed.
// Removing them in the physical-delete callback can lose the only retryable IDs
// if the subsequent database update fails, especially for main-only images.
func removeDeletedReplicaManifest(db *gorm.DB, image models.Image, r models.ImageStorage) error {
	return db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Delete(&r).Error; err != nil {
			return err
		}
		if r.Storage == "telegram" {
			return tx.Where("file_name = ?", image.FileName).Delete(&models.ImageTeleGram{}).Error
		}
		return nil
	})
}
