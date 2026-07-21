package models

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
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

var AllPermissionMap = map[string]string{
	// 用户管理
	"user:create":            "添加用户",
	"user:delete":            "删除用户",
	"user:role:update":       "修改角色",
	"user:permission:update": "编辑权限",
	"user:password:reset":    "重置密码",

	// Tag管理
	"tag:create": "新增Tag",
	"tag:delete": "删除Tag",
	"tag:update": "编辑Tag",

	// 系统设置
	"setting:upload":       "上传与存储",
	"setting:image":        "图片处理",
	"setting:security":     "安全与登录",
	"setting:notification": "通知",
	"setting:api":          "API",
	"setting:seo":          "站点SEO",

	// 存储管理
	"storage:create": "新增存储",
	"storage:update": "编辑存储",
	"storage:delete": "删除存储",

	// 图片管理
	"image:delete":        "删除图片",
	"image:tag:add":       "添加图片标签",
	"image:tag:delete":    "删除图片标签",
	"image:access:source": "图片存储源",
}

// ValidatePermissionCodes 严格校验模式：如果传入了一个不存在的权限码，直接报错
// 适用场景：管理员手动输入或通过 API 精确修改权限时
func ValidatePermissionCodes(codes []string) error {
	for _, code := range codes {
		if _, exists := AllPermissionMap[code]; !exists {
			return fmt.Errorf("非法的权限码: %s", code)
		}
	}
	return nil
}

// FilterValidPermissionCodes 宽容过滤模式：过滤掉不存在的权限码，只保留合法的
// 适用场景：前端提交了一堆复选框数据，为了防止前端被篡改，后端默默过滤掉非法项
func FilterValidPermissionCodes(codes []string) []string {
	validCodes := make([]string, 0, len(codes))
	for _, code := range codes {
		if _, exists := AllPermissionMap[code]; exists {
			validCodes = append(validCodes, code)
		}
	}
	return validCodes
}

// GetPermissionName 根据 code 获取中文名 (额外提供的工具方法)
func GetPermissionName(code string) string {
	if name, ok := AllPermissionMap[code]; ok {
		return name
	}
	return "未知权限"
}

// 权限模型
type Permission struct {
	// OIDC code 列表
	Codes []string `json:"codes" gorm:"default:[]"`
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

func (p *Permission) HasBucket(bucketID int) bool {
	for _, b := range p.Buckets {
		if b == bucketID {
			return true
		}
	}
	return false
}

func IntSliceContains(arr []int, target int) bool {
	for _, v := range arr {
		if v == target {
			return true
		}
	}
	return false
}

// HasPermission 判断是否拥有某个具体的功能权限
func (p *Permission) HasPermission(code string) bool {
	for _, c := range p.Codes {
		if c == code {
			return true
		}
	}
	return false
}
