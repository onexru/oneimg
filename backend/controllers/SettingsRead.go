package controllers

import (
	"github.com/gin-gonic/gin"
	"net/http"
	"oneimg/backend/database"
	"oneimg/backend/middlewares"
	"oneimg/backend/models"
	"oneimg/backend/utils/result"
	"oneimg/backend/utils/secureconfig"
	"oneimg/backend/utils/settings"
	"strings"
	"time"
)

// GetSettings is fail-closed: unmapped fields never leak to delegated users.
func GetSettings(c *gin.Context) {
	user, exists := middlewares.GetCurrentUser(c)
	if !exists {
		c.JSON(401, result.Error(401, "未登录"))
		return
	}
	isAdmin := c.GetString("auth_method") != "api_token" && (user.ID == models.SuperAdminID || user.Role == models.RoleAdmin)
	if !isAdmin && !user.Permission.HasPermission("setting:list") {
		c.JSON(403, result.Error(403, "无权查看设置，需要权限 setting:list"))
		return
	}
	var req GetSettingsRequest
	if c.Request.Method == http.MethodPost && c.Request.ContentLength != 0 {
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(400, result.Error(400, "请求参数错误"))
			return
		}
	} else {
		req.Keys = c.QueryArray("keys")
	}
	settingModel, err := settings.GetSettings()
	if err != nil {
		c.JSON(500, result.Error(500, "获取设置失败"))
		return
	}
	response := secureconfig.SanitizeSettingsForResponse(settingModel)
	addConfigurableSettings(response, settingModel)
	if url, err := oidcCallbackURL(settingModel); err == nil {
		response["oidc_redirect_url_effective"] = url
	}
	if url, err := casCallbackURL(settingModel); err == nil {
		response["cas_service_url_effective"] = url
	}
	var tokens []models.APICredential
	if isAdmin || user.Permission.HasPermission("setting:api") {
		query := database.GetDB().DB.Where("revoked = ?", false)
		if !isAdmin {
			query = query.Where("owner_id = ?", user.ID)
		}
		if query.Order("created_at desc").Limit(10).Find(&tokens).Error == nil {
			response["api_credentials"] = tokens
			for _, token := range tokens {
				if time.Now().Before(token.ExpiresAt) {
					response["api_token_configured"] = true
				}
			}
		}
	}
	filtered := filterSettings(response, req.Keys)
	permissions := []string{}
	if isAdmin {
		permissions = []string{"setting:upload", "setting:image", "setting:security", "setting:notification", "setting:api", "setting:seo"}
	} else {
		filterDelegatedSettings(filtered, user.Permission)
		for _, code := range user.Permission.Codes {
			if strings.HasPrefix(code, "setting:") {
				permissions = append(permissions, code)
			}
		}
	}
	metadata := configurableFieldMetadata()
	if !isAdmin {
		for key, field := range metadata {
			if !user.Permission.HasPermission(field.Permission) {
				delete(metadata, key)
			}
		}
	}
	c.JSON(200, gin.H{"code": 200, "message": "ok", "data": filtered, "setting_permissions": permissions, "setting_field_metadata": metadata})
}

// GetLoginSettings 获取登录页公开配置。
func GetLoginSettings(c *gin.Context) {
	settingModel, err := settings.GetSettings()
	if err != nil {
		c.JSON(500, result.Error(500, "获取设置失败"))
		return
	}

	c.JSON(200, result.Success("ok",
		map[string]any{
			"verify_method":      effectiveVerifyMethodWithFallback(settingModel),
			"pow_script_url":     effectivePowScriptURL(settingModel),
			"pow_widget_url":     effectivePowWidgetURL(settingModel),
			"pow_verify":         settingModel.PowVerify,
			"pow_local_fallback": settingModel.PowLocalFallback,
			"max_upload_files":   publicLimitSettings(settingModel)["max_upload_files"],
			"tag_max_length":     publicLimitSettings(settingModel)["tag_max_length"],
			"random_image_limit": publicLimitSettings(settingModel)["random_image_limit"],
			"turnstile_site_key": settingModel.TurnstileSiteKey,
			"tourist":            settingModel.Tourist,
			"start_register":     settingModel.StartRegister,
			"oidc_enabled":       oidcSettingsReady(settingModel),
			"oidc_display_name":  externalLoginDisplayName(settingModel.OIDCDisplayName, "OIDC 登录"),
			"cas_enabled":        casSettingsReady(settingModel),
			"cas_display_name":   externalLoginDisplayName(settingModel.CASDisplayName, "CAS 登录"),
		},
	))
}

// GetSEOSettings 获取站点 SEO 公开配置。
func GetSEOSettings(c *gin.Context) {
	settingModel, err := settings.GetSettings()
	if err != nil {
		c.JSON(500, result.Error(500, "获取设置失败"))
		return
	}

	c.JSON(200, result.Success("ok",
		map[string]any{
			"seo_title":       settingModel.SEOTitle,
			"seo_description": settingModel.SEODescription,
			"seo_keywords":    settingModel.SEOKeywords,
			"seo_icp":         settingModel.SEOICP,
			"public_security": settingModel.PublicSecurity,
			"seo_icon":        settingModel.SEOicon,
		},
	))
}
