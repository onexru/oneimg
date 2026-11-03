package models

import (
	"strings"
)

// Settings 系统配置模型（全局唯一配置）
// 注意：该表应只有一条记录（ID=1），所有配置项存储在同一条记录中
type Settings struct {
	ID                      int    `gorm:"type:integer;primarykey;column:id;autoIncrement:false;default:1" json:"id"`
	CompressImage           bool   `gorm:"column:compress_image;default:false" json:"compress_image"` // 是否压缩图片（默认不压缩）
	SaveWebp                bool   `gorm:"column:save_webp;default:true" json:"save_webp"`            // 是否保存webp格式（默认保存）
	Thumbnail               bool   `gorm:"column:thumbnail;default:true" json:"thumbnail"`            // 是否生成缩略图（默认生成）
	Tourist                 bool   `gorm:"column:tourist;default:false" json:"tourist"`               // 是否允许游客上传（默认允许）
	TGNotice                bool   `gorm:"column:tg_notice;default:false" json:"tg_notice"` // 是否启用TG通知（默认关闭）
	// 以下 pow_* 字段为「在线 POW 验证」下线后的历史遗留列。
	// 保留字段是为了不与既有数据库产生列差异（GORM 不会删列），
	// 但它们既不返回前端、也不参与鉴权：所有读取路径已改为走 VerifyMethod。
	PowVerify               bool   `gorm:"column:pow_verify;default:false" json:"-"`         // 已废弃：旧版 POW 开关
	PowVerifyURL            string `gorm:"column:pow_verify_url;default:''" json:"-"`        // 已废弃：在线 POW 验证地址
	PowScriptURL            string `gorm:"column:pow_script_url;default:''" json:"-"`        // 已废弃：在线 POW 脚本地址
	PowWidgetURL            string `gorm:"column:pow_widget_url;default:''" json:"-"`        // 已废弃：在线 POW challenge 地址
	PowVerifyTimeoutSeconds int    `gorm:"column:pow_verify_timeout_seconds;default:5" json:"-"` // 已废弃：在线 POW 超时
	PowLocalFallback        bool   `gorm:"column:pow_local_fallback;default:false" json:"-"`  // 已废弃：在线 POW 本地回退开关
	VerifyMethod            string `gorm:"column:verify_method;default:''" json:"verify_method"` // 人机验证方式：''|none|turnstile|cappow（空则视为 none）
	TurnstileSiteKey        string `gorm:"column:turnstile_site_key;default:''" json:"turnstile_site_key"`       // Cloudflare Turnstile 站点公钥
	TurnstileSecret         string `gorm:"column:turnstile_secret_key;default:''" json:"turnstile_secret_key"`   // Cloudflare Turnstile 密钥（敏感，加密存储）
	CappowDifficulty        int    `gorm:"column:cappow_difficulty;default:4" json:"cappow_difficulty"`          // cap-pow 本地难度（目标 hex 前缀长度，1-8）
	CloudflareAPIToken      string `gorm:"column:cloudflare_api_token;default:''" json:"cloudflare_api_token"`   // Cloudflare API Token（Turnstile:Read，用于校验公钥，敏感，加密存储）
	CloudflareAccountID     string `gorm:"column:cloudflare_account_id;default:''" json:"cloudflare_account_id"` // Cloudflare 账号 ID（用于校验公钥）
	TGBotToken              string `gorm:"column:tg_bot_token;default:''" json:"tg_bot_token"`                   // TG机器人Token
	TGReceivers             string `gorm:"column:tg_receivers;default:''" json:"tg_receivers"`                   // TG接收者（多个用逗号分隔）
	TGNoticeText            string `gorm:"column:tg_notice_text;default:''" json:"tg_notice_text"`               // TG通知文本
	StartAPI                bool   `gorm:"column:start_api;default:false" json:"start_api"`                      // 是否启用API（默认关闭）
	APIToken                string `gorm:"column:api_token;default:''" json:"api_token"`                         // 兼容旧字段
	RandomGraph             bool   `gorm:"column:random_graph;default:false" json:"random_graph"`                // 是否启用随机图（默认关闭）
	APITokenHash            string `gorm:"column:api_token_hash;default:''" json:"-"`                            // API Token哈希
	SaveOriginalName        bool   `gorm:"column:save_original_name;default:false" json:"save_original_name"`    // 是否保存原文件名（默认不保存）
	StartRegister           bool   `gorm:"column:start_register;default:false" json:"start_register"`            // 是否启用注册（默认关闭）

	// 默认存储；游客 0 沿用系统默认，显式配置不改变游客登录开关。
	GuestStorage int `gorm:"column:guest_storage;default:0" json:"guest_storage"`
	DefaultStorage   int  `gorm:"column:default_storage;default:1" json:"default_storage"`           // 单存储模式下的默认存储
	MultiStorageSync bool `gorm:"column:multi_storage_sync;default:false" json:"multi_storage_sync"` // 是否启用本机落盘后的多存储后台同步
	EncryptedStorage bool `gorm:"column:encrypted_storage;default:false" json:"encrypted_storage"`   // 是否加密新写入到各存储源的图片文件

	// 外部身份认证
	OIDCEnable             bool   `gorm:"column:oidc_enable;default:false" json:"oidc_enable"`
	OIDCIssuer             string `gorm:"column:oidc_issuer;default:''" json:"oidc_issuer"`
	OIDCClientID           string `gorm:"column:oidc_client_id;default:''" json:"oidc_client_id"`
	OIDCClientSecret       string `gorm:"column:oidc_client_secret;default:''" json:"oidc_client_secret"`
	OIDCRedirectURL        string `gorm:"column:oidc_redirect_url;default:''" json:"oidc_redirect_url"`
	OIDCScopes             string `gorm:"column:oidc_scopes;default:'openid profile email'" json:"oidc_scopes"`
	OIDCUsernameClaim      string `gorm:"column:oidc_username_claim;default:'preferred_username'" json:"oidc_username_claim"`
	OIDCDisplayName        string `gorm:"column:oidc_display_name;default:'OIDC 登录'" json:"oidc_display_name"`
	OIDCAutoProvision      bool   `gorm:"column:oidc_auto_provision;default:true" json:"oidc_auto_provision"`
	OIDCSuperAdminUsername string `gorm:"column:oidc_super_admin_username;default:''" json:"oidc_super_admin_username"`
	CASEnable              bool   `gorm:"column:cas_enable;default:false" json:"cas_enable"`
	CASServerURL           string `gorm:"column:cas_server_url;default:''" json:"cas_server_url"`
	CASServiceURL          string `gorm:"column:cas_service_url;default:''" json:"cas_service_url"`
	CASDisplayName         string `gorm:"column:cas_display_name;default:'CAS 登录'" json:"cas_display_name"`
	CASAutoProvision       bool   `gorm:"column:cas_auto_provision;default:true" json:"cas_auto_provision"`
	CASSuperAdminUsername  string `gorm:"column:cas_super_admin_username;default:''" json:"cas_super_admin_username"`

	// 默认上传配置
	MaxUploadFiles   int    `gorm:"column:max_upload_files;default:10" json:"max_upload_files"`
	TagMaxLength     int    `gorm:"column:tag_max_length;default:10" json:"tag_max_length"`
	RandomImageLimit int    `gorm:"column:random_image_limit;default:20" json:"random_image_limit"`
	MaxFileSize      int    `gorm:"column:max_file_size;default:10485760" json:"max_file_size"` // 文件最大上传大小
	// DefaultUserQuota 是单个账号的总上传配额（字节），0 表示不限。
	//
	// 只约束「按用户累计的已用空间」，与存储桶自身的 capacity 是两件事：
	// 桶容量管物理后端，这个管账号额度。单个用户可在 users.quota_bytes 覆盖，
	// 覆盖后完全以该值为准（不再与默认值取小）。
	DefaultUserQuota int64 `gorm:"column:default_user_quota;default:10737418240" json:"default_user_quota"`
	AllowedTypes     string `gorm:"column:allowed_types;default:'image/jpeg,image/png,image/gif,image/webp'" json:"allowed_types"`
	DefaultPath      string `gorm:"column:default_path;default:'/uploads/{year}/{moon}'" json:"default_path"` // 默认上传路径，魔法变量 {year} 年 {month} 月 {day} 日 {hour} 小时 {minute} 分钟 {random} 随机 {uuid} UUID {role} 角色（1 为管理员, 2 为游客）
	FileName         string `gorm:"column:file_name;default:'{random}'" json:"file_name"`                     // 上传文件名称，魔法变量 {random} 随机数 {year} 年 {month} 月 {day} 日 {hour} 小时 {minute} 分钟 {second} 秒

	// 图片直链设置
	PublicImageDomain string `gorm:"column:public_image_domain;default:''" json:"public_image_domain"` // 图片直链域名（用于非本地存储直接访问）

	// 水印设置
	WatermarkEnable bool    `gorm:"column:watermark_enable;default:false" json:"watermark_enable"`    // 是否启用水印（默认不启用）
	WatermarkText   string  `gorm:"column:watermark_text;default:'初春图床'" json:"watermark_text"`       // 水印文字（默认为初春图床）
	WatermarkPos    string  `gorm:"column:watermark_pos;default:'bottom-right'" json:"watermark_pos"` // 水印位置（默认为右下角）
	WatermarkSize   int     `gorm:"column:watermark_size;default:10" json:"watermark_size"`           // 水印字体大小（默认为10）
	WatermarkColor  string  `gorm:"column:watermark_color;default:'#000000'" json:"watermark_color"`  // 水印字体颜色（默认为黑色）
	WatermarkOpac   float64 `gorm:"column:watermark_opac;default:0.5" json:"watermark_opac"`          // 水印透明度（默认为0.5）

	// 来源白名单设置
	RefererWhiteEnable bool   `gorm:"column:referer_white_enable;default:false" json:"referer_white_enable"` // 是否启用白名单
	RefererAllowEmpty  bool   `gorm:"column:referer_allow_empty;default:true" json:"referer_allow_empty"`    // 保持直接公开链接兼容；关闭后要求 Referer
	RefererWhiteList   string `gorm:"column:referer_white_list;default:''" json:"referer_white_list"`        // 白名单（多个用逗号分隔）

	// SEO 设置
	SEOTitle       string `gorm:"column:seo_title;default:'初春图床'" json:"seo_title"`                             // SEO标题（默认为初春图床）
	SEODescription string `gorm:"column:seo_description;default:'初春图床，一个免费、稳定、高效的图床服务'" json:"seo_description"` // SEO描述（默认为初春图床，一个免费、稳定、高效的图床服务）
	SEOKeywords    string `gorm:"column:seo_keywords;default:'初春网络,雾创岛,初春图床,图床,免费,稳定,高效'" json:"seo_keywords"`  // SEO关键词（默认为初春网络,雾创岛,初春图床,图床,免费,稳定,高效）
	SEOICP         string `gorm:"column:seo_icp;default:''" json:"seo_icp"`                                     // SEO ICP备案（默认为空）
	PublicSecurity string `gorm:"column:public_security;default:''" json:"public_security"`                     // SEO 公安备案（默认为空）
	SEOicon        string `gorm:"column:seo_icon;default:''" json:"seo_icon"`                                   // SEO ICON（默认为空）
}

// 人机验证方式常量。
// 人机验证方式。在线 POW 已下线，不再保留该取值。
const (
	VerifyMethodNone      = "none"
	VerifyMethodTurnstile = "turnstile"
	VerifyMethodCappow    = "cappow"
)

// EffectiveVerifyMethod 返回实际生效的人机验证方式。
//
// 只认 VerifyMethod 字段；旧库里可能残留 'pow'（在线 POW 已移除）或空串，
// 这两种情况一律回落到 none —— 否则登录会卡在一个早已不存在的验证方式上。
// 遇到这种情况建议管理员在后台重新选择一次验证方式。
//
// 注意：这里**不含** Turnstile 配置完整性回退，因为那需要运行时状态
// （服务端是否观测到密钥错误）。需要最终生效方式时用
// controllers.EffectiveVerifyMethod，它在本函数之上叠加了回退。
func EffectiveVerifyMethod(s Settings) string {
	switch s.VerifyMethod {
	case VerifyMethodTurnstile:
		return VerifyMethodTurnstile
	case VerifyMethodCappow:
		return VerifyMethodCappow
	case VerifyMethodNone:
		return VerifyMethodNone
	default:
		return VerifyMethodNone
	}
}

// ValidTurnstileSiteKey 宽松校验 Cloudflare Turnstile 站点公钥格式
// （0x 前缀 + 足够长度）。只用于识别明显错误的占位/乱填公钥，
// 阈值放宽以避免误判合法公钥。
func ValidTurnstileSiteKey(k string) bool {
	k = strings.TrimSpace(k)
	return strings.HasPrefix(k, "0x") && len(k) >= 16
}

// TurnstileConfigComplete 判断 Turnstile 的静态配置是否齐全：
// 公钥格式合法且密钥非空。这是「配置不完整就回退本地验证」的判据之一，
// 不含运行时状态（密钥被 Cloudflare 判定无效由控制器另行标记）。
//
// 放在 models 而不是 controllers，是为了让 middlewares 也能复用 ——
// CSP 需要知道实际生效的验证方式才能决定是否放行 'wasm-unsafe-eval'。
func TurnstileConfigComplete(s Settings) bool {
	return strings.TrimSpace(s.TurnstileSecret) != "" && ValidTurnstileSiteKey(s.TurnstileSiteKey)
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
