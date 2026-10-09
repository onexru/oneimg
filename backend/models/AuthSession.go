package models

import "time"

// AuthSession stores only a hash of the bearer cookie. Epochs prevent revoked
// sessions from being resurrected by an in-flight request.
type AuthSession struct {
 ID string `gorm:"primaryKey;size:64"`
 UserID int `gorm:"index"`
 UserVersion uint64
 Epoch uint64
 Values []byte
 ExpiresAt time.Time `gorm:"index"`
 CreatedAt time.Time
}

// AuthState provides an atomic, persistent global session revocation epoch.
type AuthState struct {
 ID int `gorm:"primaryKey;autoIncrement:false"`
 Epoch uint64 `gorm:"not null;default:0"`
}
