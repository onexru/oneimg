package authsecurity

import (
	"errors"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"oneimg/backend/models"
)

const (
	LoginHistoryLimit     = 100
	LoginHistoryRetention = 30 * 24 * time.Hour
	LoginMethodPassword   = "password"
	LoginMethodOIDC       = "oidc"
	LoginMethodCAS        = "cas"
)

// RecordSuccessfulLogin is called only after a password/OIDC/CAS sign-in has
// successfully issued its session. Session renewal, reissue, guest and API-token
// activity are not sign-ins. History failures must not invalidate a good login.
func RecordSuccessfulLogin(db *gorm.DB, userID int, method string) error {
	return recordSuccessfulLoginAt(db, userID, method, time.Now().UTC())
}

func recordSuccessfulLoginAt(db *gorm.DB, userID int, method string, now time.Time) error {
	if userID <= 0 {
		return nil
	}
	if db == nil {
		return errors.New("login history database unavailable")
	}
	switch method {
	case LoginMethodPassword, LoginMethodOIDC, LoginMethodCAS:
	default:
		return errors.New("unsupported login history method")
	}
	now = now.UTC()
	return db.Transaction(func(tx *gorm.DB) error {
		// SQLite obtains a writer lock before reading; PostgreSQL/MySQL serialize
		// writers for this owner. Lock before insert+prune so concurrent logins
		// cannot leave more than the per-owner retention cap in storage.
		locked := tx.Model(&models.User{}).Where("id = ?", userID).UpdateColumn("auth_version", gorm.Expr("auth_version"))
		if locked.Error != nil {
			return locked.Error
		}
		var owner models.User
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Select("id").First(&owner, userID).Error; err != nil {
			return err
		}
		if err := tx.Where("created_at <= ?", now.Add(-LoginHistoryRetention)).Delete(&models.AuthLoginEvent{}).Error; err != nil {
			return err
		}
		event := models.AuthLoginEvent{UserID: userID, Method: method, CreatedAt: now}
		if err := tx.Create(&event).Error; err != nil {
			return err
		}
		// Do not DELETE from a subquery selecting the same table (MySQL 1093).
		// Offset reads a single cutoff row and uses stable ID ordering for ties.
		var oldestKept models.AuthLoginEvent
		cutoff := tx.Where("user_id = ?", userID).Order("created_at DESC, id DESC").Offset(LoginHistoryLimit - 1).Limit(1).Find(&oldestKept)
		if cutoff.Error != nil {
			return cutoff.Error
		}
		if cutoff.RowsAffected == 0 {
			return nil
		}
		return tx.Where("user_id = ? AND (created_at < ? OR (created_at = ? AND id < ?))", userID, oldestKept.CreatedAt, oldestKept.CreatedAt, oldestKept.ID).Delete(&models.AuthLoginEvent{}).Error
	})
}

// LoginHistory always owner-filters and caps its snapshot, including when there
// has not been another login to trigger physical retention cleanup.
func LoginHistory(db *gorm.DB, userID int) ([]models.AuthLoginEvent, error) {
	if db == nil || userID <= 0 {
		return nil, errors.New("login history requires an account")
	}
	rows := make([]models.AuthLoginEvent, 0)
	err := db.Where("user_id = ? AND created_at > ?", userID, time.Now().UTC().Add(-LoginHistoryRetention)).Order("created_at DESC, id DESC").Limit(LoginHistoryLimit).Find(&rows).Error
	return rows, err
}
