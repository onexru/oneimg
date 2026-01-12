package controllers

import (
	"errors"
	"fmt"
	"log"
	"net/http"
	"reflect"
	"regexp"
	"slices"
	"strconv"
	"strings"

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

// 十六进制颜色格式正则
var hexColorRegex = regexp.MustCompile(`^#?([0-9a-fA-F]{3}|[0-9a-fA-F]{6})$`)

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

// 返回网站SEO信息
func GetSEOSettings(c *gin.Context) {
	settings, err := settings.GetSettings()
	if err != nil {
		c.JSON(500, result.Error(500, "获取设置失败"))
		return
	}

	c.JSON(200, result.Success("ok",
		map[string]any{
			"seo_title":       settings.SEOTitle,
			"seo_description": settings.SEODescription,
			"seo_keywords":    settings.SEOKeywords,
			"seo_icp":         settings.SEOICP,
			"public_security": settings.PublicSecurity,
			"seo_icon":        settings.SEOicon,
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

	// 校验设置数据
	if err := validateSettingData(req.Key, req.Value); err != nil {
		c.JSON(http.StatusBadRequest, result.Error(400, err.Error()))
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
		if slices.Contains(keys, jsonTag) {
			dstField := dstVal.FieldByName(srcField.Name)
			if dstField.IsValid() && dstField.CanSet() {
				dstField.Set(srcFieldVal)
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
	convertedValue, err := convertValueToTargetType(key, value, fieldType)
	if err != nil {
		return err
	}

	valueVal := reflect.ValueOf(convertedValue)

	// 5. 设置字段值
	targetField.Set(valueVal)
	return nil
}

func convertValueToTargetType(key string, value any, targetType reflect.Type) (any, error) {
	valueVal := reflect.ValueOf(value)
	valueType := valueVal.Type()

	// 类型已匹配，直接返回
	if valueType == targetType {
		return value, nil
	}

	// 场景1：反射支持直接转换（如 int→float64、bool→int 等）
	if valueType.ConvertibleTo(targetType) {
		return valueVal.Convert(targetType).Interface(), nil
	}

	// 场景2：反射不支持直接转换，手动处理常见类型解析
	switch targetType.Kind() {
	// 处理 string → float64（解决watermark_opac的核心问题）
	case reflect.Float64:
		if valueType.Kind() == reflect.String {
			strVal := valueVal.String()
			floatVal, err := strconv.ParseFloat(strVal, 64)
			if err != nil {
				return nil, fmt.Errorf("设置项 %s 类型转换失败，期望 float64，实际 string（值：%s），错误：%v",
					key, strVal, err)
			}
			return floatVal, nil
		}

	// 处理 string → int/int64
	case reflect.Int:
		if valueType.Kind() == reflect.String {
			strVal := valueVal.String()
			intVal, err := strconv.Atoi(strVal)
			if err != nil {
				return nil, fmt.Errorf("设置项 %s 类型转换失败，期望 int，实际 string（值：%s），错误：%v",
					key, strVal, err)
			}
			return intVal, nil
		}
	case reflect.Int64:
		if valueType.Kind() == reflect.String {
			strVal := valueVal.String()
			int64Val, err := strconv.ParseInt(strVal, 10, 64)
			if err != nil {
				return nil, fmt.Errorf("设置项 %s 类型转换失败，期望 int64，实际 string（值：%s），错误：%v",
					key, strVal, err)
			}
			return int64Val, nil
		}

	// 处理 string → bool
	case reflect.Bool:
		if valueType.Kind() == reflect.String {
			strVal := valueVal.String()
			boolVal, err := strconv.ParseBool(strVal)
			if err != nil {
				return nil, fmt.Errorf("设置项 %s 类型转换失败，期望 bool，实际 string（值：%s），错误：%v",
					key, strVal, err)
			}
			return boolVal, nil
		}
	case reflect.String:
		// 所有基础类型都可以转为string
		return fmt.Sprintf("%v", value), nil
	}

	// 不支持的转换类型
	return nil, fmt.Errorf("设置项 %s 类型不匹配，期望 %s，实际 %T",
		key, targetType, value)
}

func validateSettingData(key string, value any) error {
	switch key {
	case "watermark_text":
		// 1. 水印文字长度校验（兼容字符串类型）
		text, ok := value.(string)
		if !ok {
			return fmt.Errorf("水印文字必须是字符串类型，实际类型：%T", value)
		}
		if len(text) > 20 {
			return fmt.Errorf("水印文字长度不能超过20个字符（当前：%d）", len(text))
		}

	case "watermark_size":
		// 2. 水印字体大小校验
		var size int
		switch v := value.(type) {
		case int:
			size = v
		case string:
			// 字符串转int
			s := strings.TrimSpace(v)
			if s == "" {
				return errors.New("水印字体大小不能为空")
			}
			num, err := strconv.Atoi(s)
			if err != nil {
				return fmt.Errorf("水印字体大小必须是整数（当前值：%s）", v)
			}
			size = num
		default:
			return fmt.Errorf("水印字体大小必须是整数或数字字符串，实际类型：%T", value)
		}
		// 范围校验
		if size < 1 || size > 100 {
			return fmt.Errorf("水印字体大小必须在1-100之间（当前：%d）", size)
		}

	case "watermark_color":
		// 3. 水印颜色校验（防空 + 十六进制格式）
		color, ok := value.(string)
		if !ok {
			return fmt.Errorf("水印颜色必须是字符串类型，实际类型：%T", value)
		}
		color = strings.TrimSpace(color)
		if color == "" {
			return errors.New("水印字体颜色不能为空")
		}
		if !hexColorRegex.MatchString(color) {
			return fmt.Errorf("水印颜色格式错误，请使用十六进制颜色码，当前值：%s", color)
		}

	case "watermark_opac":
		// 4. 水印透明度校验（兼容字符串/float64/int，转为float64后校验0-1）
		var opac float64
		switch v := value.(type) {
		case float64:
			opac = v
		case int:
			opac = float64(v) // int转float64（如 1 → 1.0）
		case string:
			// 字符串转float64
			s := strings.TrimSpace(v)
			if s == "" {
				return errors.New("水印透明度不能为空")
			}
			num, err := strconv.ParseFloat(s, 64)
			if err != nil {
				return fmt.Errorf("水印透明度必须是数字（当前值：%s）", v)
			}
			opac = num
		default:
			return fmt.Errorf("水印透明度必须是数字或数字字符串，实际类型：%T", value)
		}
		// 范围校验（0.0-1.0）
		if opac < 0.0 || opac > 1.0 {
			return fmt.Errorf("水印透明度必须在0-1之间（当前：%.2f）", opac)
		}

	case "watermark_pos":
		// 5. 水印位置校验（防空 + 合法值）
		pos, ok := value.(string)
		if !ok {
			return fmt.Errorf("水印位置必须是字符串类型，实际类型：%T", value)
		}
		pos = strings.TrimSpace(pos)
		if pos == "" {
			return errors.New("水印位置不能为空")
		}
		// 合法位置集合
		validPos := map[string]bool{
			"top-left":     true,
			"top-right":    true,
			"bottom-left":  true,
			"bottom-right": true,
			"center":       true,
		}
		if !validPos[pos] {
			return fmt.Errorf("水印位置参数不合法")
		}
	case "default_storage":
		// 检查存储配置是否存在
		db := database.GetDB().DB

		var id int
		switch v := value.(type) {
		case int:
			id = v
		case string:
			num64, err := strconv.ParseInt(v, 10, 64)
			if err != nil {
				return fmt.Errorf("%s", "解析失败: "+err.Error())
			}
			id = int(num64)
		}

		Buckets := models.Buckets{
			Id: id,
		}

		if err := db.First(&Buckets).Error; err != nil {
			return fmt.Errorf("存储桶不存在")
		}
	}

	return nil
}
