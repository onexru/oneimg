package controllers

import (
	"fmt"
	"log"
	"net/http"
	"reflect"

	"github.com/gin-gonic/gin"

	"oneimg/backend/database"
	"oneimg/backend/models"
	"oneimg/backend/utils/result"
	"oneimg/backend/utils/settings"
)

// 定义请求参数
type UpdateSettingsRequest struct {
	Key   string `json:"key" binding:"required"`
	Value any    `json:"value" binding:"required"`
}

// 自定义查询参数
type GetSettingsRequest struct {
	Keys []string `json:"keys"`
}

func GetSettings(c *gin.Context) {
	var req GetSettingsRequest
	settings, err := settings.GetSettings()
	if err != nil {
		c.JSON(500, result.Error(500, "获取设置失败"))
		return
	}
	filtered := filterSettings(&settings, req.Keys)

	c.JSON(200, result.Success("ok", filtered))
}

// 返回登录配置
func GetLoginSettings(c *gin.Context) {
	settings, err := settings.GetSettings()
	if err != nil {
		c.JSON(500, result.Error(500, "获取设置失败"))
		return
	}

	c.JSON(200, result.Success("ok",
		map[string]any{
			"pow_verify": settings.PowVerify,
			"tourist":    settings.Tourist,
		},
	))
}

func UpdateSettings(c *gin.Context) {
	var req UpdateSettingsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, result.Error(400, "请求参数错误: "+err.Error()))
		return
	}
	// 查询是否有该设置项
	settings, err := settings.GetSettings()
	if err != nil {
		c.JSON(500, result.Error(500, "获取设置失败"))
		return
	}

	if err := updateSettingsField(&settings, req.Key, req.Value); err != nil {
		c.JSON(http.StatusBadRequest, result.Error(400, err.Error()))
		return
	}

	// 更新设置项
	db := database.GetDB().DB

	if err := db.Model(&settings).Update(req.Key, req.Value).Error; err != nil {
		c.JSON(http.StatusInternalServerError, result.Error(500, "更新失败"))
		log.Println(err)
		return
	}

	c.JSON(200, result.Success("更新成功", nil))
}

// 辅助函数，筛选设置项
func filterSettings(settings *models.Settings, keys []string) *models.Settings {
	if len(keys) == 0 {
		return settings
	}

	filteredSettings := &models.Settings{}
	srcVal := reflect.ValueOf(settings).Elem()
	dstVal := reflect.ValueOf(filteredSettings).Elem()
	srcTyp := srcVal.Type()
	for i := 0; i < srcTyp.NumField(); i++ {
		srcField := srcTyp.Field(i)
		srcFieldVal := srcVal.Field(i)
		jsonTag := srcField.Tag.Get("json")
		if jsonTag == "" {
			continue
		}
		for _, key := range keys {
			if jsonTag == key {
				dstField := dstVal.FieldByName(srcField.Name)
				if dstField.IsValid() && dstField.CanSet() {
					dstField.Set(srcFieldVal)
				}
				break
			}
		}
	}
	return filteredSettings
}

func updateSettingsField(settings *models.Settings, key string, value any) error {
	// 获取结构体反射值（指针解引用）
	val := reflect.ValueOf(settings).Elem()
	typ := val.Type()

	// 1. 遍历结构体字段，匹配JSON Tag或字段名
	var targetField reflect.Value
	var fieldType reflect.Type
	found := false

	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)
		// 优先匹配JSON Tag（如 json:"tourist"）
		jsonTag := field.Tag.Get("json")
		if jsonTag == key || field.Name == key {
			targetField = val.Field(i)
			fieldType = field.Type
			found = true
			break
		}
	}

	// 校验字段是否存在
	if !found {
		return fmt.Errorf("设置项 %s 不存在", key)
	}

	// 2. 校验字段是否可修改（必须是导出字段）
	if !targetField.CanSet() {
		return fmt.Errorf("设置项 %s 不可修改", key)
	}

	// 3. 处理nil值（避免panic）
	if value == nil {
		return fmt.Errorf("设置项 %s 的值不能为空", key)
	}

	// 4. 转换value类型为字段实际类型
	valueVal := reflect.ValueOf(value)
	if valueVal.Type() != fieldType {
		// 尝试类型转换（支持常见类型：bool/string/int等）
		if !valueVal.Type().ConvertibleTo(fieldType) {
			return fmt.Errorf("设置项 %s 类型不匹配，期望 %s，实际 %T",
				key, fieldType, value)
		}
		valueVal = valueVal.Convert(fieldType)
	}

	// 5. 设置字段值
	targetField.Set(valueVal)
	return nil
}
