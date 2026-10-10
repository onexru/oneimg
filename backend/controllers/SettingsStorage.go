package controllers

import (
	"fmt"
	"oneimg/backend/database"
	"oneimg/backend/models"
	"strconv"
)

func settingDisabledByPublicDomain(key string) bool {
	switch key {
	case "watermark_enable",
		"watermark_text",
		"watermark_size",
		"watermark_color",
		"watermark_opac",
		"watermark_pos",
		"referer_white_enable",
		"referer_white_list":
		return true
	default:
		return false
	}
}

func getBucketTypeByID(id int) (string, error) {
	db := database.GetDB().DB
	var bucket models.Buckets
	if err := db.First(&bucket, id).Error; err != nil {
		return "", fmt.Errorf("存储桶不存在")
	}
	if bucket.Disabled {
		return "", fmt.Errorf("存储源已停用，请先启用")
	}
	return bucket.Type, nil
}

func settingValueToInt(value any) (int, error) {
	switch v := value.(type) {
	case int:
		return v, nil
	case int64:
		return int(v), nil
	case float64:
		return int(v), nil
	case string:
		num64, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			return 0, err
		}
		return int(num64), nil
	default:
		return 0, fmt.Errorf("类型错误：%T", value)
	}
}
