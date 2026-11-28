package models

import "time"

// APICredential holds a one-way digest and explicit image/upload scopes. The
// legacy settings token is enrolled once with a finite lifetime, never admin.
type APICredential struct {
 ID string `gorm:"primaryKey;size:64" json:"-"`
 OwnerID int `json:"owner_id"`
 Scopes string `json:"scopes"`
 ExpiresAt time.Time `gorm:"index" json:"expires_at"`
 // NeverExpires marks a credential configured as permanent. ExpiresAt still
 // carries a far-future sentinel so every existing `expires_at > now` filter
 // and the expires_at index keep working, which is what makes old rows and old
 // queries compatible without a data migration.
 NeverExpires bool `gorm:"not null;default:false" json:"never_expires"`
 // ExpirySpec stores the configured lifetime ("permanent", "30d", "12h") so
 // the UI can round-trip the original choice instead of guessing it back from
 // a shrinking timestamp. Empty on rows created before this field existed.
 ExpirySpec string `gorm:"size:32;not null;default:''" json:"expiry_spec"`
 LastUsedAt *time.Time `json:"last_used_at"`
 Revoked bool `gorm:"not null;default:false" json:"revoked"`
 Legacy bool `json:"legacy"`
 CreatedAt time.Time `json:"created_at"`
}

// Usable reports whether the credential may authenticate a request now. Kept on
// the model so the middleware, the settings view and the expiry editor cannot
// drift apart on the "permanent means no expiry" rule.
func (c APICredential) Usable(now time.Time) bool {
 if c.Revoked || c.OwnerID <= 0 {
  return false
 }
 return c.NeverExpires || now.Before(c.ExpiresAt)
}
