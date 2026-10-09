package controllers

import (
	"fmt"
	"gorm.io/gorm"
	"log"
	"oneimg/backend/models"
	"oneimg/backend/utils/powverify"
	"oneimg/backend/utils/publicurl"
	"oneimg/backend/utils/secureconfig"
	"reflect"
	"strconv"
	"strings"
)

func findSettingsField(typ reflect.Type, key string) (string, reflect.Type, error) {
	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)
		jsonTag := field.Tag.Get("json")
		if jsonTag == key || field.Name == key {
			return field.Name, field.Type, nil
		}
	}
	return "", nil, fmt.Errorf("设置项 %s 不存在", key)
}

func updateSensitiveSettingsField(settings *models.Settings, key string, value any) error {
	stringValue := strings.TrimSpace(fmt.Sprintf("%v", value))
	switch key {
	case "api_token":
		settings.APIToken = ""
		settings.APITokenHash = stringValue
		return nil
	case "tg_bot_token":
		settings.TGBotToken = stringValue
		return nil
	case "oidc_client_secret":
		settings.OIDCClientSecret = stringValue
		return nil
	case "turnstile_secret_key":
		settings.TurnstileSecret = stringValue
		return nil
	case "cloudflare_api_token":
		settings.CloudflareAPIToken = stringValue
		return nil
	default:
		return fmt.Errorf("设置项 %s 不支持敏感更新", key)
	}
}

func buildSettingsUpdate(key string, value any, fieldName string, fieldType reflect.Type) (string, any, error) {
	normalizedValue, err := secureconfig.NormalizeSettingValue(key, value)
	if err != nil {
		return "", nil, err
	}

	switch key {
	case "max_upload_files", "tag_max_length", "random_image_limit", "pow_verify_timeout_seconds":
		n, err := strictSettingInt(value)
		return key, n, err
	case "pow_verify_url", "pow_script_url", "pow_widget_url":
		text, ok := value.(string)
		if !ok {
			return "", nil, fmt.Errorf("POW 验证地址必须为字符串")
		}
		normalized, err := powverify.NormalizeURL(text)
		return key, normalized, err
	case "api_token":
		return "api_token_hash", normalizedValue, nil
	case "tg_bot_token", "oidc_client_secret", "turnstile_secret_key", "cloudflare_api_token":
		return key, normalizedValue, nil
	case "public_image_domain":
		domain, err := publicurl.NormalizeDomain(fmt.Sprintf("%v", value))
		if err != nil {
			return "", nil, err
		}
		return "public_image_domain", domain, nil
	case "oidc_issuer":
		normalized, err := normalizeOIDCIssuer(fmt.Sprintf("%v", value))
		return "oidc_issuer", normalized, err
	case "oidc_redirect_url":
		normalized, err := normalizeOIDCCallbackURL(fmt.Sprintf("%v", value))
		return "oidc_redirect_url", normalized, err
	case "cas_server_url":
		normalized, err := normalizeCASServerURL(fmt.Sprintf("%v", value))
		return "cas_server_url", normalized, err
	case "cas_service_url":
		normalized, err := normalizeCASCallbackURL(fmt.Sprintf("%v", value))
		return "cas_service_url", normalized, err
	case "oidc_scopes":
		normalized, err := normalizeOIDCScopes(fmt.Sprintf("%v", value))
		return "oidc_scopes", normalized, err
	case "oidc_client_id", "oidc_username_claim", "oidc_display_name", "oidc_super_admin_username", "cas_display_name", "cas_super_admin_username":
		return getSettingsColumnName(fieldName), strings.TrimSpace(fmt.Sprintf("%v", value)), nil
	default:
		convertedValue, convertErr := convertValueToTargetType(key, value, fieldType)
		if convertErr != nil {
			return "", nil, convertErr
		}
		return getSettingsColumnName(fieldName), convertedValue, nil
	}
}

func getSettingsColumnName(fieldName string) string {
	settingsType := reflect.TypeOf(models.Settings{})
	if field, ok := settingsType.FieldByName(fieldName); ok {
		gormTag := field.Tag.Get("gorm")
		for _, part := range strings.Split(gormTag, ";") {
			if strings.HasPrefix(part, "column:") {
				return strings.TrimPrefix(part, "column:")
			}
		}
	}
	return fieldName
}

func ensureSettingsColumn(db *gorm.DB, columnName string) error {
	fieldName, err := getSettingsFieldNameByColumn(columnName)
	if err != nil {
		return err
	}
	if db.Migrator().HasColumn(&models.Settings{}, columnName) {
		return nil
	}

	log.Printf("[数据库兼容] settings.%s 字段不存在，尝试创建", columnName)
	if err := db.Migrator().AddColumn(&models.Settings{}, fieldName); err != nil {
		return fmt.Errorf("创建 settings.%s 字段失败: %w", columnName, err)
	}
	return nil
}

func getSettingsFieldNameByColumn(columnName string) (string, error) {
	settingsType := reflect.TypeOf(models.Settings{})
	for i := 0; i < settingsType.NumField(); i++ {
		field := settingsType.Field(i)
		if getSettingsColumnName(field.Name) == columnName || field.Name == columnName {
			return field.Name, nil
		}
	}
	return "", fmt.Errorf("设置字段 %s 不存在", columnName)
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
	if value == nil {
		return nil, fmt.Errorf("设置项 %s 的值不能为空", key)
	}
	if targetType.Kind() == reflect.Int {
		return strictSettingInt(value)
	}
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
