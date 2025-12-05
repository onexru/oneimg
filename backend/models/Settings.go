package models

import (
	"strings"
)

// Settings 系统配置模型（全局唯一配置）
// 注意：该表应只有一条记录（ID=1），所有配置项存储在同一条记录中
type Settings struct {
	ID            int    `gorm:"primarykey;column:id" json:"id"`
	OriginalImage bool   `gorm:"column:original_image;default:false" json:"original_image"` // 是否保存原图（默认保存）
	SaveWebp      bool   `gorm:"column:save_webp;default:true" json:"save_webp"`            // 是否保存webp格式（默认保存）
	Thumbnail     bool   `gorm:"column:thumbnail;default:true" json:"thumbnail"`            // 是否生成缩略图（默认生成）
	Tourist       bool   `gorm:"column:tourist;default:false" json:"tourist"`               // 是否允许游客上传（默认允许）
	TGNotice      bool   `gorm:"column:tg_notice;default:false" json:"tg_notice"`           // 是否启用TG通知（默认关闭）
	PowVerify     bool   `gorm:"column:pow_verify;default:false" json:"pow_verify"`         // 是否启用POW验证（默认关闭）
	TGBotToken    string `gorm:"column:tg_bot_token;default:''" json:"tg_bot_token"`        // TG机器人Token
	TGReceivers   string `gorm:"column:tg_receivers;default:''" json:"tg_receivers"`        // TG接收者（多个用逗号分隔）
	TGNoticeText  string `gorm:"column:tg_notice_text;default:''" json:"tg_notice_text"`    // TG通知文本

	// 存储相关配置
	StorageType string `gorm:"column:storage_type;default:'default'" json:"storage_type"`   // 存储类型：default/s3/r2/webdav
	StoragePath string `gorm:"column:storage_path;default:'./uploads'" json:"storage_path"` // 本地存储路径（默认./uploads）

	// S3配置（兼容S3协议的对象存储）
	S3Endpoint  string `gorm:"column:s3_endpoint;default:''" json:"s3_endpoint"`
	S3AccessKey string `gorm:"column:s3_access_key;default:''" json:"s3_access_key"`
	S3SecretKey string `gorm:"column:s3_secret_key;default:''" json:"s3_secret_key"`
	S3Bucket    string `gorm:"column:s3_bucket;default:''" json:"s3_bucket"`

	// WebDAV配置
	WebdavURL  string `gorm:"column:webdav_url;default:''" json:"webdav_url"`
	WebdavUser string `gorm:"column:webdav_user;default:''" json:"webdav_user"`
	WebdavPass string `gorm:"column:webdav_pass;default:''" json:"webdav_pass"`
}

// TableName 指定表名（避免GORM自动复数）
func (Settings) TableName() string {
	return "settings"
}

// GetTGReceiversList 解析TG接收者为数组（多个用逗号分隔）
func (s *Settings) GetTGReceiversList() []string {
	if strings.TrimSpace(s.TGReceivers) == "" {
		return []string{}
	}
	receivers := strings.Split(s.TGReceivers, ",")
	// 去除空值和空格
	result := make([]string, 0, len(receivers))
	for _, r := range receivers {
		trimmed := strings.TrimSpace(r)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

// GetEffectiveStorageType 获取标准化的存储类型（小写）
func (s *Settings) GetEffectiveStorageType() string {
	return strings.ToLower(s.StorageType)
}

// IsValidStorageConfig 检查当前存储类型的配置是否完整
func (s *Settings) IsValidStorageConfig() bool {
	switch s.GetEffectiveStorageType() {
	case "default":
		return true // 本地存储无需额外配置（路径有默认值）
	case "s3":
		return strings.TrimSpace(s.S3Endpoint) != "" &&
			strings.TrimSpace(s.S3AccessKey) != "" &&
			strings.TrimSpace(s.S3SecretKey) != "" &&
			strings.TrimSpace(s.S3Bucket) != ""
	case "webdav":
		return strings.TrimSpace(s.WebdavURL) != "" &&
			strings.TrimSpace(s.WebdavUser) != "" &&
			strings.TrimSpace(s.WebdavPass) != ""
	default:
		return false
	}
}
