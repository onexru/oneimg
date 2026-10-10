package services

import (
	"context"
	"errors"
	"time"

	"oneimg/backend/models"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const folderDeleteBatchSize = 20

// Counts describe this invocation, except RemainingCount/PendingUploadCount.
// Incomplete work is never reported as successful deletion. Repeat the same
// confirmed request to continue after a batch, storage failure or restart.
type GalleryFolderDeleteResult struct {
	Deleted            bool  `json:"deleted"`
	Deleting           bool  `json:"deleting"`
	DeletedCount       int   `json:"deleted_count"`
	FailedCount        int   `json:"failed_count"`
	RemainingCount     int64 `json:"remaining_count"`
	PendingUploadCount int64 `json:"pending_upload_count"`
	Retryable          bool  `json:"retryable"`
}

func DeleteGalleryFolderWithImages(ctx context.Context, db *gorm.DB, ownerID, folderID int, confirmName string) (GalleryFolderDeleteResult, error) {
	result := GalleryFolderDeleteResult{}
	ctx, cancel := context.WithTimeout(ctx, 25*time.Second)
	defer cancel()
	db = db.WithContext(ctx)
	lease := uuid.NewString()
	alreadyDeleted := false
	err := db.Transaction(func(tx *gorm.DB) error {
		if err := lockFolderOwner(tx, ownerID); err != nil {
			return err
		}
		var folder models.Folder
		if err := tx.Unscoped().Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ? AND user_id = ?", folderID, ownerID).First(&folder).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return folderNotFound()
			}
			return err
		}
		if confirmName == "" || confirmName != folder.Name {
			return folderError(400, "folder_confirmation_mismatch", "请输入完整且完全一致的文件夹名称以确认删除图片")
		}
		if folder.DeletedAt.Valid {
			alreadyDeleted = true
			return nil
		}
		if folder.DeleteLeaseUntil != nil && folder.DeleteLeaseUntil.After(time.Now()) {
			return folderError(409, "folder_delete_busy", "文件夹删除正在执行，请稍后重试")
		}
		// The owner lock fences moves, new tasks and main publication BEFORE
		// physical deletion. Never hold a DB row lock over a network request.
		return tx.Model(&folder).Updates(map[string]any{"deleting": true, "delete_lease": lease, "delete_lease_until": time.Now().Add(90 * time.Second)}).Error
	})
	if err != nil {
		return result, err
	}
	if alreadyDeleted {
		result.Deleted = true
		return result, nil
	}
	result.Deleting = true

	// Cancellation retains staging/orphan ledgers until signed PUT expiry;
	// bytes are swept by the existing direct cleanup worker, not forgotten.
	if err := cancelPendingFolderUploads(ctx, db, ownerID, folderID); err != nil {
		result.FailedCount++
	}
	var images []models.Image
	if err := db.Where("user_id = ? AND folder_id = ?", ownerID, folderID).Order("id ASC").Limit(folderDeleteBatchSize).Find(&images).Error; err != nil {
		result.FailedCount++
	} else {
		for _, image := range images {
			if ctx.Err() != nil {
				break
			}
			if err := DeleteImageCompletely(ctx, image.Id); err != nil {
				result.FailedCount++
				continue
			}
			result.DeletedCount++
		}
	}
	// The request may have timed out; use a small independent context only to
	// checkpoint counts/release the lease, never to continue deleting bytes.
	finishCtx, finishCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer finishCancel()
	err = db.WithContext(finishCtx).Transaction(func(tx *gorm.DB) error {
		if err := lockFolderOwner(tx, ownerID); err != nil {
			return err
		}
		var folder models.Folder
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ? AND user_id = ?", folderID, ownerID).First(&folder).Error; err != nil {
			return err
		}
		if folder.DeleteLease != lease {
			return folderError(409, "folder_delete_busy", "文件夹删除任务已变化，请刷新后重试")
		}
		if err := tx.Model(&models.Image{}).Where("user_id = ? AND folder_id = ?", ownerID, folderID).Count(&result.RemainingCount).Error; err != nil {
			return err
		}
		if err := tx.Model(&models.DirectUploadTask{}).Where("owner_id = ? AND folder_id = ? AND image_id = 0 AND (status <> ? OR reserved_bytes > 0)", ownerID, folderID, "cancelled").Count(&result.PendingUploadCount).Error; err != nil {
			return err
		}
		if result.RemainingCount == 0 && result.PendingUploadCount == 0 && result.FailedCount == 0 {
			if err := tx.Model(&models.DirectUploadTask{}).Where("owner_id = ? AND folder_id = ?", ownerID, folderID).Update("folder_id", 0).Error; err != nil {
				return err
			}
			// Tombstone prevents an already-accepted legacy upload from silently
			// resurfacing as unfiled after a destructive delete. Release the name.
			if err := tx.Model(&folder).Updates(map[string]any{"delete_lease": "", "delete_lease_until": nil}).Error; err != nil {
				return err
			}
			if err := retireGalleryFolder(tx, &folder); err != nil {
				return err
			}
			result.Deleted = true
			result.Deleting = false
			return nil
		}
		result.Retryable = true
		return tx.Model(&folder).Updates(map[string]any{"delete_lease": "", "delete_lease_until": nil}).Error
	})
	return result, err
}

func cancelPendingFolderUploads(ctx context.Context, db *gorm.DB, ownerID, folderID int) error {
	if err := acquireStorageOperation(ctx, &storageReplicaOperationMu); err != nil {
		return err
	}
	defer storageReplicaOperationMu.Unlock()
	if err := acquireStorageOperation(ctx, &directMu); err != nil {
		return err
	}
	defer directMu.Unlock()
	return db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return fenceDirectTasks(tx, "owner_id = ? AND folder_id = ? AND image_id = 0", []any{ownerID, folderID}, 100)
	})
}
