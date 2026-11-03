package system

import (
	"oneimg/backend/models"
	"slices"
	"strings"
)

// 辅助函数，筛选设置项
func filterSettings(settingsMap map[string]any, keys []string) map[string]any {
	if len(keys) == 0 {
		return settingsMap
	}

	filteredSettings := make(map[string]any)
	for key, value := range settingsMap {
		if slices.Contains(keys, key) {
			filteredSettings[key] = value
		}
	}
	return filteredSettings
}

func filterDelegatedSettings(response map[string]any, permission models.Permission) {
	for key := range response {
		required := getSettingRequiredPerm(key)
		if required == "" || !permission.HasPermission(required) {
			delete(response, key)
		}
	}
}

// SettingKeyPermissionMap 设置项key对应的权限码映射
var SettingKeyPermissionMap = map[string]string{
	// --- 上传与存储 ---
	"default_storage":     "setting:upload",
	"guest_storage":       "setting:upload",
	"public_image_domain": "setting:upload",
	"default_path":        "setting:upload",
	"file_name":           "setting:upload",
	"max_file_size":       "setting:upload",
	"max_upload_files":    "setting:upload",
	"tag_max_length":      "setting:upload",
	"allowed_types":       "setting:upload",
	"multi_storage_sync":  "setting:upload",
	"encrypted_storage":   "setting:upload",
	"save_original_name":  "setting:upload",
	// 账号默认上传配额与 max_file_size 同属「系统级上传策略」，
	// 因此复用 setting:upload，而不是新造一个权限码。
	"default_user_quota": "setting:upload",

	// --- 图片处理 ---
	"watermark_enable": "setting:image",
	"watermark_text":   "setting:image",
	"watermark_size":   "setting:image",
	"watermark_color":  "setting:image",
	"watermark_opac":   "setting:image",
	"watermark_pos":    "setting:image",
	"compress_image":   "setting:image",
	"save_webp":        "setting:image",
	"thumbnail":        "setting:image",

	// --- 安全与登录 ---
	"verify_method":               "setting:security",
	"turnstile_site_key":          "setting:security",
	"turnstile_secret_key":        "setting:security",
	"cloudflare_api_token":        "setting:security",
	"cloudflare_account_id":       "setting:security",
	"cappow_difficulty":           "setting:security",
	"tourist":                     "setting:security",
	"start_register":              "setting:security",
	"referer_white_enable":        "setting:security",
	"referer_white_list":          "setting:security",
	"referer_allow_empty":         "setting:security",
	"oidc_redirect_url_effective": "setting:security",
	"cas_service_url_effective":   "setting:security",
	"api_credentials":             "setting:api",
	"oidc_enable":                 "setting:security",
	"oidc_issuer":                 "setting:security",
	"oidc_client_id":              "setting:security",
	"oidc_client_secret":          "setting:security",
	"oidc_auto_provision":         "setting:security",
	"oidc_super_admin_username":   "setting:security",
	"oidc_redirect_url":           "setting:security",
	"oidc_scopes":                 "setting:security",
	"oidc_username_claim":         "setting:security",
	"oidc_display_name":           "setting:security",
	"cas_enable":                  "setting:security",
	"cas_server_url":              "setting:security",
	"cas_auto_provision":          "setting:security",
	"cas_super_admin_username":    "setting:security",
	"cas_service_url":             "setting:security",
	"cas_display_name":            "setting:security",

	// --- 通知 ---
	"tg_notice":      "setting:notification",
	"tg_bot_token":   "setting:notification",
	"tg_receivers":   "setting:notification",
	"tg_notice_text": "setting:notification",

	// --- API ---
	"start_api":          "setting:api",
	"api_token":          "setting:api",
	"random_graph":       "setting:api",
	"random_image_limit": "setting:api",

	// --- 站点SEO ---
	"seo_title":       "setting:seo",
	"seo_description": "setting:seo",
	"seo_keywords":    "setting:seo",
	"seo_icp":         "setting:seo",
	"public_security": "setting:seo",
	"seo_icon":        "setting:seo",
}

// getSettingRequiredPerm
func getSettingRequiredPerm(key string) string {
	if perm, ok := SettingKeyPermissionMap[strings.TrimSuffix(key, "_configured")]; ok {
		return perm
	}
	return ""
}

// SuperAdminOnlySettingKeys 只能由超级管理员修改的设置项。
//
// 这些键共同构成「认证信任锚」：外部身份提供方的地址、用于验证其响应的密钥、
// 把某个外部用户名直接映射到本地超管的开关。它们一旦可被委派者改动，整条
// 权限体系就失去意义——持有 setting:security 的普通账号可以把自己的 IdP 配上去、
// 再把自己的外部用户名填进 *_super_admin_username，用自己的账号走一次 SSO 就拿到
// 本地超管会话（见 resolveExternalCallbackUser：外部用户名匹配即直接返回 ID=1，
// 不要求事先绑定）。
//
// 因此这一组**不能**只依赖 setting:security：那正是攻击链的入口。
// api_token 同理——它的 legacy 凭据固定归属超管（authsecurity.EnrollLegacyToken），
// 允许委派者写入等于凭空获得一枚超管身份的 API 凭据。
var SuperAdminOnlySettingKeys = map[string]struct{}{
	// OIDC 信任锚
	"oidc_enable":               {},
	"oidc_issuer":               {},
	"oidc_client_id":            {},
	"oidc_client_secret":        {},
	"oidc_redirect_url":         {},
	"oidc_scopes":               {},
	"oidc_username_claim":       {},
	"oidc_auto_provision":       {},
	"oidc_super_admin_username": {},
	"oidc_display_name":         {},
	// CAS 信任锚
	"cas_enable":               {},
	"cas_server_url":           {},
	"cas_service_url":          {},
	"cas_auto_provision":       {},
	"cas_super_admin_username": {},
	"cas_display_name":         {},
	// 全局 API 凭据：legacy 凭据固定归属超管，写入等于提权
	"api_token": {},
	// 回调地址回显：虽然不是写入入口，但它暴露公网可达的 IdP 端点与
	// 完整回调 URL，属于信任锚的一部分，一并收口。
	"oidc_redirect_url_effective": {},
	"cas_service_url_effective":   {},
}

// IsSuperAdminOnlySetting 判断某个设置项是否只能由超管修改。
// 兼容 xxx_configured 这类只读回显键。
func IsSuperAdminOnlySetting(key string) bool {
	_, ok := SuperAdminOnlySettingKeys[strings.TrimSuffix(key, "_configured")]
	return ok
}
