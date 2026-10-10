package controllers

import (
	"errors"
	"fmt"
	"oneimg/backend/models"
	"oneimg/backend/utils/publicurl"
	"oneimg/backend/utils/securestorage"
	"oneimg/backend/utils/settings"
	"reflect"
	"regexp"
	"strconv"
	"strings"
)

var hexColorRegex = regexp.MustCompile(`^#?([0-9a-fA-F]{3}|[0-9a-fA-F]{6})$`)

func validateSettingData(key string, value any) error {
	if handled, err := validateConfigurableSetting(key, value); handled {
		return err
	}
	if settingDisabledByPublicDomain(key) {
		setting, err := settings.GetSettings()
		if err != nil {
			return fmt.Errorf("获取系统配置失败")
		}
		if publicurl.HasDomain(setting) {
			return fmt.Errorf("已配置图片直链域名，该设置不会生效，请先清空图片直链域名")
		}
	}

	switch key {
	case "oidc_issuer":
		_, err := normalizeOIDCIssuer(fmt.Sprintf("%v", value))
		return err
	case "oidc_redirect_url":
		_, err := normalizeOIDCCallbackURL(fmt.Sprintf("%v", value))
		return err
	case "cas_service_url":
		_, err := normalizeCASCallbackURL(fmt.Sprintf("%v", value))
		return err
	case "cas_server_url":
		_, err := normalizeCASServerURL(fmt.Sprintf("%v", value))
		return err
	case "oidc_scopes":
		_, err := normalizeOIDCScopes(fmt.Sprintf("%v", value))
		return err
	case "oidc_username_claim":
		claim := strings.TrimSpace(fmt.Sprintf("%v", value))
		if claim == "" || !oidcClaimNameRegex.MatchString(claim) {
			return fmt.Errorf("OIDC 用户名 Claim 格式不正确")
		}
	case "oidc_client_id":
		if len(strings.TrimSpace(fmt.Sprintf("%v", value))) > 512 {
			return fmt.Errorf("OIDC Client ID 过长")
		}
	case "oidc_client_secret":
		if len(strings.TrimSpace(fmt.Sprintf("%v", value))) > 4096 {
			return fmt.Errorf("OIDC Client Secret 过长")
		}
	case "oidc_display_name", "cas_display_name":
		name := strings.TrimSpace(fmt.Sprintf("%v", value))
		if len([]rune(name)) > 40 {
			return fmt.Errorf("登录按钮名称不能超过40个字符")
		}
	case "oidc_super_admin_username", "cas_super_admin_username":
		if len([]rune(strings.TrimSpace(fmt.Sprintf("%v", value)))) > 255 {
			return fmt.Errorf("超级管理员映射用户名不能超过255个字符")
		}
	case "oidc_enable":
		enabled, err := convertValueToTargetType(key, value, reflect.TypeOf(false))
		if err != nil {
			return err
		}
		if enabled.(bool) {
			setting, err := settings.GetSettings()
			if err != nil || !oidcSettingsComplete(setting) {
				return fmt.Errorf("请先完整配置 OIDC Issuer、Client ID、Client Secret 和回调地址")
			}
		}
	case "cas_enable":
		enabled, err := convertValueToTargetType(key, value, reflect.TypeOf(false))
		if err != nil {
			return err
		}
		if enabled.(bool) {
			setting, err := settings.GetSettings()
			if err != nil || !casSettingsComplete(setting) {
				return fmt.Errorf("请先完整配置 CAS Server URL 和回调地址")
			}
		}
	case "verify_method":
		method := strings.TrimSpace(fmt.Sprintf("%v", value))
		switch method {
		case models.VerifyMethodNone, models.VerifyMethodPOW, models.VerifyMethodTurnstile, models.VerifyMethodCappow:
		default:
			return fmt.Errorf("验证方式不合法，可选：无验证、在线POW、Turnstile、cap-pow")
		}
		// 允许先切换方式、后配置密钥：若 Turnstile 密钥未配置，登录/注册时
		// 会提示「Turnstile 验证尚未配置」，避免配置死锁。
	case "turnstile_site_key":
		siteKey := strings.TrimSpace(fmt.Sprintf("%v", value))
		if siteKey == "" {
			return nil // 允许清空
		}
		// 保存前通过 Cloudflare 管理 API 校验公钥真实存在（需先配置 API Token 与账号 ID）
		setting, err := settings.GetSettingsWithSecrets("cloudflare_api_token")
		if err != nil {
			return fmt.Errorf("获取系统配置失败")
		}
		if err := validateTurnstileSiteKeyAPI(siteKey, setting.CloudflareAPIToken, setting.CloudflareAccountID); err != nil {
			return err
		}
	case "turnstile_secret_key":
		// 留空表示不修改（沿用已配置密钥），跳过校验
		if strings.TrimSpace(fmt.Sprintf("%v", value)) == "" {
			return nil
		}
		// 保存前发起模拟验证请求，确认密钥被 Cloudflare 识别
		if err := probeTurnstileSecret(fmt.Sprintf("%v", value)); err != nil {
			return err
		}
	case "cappow_difficulty":
		diff, err := settingValueToInt(value)
		if err != nil {
			return fmt.Errorf("cap-pow 难度必须为整数")
		}
		if diff < 1 || diff > 8 {
			return fmt.Errorf("cap-pow 难度必须在 1-8 之间")
		}
	case "encrypted_storage":
		enabled, err := convertValueToTargetType(key, value, reflect.TypeOf(false))
		if err != nil {
			return err
		}
		if enabled.(bool) {
			setting, err := settings.GetSettings()
			if err != nil {
				return fmt.Errorf("获取系统配置失败")
			}
			if strings.TrimSpace(setting.PublicImageDomain) != "" {
				return fmt.Errorf("加密存储要求所有图片经程序解密，请先清空图片直链域名")
			}
			if _, err := securestorage.Encrypt(nil); err != nil {
				return err
			}
		}
	case "public_image_domain":
		domain, err := publicurl.NormalizeDomain(fmt.Sprintf("%v", value))
		if err != nil {
			return err
		}
		if domain == "" {
			return nil
		}
		setting, err := settings.GetSettings()
		if err != nil {
			return fmt.Errorf("获取系统配置失败")
		}
		if setting.EncryptedStorage {
			return fmt.Errorf("加密存储已开启，图片必须经程序解密，不能配置直链域名")
		}
		bucketType, err := getBucketTypeByID(setting.DefaultStorage)
		if err != nil {
			return err
		}
		if !publicurl.SupportsStorage(bucketType) {
			return fmt.Errorf("当前默认存储不支持图片直链域名，请先切换到 S3 或 R2 存储")
		}

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
	case "guest_storage":
		return validateGuestStorage(value)
	case "default_storage":
		// 检查存储配置是否存在
		id, err := settingValueToInt(value)
		if err != nil {
			return fmt.Errorf("%s", "解析失败: "+err.Error())
		}

		bucketType, err := getBucketTypeByID(id)
		if err != nil {
			return err
		}
		setting, err := settings.GetSettings()
		if err != nil {
			return fmt.Errorf("获取系统配置失败")
		}
		if publicurl.HasDomain(setting) && !publicurl.SupportsStorage(bucketType) {
			return fmt.Errorf("图片直链域名仅支持 S3/R2 存储，请先清空该配置")
		}
	}

	return nil
}
