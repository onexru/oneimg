package controllers

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
	"pow_verify":                  "setting:security",
	"pow_script_url":              "setting:security",
	"pow_widget_url":              "setting:security",
	"pow_verify_url":              "setting:security",
	"pow_verify_timeout_seconds":  "setting:security",
	"pow_local_fallback":          "setting:security",
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
