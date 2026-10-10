package services

import (
	"context"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"oneimg/backend/models"
	"time"
)

// CancelDirectUploadsForImage MUST precede physical replica deletion. It fences
// future thumbnails durably, including explicit retries and process restarts.
// It never removes the already-published image or its charged storage bytes.
func CancelDirectUploadsForImage(ctx context.Context, imageID int) error {
	return directDeleteFence(ctx, "image_id = ?", imageID)
}

// CancelDirectUploadsForBucket MUST precede bucket replica/config deletion.
func CancelDirectUploadsForBucket(ctx context.Context, bucketID int) error {
	// Keep credentials until every staging/orphan ledger has been swept. Disable
	// the source first so no new task can race into the deletion/cleanup gap.
	directMu.Lock()
	db, err := directDB(ctx)
	var existing int64
	if err == nil {
		err = db.Model(&models.DirectUploadTask{}).Where("bucket_id = ?", bucketID).Count(&existing).Error
	}
	if err == nil && existing == 0 {
		directMu.Unlock()
		return nil
	}
	if err == nil {
		err = db.Model(&models.Buckets{}).Where("id = ?", bucketID).Update("disabled", true).Error
	}
	directMu.Unlock()
	if err != nil {
		return directDBError(err)
	}
	if err = directDeleteFence(ctx, "bucket_id = ?", bucketID); err != nil {
		return err
	}
	var pending int64
	if err = db.Model(&models.DirectUploadTask{}).Where("bucket_id = ? AND cleaned_at IS NULL", bucketID).Count(&pending).Error; err != nil {
		return directDBError(err)
	}
	if pending > 0 {
		WakeDirectUploadWorker()
		return directError(409, "bucket_cleanup_pending", "存储源已停用，正在清理直传临时文件；请在上传链接到期并完成清理后重试删除（通常最多 20 分钟）", true)
	}
	return nil
}

func directDeleteFence(ctx context.Context, where string, id int) error {
	if err := acquireStorageOperation(ctx, &storageReplicaOperationMu); err != nil {
		return err
	}
	defer storageReplicaOperationMu.Unlock()
	if err := acquireStorageOperation(ctx, &directMu); err != nil {
		return err
	}
	defer directMu.Unlock()
	db, err := directDB(ctx)
	if err != nil {
		return err
	}
	return directDBError(db.Transaction(func(tx *gorm.DB) error {
		return fenceDirectTasks(tx, where, []any{id}, 0)
	}))
}

// Shared persistent cancellation ledger for image/bucket and folder deletion.
// Call under storageReplicaOperationMu then directMu; rows precede quota locks.
func fenceDirectTasks(tx *gorm.DB, where string, args []any, limit int) error {
	var tasks []models.DirectUploadTask
	q := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where(where, args...).Where("error_code <> ?", "image_deleted").Order("id ASC")
	if limit > 0 {
		q = q.Limit(limit)
	}
	if err := q.Find(&tasks).Error; err != nil {
		return err
	}
	for _, t := range tasks {
		if err := directRelease(tx, t); err != nil {
			return err
		}
		fields := map[string]any{"reserved_bytes": 0, "retryable": false, "error_code": "image_deleted", "message": "图片、文件夹或存储源已删除，后台处理已停止", "next_retry_at": nil}
		if t.Status == "ready" {
			if t.ThumbnailStatus != "ready" {
				fields["thumbnail_status"] = "disabled"
			}
		} else {
			fields["status"] = "cancelled"
		}
		if err := tx.Model(&models.DirectUploadTask{}).Where("id = ?", t.ID).Updates(fields).Error; err != nil {
			return err
		}
	}
	return nil
}

func directCleanup() {
	storageReplicaOperationMu.Lock()
	defer storageReplicaOperationMu.Unlock()
	directMu.Lock()
	defer directMu.Unlock()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	db, err := directDB(ctx)
	if err != nil {
		return
	}
	now := time.Now()
	// Expiry releases reservations even if the source is disabled/deleted.
	var expired []models.DirectUploadTask
	if db.Where("expires_at <= ? AND (status NOT IN ? OR reserved_bytes > 0 OR (status = ? AND (thumbnail_status IN ? OR retryable = ?)))", now, []string{"cancelled", "ready"}, "ready", []string{"pending", "processing"}, true).Limit(100).Find(&expired).Error != nil {
		return
	}
	for _, t := range expired {
		if err := db.Transaction(func(tx *gorm.DB) error {
			// Folder cancellation may have released this snapshot's reservation
			// on another process. Reload under the same task-before-bucket order.
			var current models.DirectUploadTask
			if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&current, "id = ?", t.ID).Error; err != nil {
				return err
			}
			t = current
			if t.ErrorCode == "image_deleted" {
				return nil
			}
			if err := directRelease(tx, t); err != nil {
				return err
			}
			fields := map[string]any{"reserved_bytes": 0, "retryable": false, "next_retry_at": nil, "error_code": "task_expired", "message": "任务已过期，请重新选择文件"}
			if t.Status == "ready" {
				if t.ThumbnailStatus != "ready" && t.ThumbnailStatus != "disabled" {
					fields["thumbnail_status"] = "failed"
				}
			} else {
				fields["status"] = "cancelled"
			}
			return tx.Model(&models.DirectUploadTask{}).Where("id = ?", t.ID).Updates(fields).Error
		}); err != nil {
			return
		}
	}
	var tasks []models.DirectUploadTask
	// Grace covers in-flight browser PUTs and a cancelled proxy request. After
	// expiry+grace no still-valid signature can recreate cleaned staging objects.
	cutoff := now.Add(-5 * time.Minute)
	if db.Where("cleaned_at IS NULL AND updated_at < ? AND (last_put_expires_at IS NULL OR last_put_expires_at < ?) AND (status = ? OR status = ? OR (status = ? AND retryable = ?))", cutoff, cutoff, "ready", "cancelled", "failed", false).Limit(50).Find(&tasks).Error != nil {
		return
	}
	for _, t := range tasks {
		var b models.Buckets
		if db.First(&b, t.BucketID).Error != nil {
			continue
		}
		store, err := directOpenStore(b)
		if err != nil {
			continue
		}
		keys := []string{t.DirectKey, t.ProxyKey}
		if t.Status != "ready" {
			keys = append(keys, t.FinalKey, t.ThumbnailKey)
		} else if t.ThumbnailStatus == "failed" || t.ThumbnailStatus == "disabled" {
			keys = append(keys, t.ThumbnailKey)
		}
		ok := true
		for _, key := range keys {
			if key != "" {
				if err := store.Delete(ctx, key); err != nil {
					ok = false
					break
				}
			}
		}
		// Pending thumbnail can later create an orphan, so retain its cleanup ledger.
		if ok && !(t.Status == "ready" && (t.ThumbnailStatus == "pending" || t.ThumbnailStatus == "processing" || t.ThumbnailStatus == "failed" && t.Retryable)) {
			if db.Model(&models.DirectUploadTask{}).Where("id = ?", t.ID).Update("cleaned_at", now).Error != nil {
				return
			}
		}
	}
}
