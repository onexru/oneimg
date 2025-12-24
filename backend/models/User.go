package models

// 用户模型
type User struct {
	ID       int    `json:"id" gorm:"type:integer;primaryKey;autoIncrement"`
	Role     int    `json:"role" gorm:"default:1;uniqueIndex:unique_idx"`
	Username string `json:"username" gorm:"unique;not null"`
	Password string `json:"password" gorm:"not null"`
}
