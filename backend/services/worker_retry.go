package services

import (
	"context"
	"gorm.io/gorm"
	"oneimg/backend/database"
	"oneimg/backend/models"
	storageSettings "oneimg/backend/utils/settings"
	"time"
)

// Scheduled work is active, even while waiting for a retry. Do not allow idle
// backoff to overshoot its due time. New tasks still use immediate wake signals.
func workerRetryDelay(idle time.Duration, due, now time.Time) time.Duration {
	if due.IsZero() {
		return idle
	}
	delay := due.Sub(now)
	if delay < 10*time.Millisecond {
		delay = 10 * time.Millisecond
	}
	if delay < idle {
		return delay
	}
	return idle
}
func nextWorkerRetry(db *gorm.DB, model interface{}, where string, args ...interface{}) time.Time {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var retry struct{ NextRetryAt *time.Time }
	result := db.WithContext(ctx).Model(model).Select("next_retry_at").Where(where, args...).Where("next_retry_at > ?", time.Now()).Order("next_retry_at ASC").Limit(1).Find(&retry)
	if result.Error != nil || retry.NextRetryAt == nil {
		return time.Time{}
	}
	return *retry.NextRetryAt
}
func nextDirectWorkerRetry() time.Time {
	db, err := directDB(context.Background())
	if err != nil {
		return time.Time{}
	}
	return nextWorkerRetry(db, &models.DirectUploadTask{}, "expires_at > ? AND ((status = ? AND attempts < ?) OR (status = ? AND thumbnail_status = ? AND thumbnail_attempts < ?))", time.Now(), "queued", directMaxAttempts, "ready", "pending", directMaxAttempts)
}
func nextStorageWorkerRetry() time.Time {
	db := database.GetDB()
	if db == nil || db.DB == nil {
		return time.Time{}
	}
	setting, err := storageSettings.GetSettings()
	if err != nil || !setting.MultiStorageSync {
		return time.Time{}
	}
	return nextWorkerRetry(db.DB, &models.ImageStorage{}, "status = ? AND retry_count < ?", models.ImageStorageStatusPending, storageSyncMaxAttempts)
}
