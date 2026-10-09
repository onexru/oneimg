package controllers

import (
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"log"
	"net/http"
	"oneimg/backend/database"
	"oneimg/backend/middlewares"
	"oneimg/backend/models"
	"oneimg/backend/utils/authsecurity"
	"oneimg/backend/utils/result"
	"oneimg/backend/utils/secureconfig"
	"oneimg/backend/utils/settings"
	"reflect"
)

func UpdateSettings(c *gin.Context) {
	if c.GetString("auth_method") == "api_token" {
		c.JSON(403, result.Error(403, "API 凭据不能修改系统设置"))
		return
	}
	var req UpdateSettingsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, result.Error(400, "请求参数错误"))
		return
	}
	if req.Key == "regenerate" {
		RegenerateAPIToken(c)
		return
	}
	if req.Key == "renew_session" {
		RenewSession(c)
		return
	}
	if getSettingRequiredPerm(req.Key) == "" {
		c.JSON(400, result.Error(400, "未知的设置项"))
		return
	}
	if req.Value == nil {
		c.JSON(http.StatusBadRequest, result.Error(400, "设置值不能为空"))
		return
	}
	user, exists := middlewares.GetCurrentUser(c)
	if !exists {
		c.JSON(401, result.Error(401, "未登录"))
		return
	}
	if user.ID != models.SuperAdminID {
		requiredPerm := getSettingRequiredPerm(req.Key)
		if requiredPerm == "" {
			c.JSON(400, result.Error(400, "未知的设置项"))
			return
		}
		requiredPermName := models.GetPermissionName(requiredPerm)
		if !user.Permission.HasPermission(requiredPerm) {
			c.JSON(403, result.Error(403, "无权修改该设置项，需要权限: "+requiredPermName))
			return
		}
	}

	// 查询是否有该设置项
	settingModel, err := settings.GetSettings()
	if err != nil {
		c.JSON(500, result.Error(500, "获取设置失败"))
		return
	}

	// 校验设置数据
	if err := validateSettingData(req.Key, req.Value); err != nil {
		c.JSON(http.StatusBadRequest, result.Error(400, err.Error()))
		return
	}

	fieldName, fieldType, err := findSettingsField(reflect.TypeOf(settingModel), req.Key)
	if err != nil {
		c.JSON(http.StatusBadRequest, result.Error(400, err.Error()))
		return
	}

	if secureconfig.IsSettingsSensitiveKey(req.Key) {
		if err := updateSensitiveSettingsField(&settingModel, req.Key, req.Value); err != nil {
			c.JSON(http.StatusBadRequest, result.Error(400, err.Error()))
			return
		}
	} else if err := updateSettingsField(&settingModel, req.Key, req.Value); err != nil {
		c.JSON(http.StatusBadRequest, result.Error(400, err.Error()))
		return
	}

	// 更新设置项
	db := database.GetDB().DB

	updateColumn, updateValue, err := buildSettingsUpdate(req.Key, req.Value, fieldName, fieldType)
	if err != nil {
		c.JSON(http.StatusBadRequest, result.Error(400, err.Error()))
		return
	}

	if updateValue == nil && secureconfig.IsSettingsSensitiveKey(req.Key) {
		c.JSON(200, result.Success("无需更新", nil))
		return
	}

	if err := ensureSettingsColumn(db, updateColumn); err != nil {
		c.JSON(http.StatusInternalServerError, result.Error(500, "数据库字段兼容处理失败"))
		log.Printf("设置 %s 更新失败（%T）", req.Key, err)
		return
	}

	if err := db.Transaction(func(tx *gorm.DB) error {
		if req.Key == "api_token" {
			var state models.AuthState
			if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&state, 1).Error; err != nil {
				return err
			}
			if err := tx.Model(&models.APICredential{}).Where("revoked = ?", false).Update("revoked", true).Error; err != nil {
				return err
			}
			if err := tx.Model(&settingModel).Updates(map[string]any{"api_token": "", "api_token_hash": updateValue}).Error; err != nil {
				return err
			}
			settingModel.APIToken = ""
			settingModel.APITokenHash = updateValue.(string)
			// An explicit settings edit is a rotation, not a lazy extension of expiry.
			if err := tx.Where("id = ?", authsecurity.LegacyTokenID(settingModel)).Delete(&models.APICredential{}).Error; err != nil {
				return err
			}
			return authsecurity.EnrollLegacyToken(tx, settingModel)
		}
		return tx.Model(&settingModel).Update(updateColumn, updateValue).Error
	}); err != nil {
		c.JSON(http.StatusInternalServerError, result.Error(500, "更新失败"))
		log.Printf("设置 %s 更新失败（%T）", req.Key, err)
		return
	}

	settings.Invalidate()
	c.JSON(200, result.Success("更新成功", nil))
}
