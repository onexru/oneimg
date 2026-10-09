package models

import "time"

// Positive sequence IDs here are exposed as NEGATIVE user IDs; they can never
// collide with users.id. OwnerKey is public, not authentication material.
type GuestIdentity struct {
 ID int `gorm:"primaryKey;autoIncrement"`
 OwnerKey string `gorm:"uniqueIndex;size:36;not null"`
 CredentialHash string `gorm:"uniqueIndex;size:64;not null" json:"-"`
 ExpiresAt time.Time `gorm:"index"`
 CreatedAt time.Time
}
