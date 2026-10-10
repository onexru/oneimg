package models

import (
	"gorm.io/gorm"
	"time"
)

// Folder is an account-owned logical gallery grouping, never a storage path.
// NameKey is a hash of the Unicode-normalized, case-folded name, so uniqueness
// does not depend on the database's collation. ID zero means unfiled (no row).
type Folder struct {
	ID               int            `json:"id" gorm:"primaryKey;autoIncrement"`
	UserID           int            `json:"-" gorm:"not null;uniqueIndex:idx_folder_owner_name,priority:1;index"`
	Name             string         `json:"name" gorm:"size:64;not null"`
	Description      string         `json:"description" gorm:"type:text;not null;default:''"`
	NameKey          string         `json:"-" gorm:"size:64;not null;uniqueIndex:idx_folder_owner_name,priority:2"`
	Deleting         bool           `json:"deleting" gorm:"not null;default:false"`
	DeleteLease      string         `json:"-" gorm:"size:36"`
	DeleteLeaseUntil *time.Time     `json:"-"`
	DeletedAt        gorm.DeletedAt `json:"-" gorm:"index"`
	CreatedAt        time.Time      `json:"created_at"`
	UpdatedAt        time.Time      `json:"-"`
}
