package models

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"time"
)

// 用户模型
type User struct {
	ID         int        `json:"id" gorm:"type:integer;primaryKey;autoIncrement"`
	Role       int        `json:"role" gorm:"default:1"`
	Username   string     `json:"username" gorm:"unique;not null"`
	Password   string     `json:"-" gorm:"not null"`
	Permission Permission `json:"permission" gorm:"type:jsonb"`
	CreatedAt  time.Time  `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt  time.Time  `json:"updated_at" gorm:"autoUpdateTime"`
}

const (
	SuperAdminID = 1
	RoleAdmin    = 1
	RoleGuest    = 2
	RoleUser     = 3
)

// 权限模型
type Permission struct {
	// 存储权限:存储桶ID列表
	Buckets []int `json:"buckets" gorm:"default:[]"`
}

// 写入数据库：结构体序列化为 JSON
func (p Permission) Value() (driver.Value, error) {
	return json.Marshal(p)
}

// 读取数据库：JSON 反序列化回结构体
func (p *Permission) Scan(src any) error {
	if src == nil {
		p.Buckets = []int{}
		return nil
	}
	var data []byte
	switch v := src.(type) {
	case []byte:
		data = v
	case string:
		data = []byte(v)
	default:
		return errors.New("invalid json source for Permission")
	}
	if len(data) == 0 || string(data) == "null" || string(data) == "[]" {
		p.Buckets = []int{}
		return nil
	}
	return json.Unmarshal(data, p)
}

func IntSliceContains(arr []int, target int) bool {
	for _, v := range arr {
		if v == target {
			return true
		}
	}
	return false
}
