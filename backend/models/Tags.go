package models

type Tags struct {
	Id int `json:"id" gorm:"type:integer;primaryKey;autoIncrement"`
	// Keep the legacy unique constraint as well as the named index: GORM
	// must not try dropping a guessed constraint name on upgraded PostgreSQL.
	Name string `json:"name" gorm:"not null;default:'';unique;uniqueIndex:name;size:50"`
}
