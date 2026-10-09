package services

import (
	"context"
	"errors"
	"gorm.io/gorm"
	"oneimg/backend/models"
	"time"
)

func RetryDirectUpload(ctx context.Context, ownerID int, id string) (DirectUploadResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	directMu.Lock()
	defer directMu.Unlock()
	db, err := directDB(ctx)
	if err != nil {
		return DirectUploadResponse{}, err
	}
	t, err := directTask(db, ownerID, id, true)
	if err != nil {
		return DirectUploadResponse{}, err
	}
	if time.Now().After(t.ExpiresAt) {
		return DirectUploadResponse{}, directError(410, "task_expired", "任务已过期，请重新选择文件", false)
	}
	fields := map[string]any{"error_code": "", "message": "", "retryable": false, "next_retry_at": nil}
	if t.Status == "ready" && t.ThumbnailStatus == "failed" && t.Retryable {
		fields["thumbnail_status"] = "pending"
		fields["thumbnail_attempts"] = 0
	} else if t.Status == "failed" && t.Retryable {
		fields["status"] = "queued"
		fields["attempts"] = 0
	} else if t.Status == "awaiting_upload" {
		return DirectUploadResponse{}, directError(409, "file_required", "请重新选择原文件继续传输", true)
	} else {
		return directResponse(db, t)
	}
	if time.Since(t.UpdatedAt) < 5*time.Second {
		return DirectUploadResponse{}, directError(429, "retry_too_soon", "请稍等片刻再重试", true)
	}
	if err = db.Model(&models.DirectUploadTask{}).Where("id = ? AND status = ? AND thumbnail_status = ?", id, t.Status, t.ThumbnailStatus).Updates(fields).Error; err != nil {
		return DirectUploadResponse{}, directDBError(err)
	}
	WakeDirectUploadWorker()
	return GetDirectUpload(ctx, ownerID, id)
}

func CancelDirectUpload(ctx context.Context, ownerID int, id string) (DirectUploadResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	directMu.Lock()
	defer directMu.Unlock()
	db, err := directDB(ctx)
	if err != nil {
		return DirectUploadResponse{}, err
	}
	// Cancellation must remain possible after bucket removal/permission revocation;
	// owner is still authenticated, and no new remote bytes can be published.
	t, err := directTask(db, ownerID, id, false)
	if err != nil {
		return DirectUploadResponse{}, err
	}
	if t.Status == "ready" || t.Status == "cancelled" {
		return directResponse(db, t)
	}
	err = db.Transaction(func(tx *gorm.DB) error {
		q := tx.Model(&models.DirectUploadTask{}).Where("id = ? AND status = ?", id, t.Status).Updates(map[string]any{"status": "cancelled", "reserved_bytes": 0, "retryable": false, "error_code": "cancelled", "message": "上传已取消", "next_retry_at": nil})
		if q.Error != nil {
			return q.Error
		}
		if q.RowsAffected != 1 {
			return directError(409, "task_changed", "任务状态已改变，请刷新", true)
		}
		// A deleted bucket has no remaining accounting row to release.
		var n int64
		if e := tx.Model(&models.Buckets{}).Where("id = ?", t.BucketID).Count(&n).Error; e != nil {
			return e
		}
		if n == 0 {
			return nil
		}
		return directQuota(tx, t.BucketID, -t.ReservedBytes)
	})
	if err != nil {
		return DirectUploadResponse{}, directDBError(err)
	}
	WakeDirectUploadWorker()
	return GetDirectUpload(ctx, ownerID, id)
}

func directAsError(err error) *DirectUploadError {
	var folderErr *GalleryFolderError
	if errors.As(err, &folderErr) {
		return directError(folderErr.Status, folderErr.Code, folderErr.Message, false)
	}
	var e *DirectUploadError
	if errors.As(err, &e) {
		return e
	}
	return directError(503, "processing_failed", "图片处理暂时失败，请重试", true)
}
