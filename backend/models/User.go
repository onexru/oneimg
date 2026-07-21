package models

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

// User 用户模型。
type User struct {
	ID         int        `json:"id" gorm:"type:integer;primaryKey;autoIncrement"`
	Role       int        `json:"role" gorm:"default:1"`
	Username   string     `json:"username" gorm:"unique;not null"`
	Password   string     `json:"-" gorm:"not null"`
	Permission Permission `json:"permission" gorm:"type:jsonb"`
	CreatedAt  time.Time  `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt  time.Time  `json:"updated_at" gorm:"autoUpdateTime"`
}

// 角色与超级管理员约定。
const (
	SuperAdminID = 1
	RoleAdmin    = 1
	RoleGuest    = 2
	RoleUser     = 3
)

// AllPermissionMap 权限码到中文名称的映射。
var AllPermissionMap = map[string]string{
	"user:create":            "添加用户",
	"user:delete":            "删除用户",
	"user:role:update":       "修改角色",
	"user:permission:update": "编辑权限",
	"user:password:reset":    "重置密码",

	"tag:create": "新增Tag",
	"tag:delete": "删除Tag",
	"tag:update": "编辑Tag",

	"setting:upload":       "上传与存储",
	"setting:image":        "图片处理",
	"setting:security":     "安全与登录",
	"setting:notification": "通知",
	"setting:api":          "API",
	"setting:seo":          "站点SEO",

	"storage:create": "新增存储",
	"storage:update": "编辑存储",
	"storage:delete": "删除存储",

	"image:delete":        "删除图片",
	"image:tag:add":       "添加图片标签",
	"image:tag:delete":    "删除图片标签",
	"image:access:source": "图片存储源",
}

// ValidatePermissionCodes 严格校验：存在非法权限码则报错。
func ValidatePermissionCodes(codes []string) error {
	for _, code := range codes {
		if _, exists := AllPermissionMap[code]; !exists {
			return fmt.Errorf("非法的权限码: %s", code)
		}
	}
	return nil
}

// FilterValidPermissionCodes 过滤掉非法权限码，仅保留合法项。
func FilterValidPermissionCodes(codes []string) []string {
	validCodes := make([]string, 0, len(codes))
	for _, code := range codes {
		if _, exists := AllPermissionMap[code]; exists {
			validCodes = append(validCodes, code)
		}
	}
	return validCodes
}

// GetPermissionName 返回权限码中文名；未知码返回「未知权限」。
func GetPermissionName(code string) string {
	if name, ok := AllPermissionMap[code]; ok {
		return name
	}
	return "未知权限"
}

// Permission 用户权限：功能码列表 + 可用存储桶 ID。
type Permission struct {
	Codes   []string `json:"codes" gorm:"default:[]"`
	Buckets []int    `json:"buckets" gorm:"default:[]"`
}

// Value 序列化为 JSON 写入数据库。
func (p Permission) Value() (driver.Value, error) {
	return json.Marshal(p)
}

// Scan 从数据库 JSON 反序列化。
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

// HasBucket 判断是否拥有指定存储桶权限。
func (p *Permission) HasBucket(bucketID int) bool {
	for _, b := range p.Buckets {
		if b == bucketID {
			return true
		}
	}
	return false
}

// IntSliceContains 判断整型切片是否包含目标值。
func IntSliceContains(arr []int, target int) bool {
	for _, v := range arr {
		if v == target {
			return true
		}
	}
	return false
}

// HasPermission 判断是否拥有指定功能权限码。
func (p *Permission) HasPermission(code string) bool {
	for _, c := range p.Codes {
		if c == code {
			return true
		}
	}
	return false
}
