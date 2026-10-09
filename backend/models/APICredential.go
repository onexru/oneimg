package models

import "time"

// APICredential holds a one-way digest and explicit image/upload scopes. The
// legacy settings token is enrolled once with a finite lifetime, never admin.
type APICredential struct {
 ID string `gorm:"primaryKey;size:64" json:"-"`
 OwnerID int `json:"owner_id"`
 Scopes string `json:"scopes"`
 ExpiresAt time.Time `gorm:"index" json:"expires_at"`
 LastUsedAt *time.Time `json:"last_used_at"`
 Revoked bool `gorm:"not null;default:false" json:"revoked"`
 Legacy bool `json:"legacy"`
 CreatedAt time.Time `json:"created_at"`
}
