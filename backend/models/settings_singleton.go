package models

import (
	"fmt"
	"gorm.io/gorm"
)

// ORM writes and database constraints agree; migrations normalize legacy IDs
// before adding constraints. Zero ID means the global singleton, never a new row.
func (s *Settings) BeforeSave(tx *gorm.DB) error {
	if s.ID == 0 {
		s.ID = 1
	}
	if s.ID != 1 {
		return fmt.Errorf("settings ID must be 1")
	}
	return nil
}
