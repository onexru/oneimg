package services

import (
	"context"
	"errors"
	"gorm.io/gorm"
	"oneimg/backend/models"
	"oneimg/backend/utils/images"
	"sync"
	"time"
)

var directStartOnce sync.Once

var directWake = make(chan struct{}, 1)

func StartDirectUploadWorker() {
	// No init() side effect: only the application's explicit startup starts work.
	db, err := directDB(context.Background())
	if err != nil {
		return
	}
	directStartOnce.Do(func() {
		go func() {
			// Retry recovery on DB unavailability instead of silently abandoning claims.
			for {
				ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
				err := directRecover(db.WithContext(ctx))
				cancel()
				if err == nil {
					break
				}
				time.Sleep(5 * time.Second)
			}
			runDurableWorker(context.Background(), directWake, processNextDirectUpload, directCleanup, time.Minute, nextDirectWorkerRetry)
		}()
	})
	WakeDirectUploadWorker()
}

func WakeDirectUploadWorker() {
	select {
	case directWake <- struct{}{}:
	default:
	}
}

func directRecover(db *gorm.DB) error {
	return db.Transaction(func(tx *gorm.DB) error {
		updates := []struct {
			where  string
			args   []any
			fields map[string]any
		}{
			{"status = ?", []any{"receiving"}, map[string]any{"status": "awaiting_upload", "error_code": "transfer_interrupted", "message": "上次传输中断，请重新选择原文件", "retryable": true}},
			{"status = ? AND attempts < ?", []any{"processing", directMaxAttempts}, map[string]any{"status": "queued", "next_retry_at": nil}},
			{"status = ? AND attempts >= ?", []any{"processing", directMaxAttempts}, map[string]any{"status": "failed", "next_retry_at": nil, "error_code": "processing_interrupted", "message": "处理多次中断，请手动重试", "retryable": true}},
			{"status = ? AND thumbnail_status = ? AND thumbnail_attempts < ?", []any{"ready", "processing", directMaxAttempts}, map[string]any{"thumbnail_status": "pending", "next_retry_at": nil}},
			{"status = ? AND thumbnail_status = ? AND thumbnail_attempts >= ?", []any{"ready", "processing", directMaxAttempts}, map[string]any{"thumbnail_status": "failed", "next_retry_at": nil, "error_code": "thumbnail_interrupted", "message": "缩略图处理多次中断，主图仍可使用", "retryable": true}},
		}
		for _, u := range updates {
			if err := tx.Model(&models.DirectUploadTask{}).Where(u.where, u.args...).Updates(u.fields).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func processNextDirectUpload() bool {
	// Shares the lock with all legacy replica deletion operations. The explicit
	// deletion hook additionally fences the gap before controllers delete rows.
	storageReplicaOperationMu.Lock()
	defer storageReplicaOperationMu.Unlock()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	db, err := directDB(ctx)
	if err != nil {
		return false
	}
	directMu.Lock()
	var t models.DirectUploadTask
	q := db.Where("expires_at > ? AND (next_retry_at IS NULL OR next_retry_at <= ?) AND ((status = ? AND attempts < ?) OR (status = ? AND thumbnail_status = ? AND thumbnail_attempts < ?))", time.Now(), time.Now(), "queued", directMaxAttempts, "ready", "pending", directMaxAttempts).Order("created_at ASC").Limit(1).Find(&t)
	if q.Error != nil || q.RowsAffected == 0 {
		directMu.Unlock()
		return false
	}
	thumb := t.Status == "ready"
	fields := map[string]any{"status": "processing", "attempts": t.Attempts + 1, "next_retry_at": nil}
	if thumb {
		fields = map[string]any{"thumbnail_status": "processing", "thumbnail_attempts": t.ThumbnailAttempts + 1, "next_retry_at": nil}
	}
	q = db.Model(&models.DirectUploadTask{}).Where("id = ? AND status = ? AND thumbnail_status = ?", t.ID, t.Status, t.ThumbnailStatus).Updates(fields)
	directMu.Unlock()
	if q.Error != nil {
		return false
	}
	if q.RowsAffected == 0 {
		return true
	}
	if thumb {
		t.ThumbnailStatus = "processing"
		t.ThumbnailAttempts++
	} else {
		t.Status = "processing"
		t.Attempts++
	}
	err = directRunSafe(func() error {
		if thumb {
			return directProcessThumbnail(ctx, db, t)
		}
		return directProcessMain(ctx, db, t)
	})
	if err != nil {
		finishCtx, finishCancel := context.WithTimeout(context.Background(), 15*time.Second)
		directMarkFailed(db.WithContext(finishCtx), t, thumb, err)
		finishCancel()
	}
	return true
}

func directRunSafe(fn func() error) (err error) {
	defer func() {
		if recover() != nil {
			err = directError(422, "invalid_image", "图片解码失败或格式损坏，请选择其他文件", false)
		}
	}()
	return fn()
}

func directImageError(err error) error {
	if errors.Is(err, images.ErrImageDimensions) {
		return directError(422, "image_dimensions", "图片尺寸过大（最多 1600 万像素、单边 8192；动图最多 200 帧和 6400 万帧像素）", false)
	}
	if errors.Is(err, images.ErrUnsupportedFormat) {
		return directError(415, "invalid_image", "实际文件格式不受支持或与声明不一致", false)
	}
	return directError(422, "invalid_image", "图片解码失败或文件已损坏", false)
}
