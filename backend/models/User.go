package models

// 用户模型
type User struct {
	Id       int    `gorm:"primaryKey;column:id;autoIncrement" json:"id"`
	Role     int    `gorm:"default:1;uniqueIndex:unique_idx" json:"role"`
	Username string `gorm:"unique;not null" json:"username"`
	Password string `gorm:"not null" json:"password"`
}
