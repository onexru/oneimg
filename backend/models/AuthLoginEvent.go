package models

import (
	"time"

	"gorm.io/gorm"
)

// AuthLoginEvent records an account sign-in independently of its session lifetime.
// Store only an owner, a timestamp and the allowlisted authentication method:
// never bearer cookies, session handles, passwords, SSO tokens or request metadata.
// UserID deliberately is not a foreign key: guests are excluded by the recorder.
type AuthLoginEvent struct {
	ID        uint64    `gorm:"primaryKey;autoIncrement" json:"-"`
	UserID    int       `gorm:"not null;index:idx_auth_login_owner_created,priority:1" json:"-"`
	CreatedAt time.Time `gorm:"not null;index;index:idx_auth_login_owner_created,priority:2" json:"created_at"`
	Method    string    `gorm:"type:varchar(16);not null" json:"method"`
}

// SQLite compares stored timestamps as text. Normalize offsets on every write
// so local-time fixtures and UTC retention predicates share the same ordering.
func (event *AuthLoginEvent) BeforeSave(_ *gorm.DB) error {
	if event.CreatedAt.IsZero() {
		event.CreatedAt = time.Now().UTC()
	} else {
		event.CreatedAt = event.CreatedAt.UTC()
	}
	return nil
}
