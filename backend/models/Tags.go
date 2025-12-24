package models

type Tags struct {
	Id   int    `json:"id" gorm:"type:integer;primaryKey;autoIncrement"`
	Name string `json:"name" gorm:"not null;default:'';uniqueIndex:name;size:50"`
}
