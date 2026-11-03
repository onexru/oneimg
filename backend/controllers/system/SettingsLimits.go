package system

import (
	"fmt"
	"oneimg/backend/config"
	shared "oneimg/backend/controllers/shared"
	"oneimg/backend/models"
	"oneimg/backend/utils/uploadpolicy"
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
		"max_upload_files":   {Type: "integer", Default: config.DefaultMaxUploadFiles, Min: 1, Max: config.HardMaxUploadFiles, Permission: "setting:upload", Description: "每次请求最多上传文件数"},
		"tag_max_length":     {Type: "integer", Default: config.DefaultTagMaxLength, Min: 1, Max: config.HardTagMaxLength, Permission: "setting:upload", Description: "标签最多 Unicode 字符数"},
		"random_image_limit": {Type: "integer", Default: config.DefaultRandomImageLimit, Min: 1, Max: config.HardRandomImageLimit, Permission: "setting:api", Description: "随机图片接口每次最多返回数量"},
		// 账号默认上传配额，单位字节。0 = 不限（由 validateSettingData 单独放行，
		// 因为这里的 Min 是 1）。上界 1 PiB 与 UpdateUserQuota 保持一致。
		"default_user_quota": {Type: "integer", Default: int64(10) << 30, Min: 0, Max: int(1) << 50, Permission: "setting:upload", Description: "单账号默认上传配额（字节，0 表示不限）"},
	}
}
func publicLimitSettings(s models.Settings) map[string]any {
	return map[string]any{"max_upload_files": uploadpolicy.MaxFiles(s), "tag_max_length": uploadpolicy.TagLength(s), "random_image_limit": uploadpolicy.RandomLimit(s)}
}
func addConfigurableSettings(response map[string]any, s models.Settings) {
	for key, value := range publicLimitSettings(s) {
		response[key] = value
	}
}
func validateConfigurableSetting(key string, value any) (bool, error) {
	meta, ok := configurableFieldMetadata()[key]
	if !ok {
		return false, nil
	}
	switch meta.Type {
	case "integer":
		// default_user_quota 以字节计，10 GiB 就超过 int32；用 int64 解析，
		// 避免在 32 位目标上被静默截断成一个看起来合理却完全错误的额度。
		if key == "default_user_quota" {
			n, err := shared.StrictInt64(value)
			if err != nil || n < int64(meta.Min) || n > int64(meta.Max) {
				return true, fmt.Errorf("%s 必须是 %d-%d 之间的整数（单位：字节）", key, meta.Min, meta.Max)
			}
			return true, nil
		}
		n, err := shared.StrictSettingInt(value)
		if err != nil || n < meta.Min || n > meta.Max {
			return true, fmt.Errorf("%s 必须是 %d-%d 之间的整数", key, meta.Min, meta.Max)
		}
	case "string":
		// 目前可配置的字符串项都只是普通文本；原先这里还会对 POW 验证地址
		// 做 URL 规范化校验，该设置随在线 POW 一起下线。
		if _, ok := value.(string); !ok {
			return true, fmt.Errorf("%s 必须为字符串", key)
		}
	case "boolean":
		if _, ok := value.(bool); !ok {
			return true, fmt.Errorf("%s 必须为布尔值", key)
		}
	}
	return true, nil
}
