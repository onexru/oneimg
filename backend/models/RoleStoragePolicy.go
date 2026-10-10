package models

// An absent row preserves legacy role behavior. Custom [] is an intentional
// empty inherited pool, not a fallback to legacy defaults.
type RoleStoragePolicy struct {
	Role      int    `json:"role" gorm:"primaryKey;autoIncrement:false"`
	Mode      string `json:"mode" gorm:"not null"`
	BucketIDs []int  `json:"bucket_ids" gorm:"type:text;serializer:json;not null"`
}
