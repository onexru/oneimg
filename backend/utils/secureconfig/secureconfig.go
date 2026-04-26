package secureconfig

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"

	"oneimg/backend/config"
	"oneimg/backend/models"

	"golang.org/x/crypto/bcrypt"
)

const encryptedPrefix = "enc:v1:"

var bucketSensitiveKeys = map[string]struct{}{
	"s3_access_key": {},
	"s3_secret_key": {},
	"r2_access_key": {},
	"r2_secret_key": {},
	"ftp_user":      {},
	"ftp_pass":      {},
	"webdav_user":   {},
	"webdav_pass":   {},
	"tg_bot_token":  {},
}

var settingsSensitiveKeys = map[string]struct{}{
	"api_token":    {},
	"tg_bot_token": {},
}

func EncryptBucketConfigValues(configMap map[string]any) (map[string]any, error) {
	return encryptMapValues(configMap, bucketSensitiveKeys)
}

func DecryptBucketConfigValues(configMap map[string]any) (map[string]any, error) {
	return decryptMapValues(configMap, bucketSensitiveKeys)
}

func MaskBucketConfigValues(configMap map[string]any) map[string]any {
	return maskMapValues(configMap, bucketSensitiveKeys)
}

func IsBucketSensitiveKey(key string) bool {
	_, ok := bucketSensitiveKeys[key]
	return ok
}

func IsSettingsSensitiveKey(key string) bool {
	_, ok := settingsSensitiveKeys[key]
	return ok
}

const ConfiguredStatus = ""

func SanitizeSettingsForResponse(setting models.Settings) map[string]any {
	tgBotTokenStatus := ""
	if strings.TrimSpace(setting.TGBotToken) != "" {
		tgBotTokenStatus = ConfiguredStatus
	}

	apiTokenStatus := ""
	if strings.TrimSpace(setting.APITokenHash) != "" {
		apiTokenStatus = ConfiguredStatus
	}

	return map[string]any{
		"id":                      setting.ID,
		"original_image":          setting.OriginalImage,
		"save_webp":               setting.SaveWebp,
		"thumbnail":               setting.Thumbnail,
		"tourist":                 setting.Tourist,
		"tg_notice":               setting.TGNotice,
		"pow_verify":              setting.PowVerify,
		"tg_bot_token":            tgBotTokenStatus,
		"tg_bot_token_configured": strings.TrimSpace(setting.TGBotToken) != "",
		"tg_receivers":            setting.TGReceivers,
		"tg_notice_text":          setting.TGNoticeText,
		"start_api":               setting.StartAPI,
		"api_token":               apiTokenStatus,
		"api_token_configured":    strings.TrimSpace(setting.APITokenHash) != "",
		"save_original_name":      setting.SaveOriginalName,
		"default_storage":         setting.DefaultStorage,
		"max_file_size":           setting.MaxFileSize,
		"allowed_types":           setting.AllowedTypes,
		"watermark_enable":        setting.WatermarkEnable,
		"watermark_text":          setting.WatermarkText,
		"watermark_pos":           setting.WatermarkPos,
		"watermark_size":          setting.WatermarkSize,
		"watermark_color":         setting.WatermarkColor,
		"watermark_opac":          setting.WatermarkOpac,
		"referer_white_enable":    setting.RefererWhiteEnable,
		"referer_white_list":      setting.RefererWhiteList,
		"seo_title":               setting.SEOTitle,
		"seo_description":         setting.SEODescription,
		"seo_keywords":            setting.SEOKeywords,
		"seo_icp":                 setting.SEOICP,
		"public_security":         setting.PublicSecurity,
		"seo_icon":                setting.SEOicon,
	}
}

func NormalizeSettingValue(key string, value any) (any, error) {
	if _, ok := settingsSensitiveKeys[key]; !ok {
		return value, nil
	}

	strValue := strings.TrimSpace(toString(value))
	if strValue == "" || strValue == ConfiguredStatus {
		return nil, nil
	}

	if key == "api_token" {
		hash, err := bcrypt.GenerateFromPassword([]byte(strValue), bcrypt.DefaultCost)
		if err != nil {
			return nil, err
		}
		return string(hash), nil
	}

	return encryptString(strValue)
}

func CompareSecretHash(hashValue, rawValue string) bool {
	if strings.TrimSpace(hashValue) == "" || strings.TrimSpace(rawValue) == "" {
		return false
	}
	err := bcrypt.CompareHashAndPassword([]byte(hashValue), []byte(rawValue))
	return err == nil
}

func TryMigrateSettingsSecrets(setting *models.Settings) (bool, error) {
	changed := false

	if strings.TrimSpace(setting.TGBotToken) != "" && !IsEncryptedValue(setting.TGBotToken) {
		encrypted, err := encryptString(setting.TGBotToken)
		if err != nil {
			return false, err
		}
		setting.TGBotToken = encrypted
		changed = true
	}

	if strings.TrimSpace(setting.APIToken) != "" && strings.TrimSpace(setting.APITokenHash) == "" {
		hash, err := bcrypt.GenerateFromPassword([]byte(setting.APIToken), bcrypt.DefaultCost)
		if err != nil {
			return false, err
		}
		setting.APITokenHash = string(hash)
		setting.APIToken = ""
		changed = true
	}

	return changed, nil
}

func DecryptSettingValue(key, value string) (string, error) {
	if !IsSettingsSensitiveKey(key) {
		return value, nil
	}
	trimmed := strings.TrimSpace(value)
	if trimmed == "" || !IsEncryptedValue(trimmed) {
		return value, nil
	}
	return decryptString(trimmed)
}

func IsEncryptedValue(value string) bool {
	return strings.HasPrefix(strings.TrimSpace(value), encryptedPrefix)
}

func GetString(configMap map[string]any, key string) string {
	if configMap == nil {
		return ""
	}
	return toString(configMap[key])
}

func GetInt(configMap map[string]any, key string) int {
	if configMap == nil {
		return 0
	}
	switch value := configMap[key].(type) {
	case int:
		return value
	case int32:
		return int(value)
	case int64:
		return int(value)
	case float64:
		return int(value)
	case float32:
		return int(value)
	case string:
		parsed, err := strconv.Atoi(strings.TrimSpace(value))
		if err == nil {
			return parsed
		}
	}
	return 0
}

func encryptMapValues(configMap map[string]any, sensitiveKeys map[string]struct{}) (map[string]any, error) {
	result := make(map[string]any, len(configMap))
	for key, value := range configMap {
		result[key] = value
		if _, ok := sensitiveKeys[key]; !ok {
			continue
		}
		stringValue := strings.TrimSpace(toString(value))
		if stringValue == "" || IsEncryptedValue(stringValue) {
			result[key] = stringValue
			continue
		}
		encrypted, err := encryptString(stringValue)
		if err != nil {
			return nil, err
		}
		result[key] = encrypted
	}
	return result, nil
}

func decryptMapValues(configMap map[string]any, sensitiveKeys map[string]struct{}) (map[string]any, error) {
	result := make(map[string]any, len(configMap))
	for key, value := range configMap {
		result[key] = value
		if _, ok := sensitiveKeys[key]; !ok {
			continue
		}
		stringValue := toString(value)
		if stringValue == "" || !IsEncryptedValue(stringValue) {
			result[key] = stringValue
			continue
		}
		decrypted, err := decryptString(stringValue)
		if err != nil {
			return nil, err
		}
		result[key] = decrypted
	}
	return result, nil
}

func maskMapValues(configMap map[string]any, sensitiveKeys map[string]struct{}) map[string]any {
	result := make(map[string]any, len(configMap))
	for key, value := range configMap {
		result[key] = value
		if _, ok := sensitiveKeys[key]; ok {
			strVal := strings.TrimSpace(toString(value))
			if strVal != "" {
				result[key] = ConfiguredStatus
				result[key+"_configured"] = true
			} else {
				result[key] = ""
				result[key+"_configured"] = false
			}
		}
	}
	return result
}

func encryptString(plainText string) (string, error) {
	secret := getSecretKey()
	block, err := aes.NewCipher(secret)
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}

	cipherText := gcm.Seal(nonce, nonce, []byte(plainText), nil)
	return encryptedPrefix + base64.StdEncoding.EncodeToString(cipherText), nil
}

func decryptString(cipherText string) (string, error) {
	secret := getSecretKey()
	encoded := strings.TrimPrefix(cipherText, encryptedPrefix)
	raw, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return "", err
	}

	block, err := aes.NewCipher(secret)
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonceSize := gcm.NonceSize()
	if len(raw) < nonceSize {
		return "", errors.New("密文格式无效")
	}

	nonce, cipherBytes := raw[:nonceSize], raw[nonceSize:]
	plain, err := gcm.Open(nil, nonce, cipherBytes, nil)
	if err != nil {
		return "", err
	}
	return string(plain), nil
}

func getSecretKey() []byte {
	secret := strings.TrimSpace(config.App.ConfigSecret)
	if secret == "" {
		secret = strings.TrimSpace(config.App.SessionSecret)
	}
	sum := sha256.Sum256([]byte(secret))
	return sum[:]
}

func toString(value any) string {
	switch val := value.(type) {
	case nil:
		return ""
	case string:
		return val
	case fmt.Stringer:
		return val.String()
	case float64:
		return strconv.FormatFloat(val, 'f', -1, 64)
	case float32:
		return strconv.FormatFloat(float64(val), 'f', -1, 32)
	case int:
		return strconv.Itoa(val)
	case int64:
		return strconv.FormatInt(val, 10)
	case int32:
		return strconv.FormatInt(int64(val), 10)
	case uint64:
		return strconv.FormatUint(val, 10)
	case uint32:
		return strconv.FormatUint(uint64(val), 10)
	case bool:
		return strconv.FormatBool(val)
	default:
		return fmt.Sprintf("%v", val)
	}
}

func ConstantTimeEqual(a, b string) bool {
	if len(a) == 0 || len(b) == 0 {
		return false
	}
	macA := hmac.New(sha256.New, []byte("oneimg-constant-time"))
	macA.Write([]byte(a))
	macB := hmac.New(sha256.New, []byte("oneimg-constant-time"))
	macB.Write([]byte(b))
	return subtle.ConstantTimeCompare(macA.Sum(nil), macB.Sum(nil)) == 1
}
