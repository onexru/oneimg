package controllers

import (
	"encoding/json"
	"fmt"
	"math"
	"oneimg/backend/config"
	"oneimg/backend/models"
	"oneimg/backend/utils/powverify"
	"oneimg/backend/utils/uploadpolicy"
	"strconv"
	"strings"
)

type settingFieldMetadata struct {
	Type        string `json:"type"`
	Default     any    `json:"default"`
	Min         int    `json:"min,omitempty"`
	Max         int    `json:"max,omitempty"`
	Permission  string `json:"permission"`
	Description string `json:"description"`
}

func configurableFieldMetadata() map[string]settingFieldMetadata {
	return map[string]settingFieldMetadata{
		"max_upload_files":           {Type: "integer", Default: config.DefaultMaxUploadFiles, Min: 1, Max: config.HardMaxUploadFiles, Permission: "setting:upload", Description: "每次请求最多上传文件数"},
		"tag_max_length":             {Type: "integer", Default: config.DefaultTagMaxLength, Min: 1, Max: config.HardTagMaxLength, Permission: "setting:upload", Description: "标签最多 Unicode 字符数"},
		"random_image_limit":         {Type: "integer", Default: config.DefaultRandomImageLimit, Min: 1, Max: config.HardRandomImageLimit, Permission: "setting:api", Description: "随机图片接口每次最多返回数量"},
		"pow_script_url":             {Type: "string", Default: "https://cha.eta.im/static/js/pow.min.js", Permission: "setting:security", Description: "浏览器运行的可信公共 HTTPS PoW 脚本；空值沿用旧提供方"},
		"pow_widget_url":             {Type: "string", Default: "https://cha.eta.im/", Permission: "setting:security", Description: "同提供方的公共 HTTPS challenge 地址；空值沿用旧提供方"},
		"pow_verify_url":             {Type: "string", Default: config.LegacyPowVerifyURL, Permission: "setting:security", Description: "公共 HTTPS（443）验证地址；空值沿用旧提供方，不允许凭据、查询参数、片段或重定向"},
		"pow_verify_timeout_seconds": {Type: "integer", Default: config.DefaultPowVerifyTimeoutSeconds, Min: 1, Max: config.HardPowVerifyTimeoutSeconds, Permission: "setting:security", Description: "旧 POW 提供方请求超时（秒）"},
		"pow_local_fallback":         {Type: "boolean", Default: false, Permission: "setting:security", Description: "显式改用本地 cap-pow，不联系旧 POW 提供方；默认关闭以兼容旧站点"},
	}
}
func publicLimitSettings(s models.Settings) map[string]any {
	return map[string]any{"max_upload_files": uploadpolicy.MaxFiles(s), "tag_max_length": uploadpolicy.TagLength(s), "random_image_limit": uploadpolicy.RandomLimit(s)}
}
func addConfigurableSettings(response map[string]any, s models.Settings) {
	for key, value := range publicLimitSettings(s) {
		response[key] = value
	}
	response["pow_script_url"] = effectivePowScriptURL(s)
	response["pow_widget_url"] = effectivePowWidgetURL(s)
	response["pow_verify_url"] = uploadpolicy.PowURL(s)
	response["pow_verify_timeout_seconds"] = uploadpolicy.PowTimeoutSeconds(s)
	response["pow_local_fallback"] = s.PowLocalFallback
}
func validateConfigurableSetting(key string, value any) (bool, error) {
	meta, ok := configurableFieldMetadata()[key]
	if !ok {
		return false, nil
	}
	switch meta.Type {
	case "integer":
		n, err := strictSettingInt(value)
		if err != nil || n < meta.Min || n > meta.Max {
			return true, fmt.Errorf("%s 必须是 %d-%d 之间的整数", key, meta.Min, meta.Max)
		}
	case "string":
		text, ok := value.(string)
		if !ok {
			return true, fmt.Errorf("%s 必须为字符串", key)
		}
		_, err := powverify.NormalizeURL(text)
		return true, err
	case "boolean":
		if _, ok := value.(bool); !ok {
			return true, fmt.Errorf("%s 必须为布尔值", key)
		}
	}
	return true, nil
}

// Reject fractions/NaN/overflows rather than reflect's silent numeric truncation.
func strictSettingInt(value any) (int, error) {
	var text string
	switch v := value.(type) {
	case int:
		return v, nil
	case int64:
		text = strconv.FormatInt(v, 10)
	case json.Number:
		text = v.String()
	case float64:
		if math.IsNaN(v) || math.IsInf(v, 0) || math.Trunc(v) != v {
			return 0, fmt.Errorf("必须是有限整数")
		}
		text = strconv.FormatFloat(v, 'f', -1, 64)
	case string:
		text = strings.TrimSpace(v)
	default:
		return 0, fmt.Errorf("必须是整数")
	}
	n, err := strconv.ParseInt(text, 10, strconv.IntSize)
	return int(n), err
}
