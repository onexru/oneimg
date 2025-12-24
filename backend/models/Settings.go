package models

import (
	"strings"
)

// Settings 系统配置模型（全局唯一配置）
// 注意：该表应只有一条记录（ID=1），所有配置项存储在同一条记录中
type Settings struct {
	ID            int    `gorm:"type:integer;primarykey;column:id;autoIncrement" json:"id"`
	OriginalImage bool   `gorm:"column:original_image;default:false" json:"original_image"` // 是否保存原图（默认保存）
	SaveWebp      bool   `gorm:"column:save_webp;default:true" json:"save_webp"`            // 是否保存webp格式（默认保存）
	Thumbnail     bool   `gorm:"column:thumbnail;default:true" json:"thumbnail"`            // 是否生成缩略图（默认生成）
	Tourist       bool   `gorm:"column:tourist;default:false" json:"tourist"`               // 是否允许游客上传（默认允许）
	TGNotice      bool   `gorm:"column:tg_notice;default:false" json:"tg_notice"`           // 是否启用TG通知（默认关闭）
	PowVerify     bool   `gorm:"column:pow_verify;default:false" json:"pow_verify"`         // 是否启用POW验证（默认关闭）
	TGBotToken    string `gorm:"column:tg_bot_token;default:''" json:"tg_bot_token"`        // TG机器人Token
	TGReceivers   string `gorm:"column:tg_receivers;default:''" json:"tg_receivers"`        // TG接收者（多个用逗号分隔）
	TGNoticeText  string `gorm:"column:tg_notice_text;default:''" json:"tg_notice_text"`    // TG通知文本
	StartAPI      bool   `gorm:"column:start_api;default:false" json:"start_api"`           // 是否启用API（默认关闭）
	APIToken      string `gorm:"column:api_token;default:''" json:"api_token"`              // API Token

	// 水印设置
	WatermarkEnable bool    `gorm:"column:watermark_enable;default:false" json:"watermark_enable"`    // 是否启用水印（默认不启用）
	WatermarkText   string  `gorm:"column:watermark_text;default:'初春图床'" json:"watermark_text"`       // 水印文字（默认为初春图床）
	WatermarkPos    string  `gorm:"column:watermark_pos;default:'bottom-right'" json:"watermark_pos"` // 水印位置（默认为右下角）
	WatermarkSize   int     `gorm:"column:watermark_size;default:10" json:"watermark_size"`           // 水印字体大小（默认为10）
	WatermarkColor  string  `gorm:"column:watermark_color;default:'#000000'" json:"watermark_color"`  // 水印字体颜色（默认为黑色）
	WatermarkOpac   float64 `gorm:"column:watermark_opac;default:0.5" json:"watermark_opac"`          // 水印透明度（默认为0.5）

	// 来源白名单设置
	RefererWhiteEnable bool   `gorm:"column:referer_white_enable;default:false" json:"referer_white_enable"` // 是否启用白名单
	RefererWhiteList   string `gorm:"column:referer_white_list;default:''" json:"referer_white_list"`        // 白名单（多个用逗号分隔）

	// 存储相关配置
	StorageType string `gorm:"column:storage_type;default:'default'" json:"storage_type"`   // 存储类型：default/s3/r2/webdav
	StoragePath string `gorm:"column:storage_path;default:'./uploads'" json:"storage_path"` // 本地存储路径（默认./uploads）

	// SEO 设置
	SEOTitle       string `gorm:"column:seo_title;default:'初春图床'" json:"seo_title"`                             // SEO标题（默认为初春图床）
	SEODescription string `gorm:"column:seo_description;default:'初春图床，一个免费、稳定、高效的图床服务'" json:"seo_description"` // SEO描述（默认为初春图床，一个免费、稳定、高效的图床服务）
	SEOKeywords    string `gorm:"column:seo_keywords;default:'初春网络,雾创岛,初春图床,图床,免费,稳定,高效'" json:"seo_keywords"`  // SEO关键词（默认为初春网络,雾创岛,初春图床,图床,免费,稳定,高效）
	SEOICP         string `gorm:"column:seo_icp;default:''" json:"seo_icp"`                                     // SEO ICP备案（默认为空）
	PublicSecurity string `gorm:"column:public_security;default:''" json:"public_security"`                     // SEO 公安备案（默认为空）
	SEOicon        string `gorm:"column:seo_icon;default:''" json:"seo_icon"`                                   // SEO ICON（默认为空）

	// S3配置（兼容S3协议的对象存储）
	S3Endpoint  string `gorm:"column:s3_endpoint;default:''" json:"s3_endpoint"`
	S3AccessKey string `gorm:"column:s3_access_key;default:''" json:"s3_access_key"`
	S3SecretKey string `gorm:"column:s3_secret_key;default:''" json:"s3_secret_key"`
	S3Bucket    string `gorm:"column:s3_bucket;default:''" json:"s3_bucket"`

	// WebDAV配置
	WebdavURL  string `gorm:"column:webdav_url;default:''" json:"webdav_url"`
	WebdavUser string `gorm:"column:webdav_user;default:''" json:"webdav_user"`
	WebdavPass string `gorm:"column:webdav_pass;default:''" json:"webdav_pass"`

	// FTP配置
	FTPHost string `gorm:"column:ftp_host;default:''" json:"ftp_host"`
	FTPUser string `gorm:"column:ftp_user;default:''" json:"ftp_user"`
	FTPPass string `gorm:"column:ftp_pass;default:''" json:"ftp_pass"`
	FTPPort int    `gorm:"column:ftp_port;default:21" json:"ftp_port"`
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
