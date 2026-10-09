package services

import (
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"oneimg/backend/models"
	"time"
)

func directMarkFailed(db *gorm.DB, t models.DirectUploadTask, thumb bool, cause error) error {
	directMu.Lock()
	defer directMu.Unlock()
	e := directAsError(cause)
	attempts := t.Attempts
	if thumb {
		attempts = t.ThumbnailAttempts
	}
	fields := map[string]any{"error_code": e.Code, "message": e.Message, "retryable": e.Retryable, "next_retry_at": nil}
	status := "failed"
	if e.Retryable && attempts < directMaxAttempts {
		if thumb {
			status = "pending"
		} else {
			status = "queued"
		}
		fields["next_retry_at"] = time.Now().Add(time.Duration(attempts*attempts) * 10 * time.Second)
	}
	where := "id = ? AND status = ?"
	args := []any{t.ID, "processing"}
	if thumb {
		where += " AND thumbnail_status = ?"
		args = []any{t.ID, "ready", "processing"}
		fields["thumbnail_status"] = status
	} else {
		fields["status"] = status
	}
	return db.Transaction(func(tx *gorm.DB) error {
		var current models.DirectUploadTask
		q := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where(where, args...).Find(&current)
		if q.Error != nil {
			return q.Error
		}
		if q.RowsAffected == 0 {
			return nil
		}
		// Recoverable main work retains its reservation for independent retries.
		// Thumbnail failure releases any provisional thumbnail charge immediately.
		if !e.Retryable || thumb {
			if err := directRelease(tx, current); err != nil {
				return err
			}
			fields["reserved_bytes"] = 0
		}
		return tx.Model(&models.DirectUploadTask{}).Where(where, args...).Updates(fields).Error
	})
}

func directRelease(tx *gorm.DB, t models.DirectUploadTask) error {
	var n int64
	if err := tx.Model(&models.Buckets{}).Where("id = ?", t.BucketID).Count(&n).Error; err != nil {
		return err
	}
	if n == 0 {
		return nil
	}
	return directQuota(tx, t.BucketID, -t.ReservedBytes)
}
