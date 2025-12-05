package models

// 用户模型
type User struct {
	Id       int    `json:"id"`
	Role     int    `json:"role"`
	Username string `json:"username"`
	Password string `json:"password"`
}
