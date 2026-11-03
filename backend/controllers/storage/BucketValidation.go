package storage

import (
	"encoding/json"
	"errors"
	"fmt"
	"oneimg/backend/utils/buckets"
	storageS3 "oneimg/backend/utils/s3"
	"oneimg/backend/utils/secureconfig"
	"slices"
	"strconv"
	"strings"
)

// bucketEndpointConfigKeys 每种存储类型中「决定数据发往哪里」的字段。
// 与 BucketTest.go 的 bucketEndpointKeys 保持一致（那里用于连接测试，
// 这里用于持久化更新）。
var bucketEndpointConfigKeys = map[string][]string{
	"s3":       {"s3_endpoint"},
	"r2":       {"r2_endpoint"},
	"webdav":   {"webdav_url"},
	"ftp":      {"ftp_host", "ftp_port"},
	"telegram": {},
	"default":  {"storagePath"},
}

// bucketRequiredCredentialKeys 每种存储类型里必须由调用方显式提供的凭据字段。
// 这些字段留空时会沿用库里已存（解密后的）口令 —— 正是「改端点 + 留空口令」
// 这一组合让攻击者能把库内口令发往自己的服务器。
var bucketRequiredCredentialKeys = map[string][]string{
	"s3":       {"s3_access_key", "s3_secret_key"},
	"r2":       {"r2_access_key", "r2_secret_key"},
	"webdav":   {"webdav_user", "webdav_pass"},
	"ftp":      {"ftp_user", "ftp_pass"},
	"telegram": {"tg_bot_token"},
	"default":  {},
}

// endpointChanged 判断本次提交是否改动了存储端点。
func endpointChanged(storageType string, existingConfig, incomingConfig map[string]any) bool {
	for _, key := range bucketEndpointConfigKeys[storageType] {
		incoming, supplied := incomingConfig[key]
		if !supplied {
			continue
		}
		old := fmt.Sprintf("%v", existingConfig[key])
		next := fmt.Sprintf("%v", incoming)
		if strings.TrimSpace(next) == "" {
			continue
		}
		if strings.TrimSpace(old) != strings.TrimSpace(next) {
			return true
		}
	}
	return false
}

// credentialsSupplied 判断调用方是否把该类型的全部必填凭据都重新填了一遍。
func credentialsSupplied(storageType string, incomingConfig map[string]any) bool {
	for _, key := range bucketRequiredCredentialKeys[storageType] {
		value, exists := incomingConfig[key]
		if !exists || strings.TrimSpace(fmt.Sprintf("%v", value)) == "" {
			return false
		}
	}
	return true
}

// ensureEndpointChangeReauthorized 阻止「保留库内口令、只换端点」的更新。
//
// 攻击模型：持 storage:update 的账号把 webdav_url 改成自己的地址、口令字段留空，
// 服务端就会用库里解密出来的旧口令去连新地址 —— 等于把读不到的口令偷走，
// 且此后所有上传都会被持久导向攻击者。
// 因此端点一旦变化，必须重新完整填写凭据，由调用方证明自己本来就知道口令。
func ensureEndpointChangeReauthorized(storageType string, existingConfig, incomingConfig map[string]any) error {
	if !endpointChanged(storageType, existingConfig, incomingConfig) {
		return nil
	}
	// 取解密后的库内配置来比对端点
	decryptedExisting, err := secureconfig.DecryptBucketConfigValues(existingConfig)
	if err != nil {
		// 解不开就按「端点确实变了」处理，要求重新填写
		decryptedExisting = existingConfig
	}
	if !endpointChanged(storageType, decryptedExisting, incomingConfig) {
		return nil
	}
	if credentialsSupplied(storageType, incomingConfig) {
		return nil
	}
	return errors.New("修改存储端点时必须重新填写完整的认证信息（用户名/密码或密钥），不能沿用已保存的凭据")
}

// 辅助函数：检查是否包含某个元素
func sliceContains(slice []string, target string) bool {
	return slices.Contains(slice, target)
}

// 辅助函数：校验空值
// Keep provider-specific behavior explicit instead of hiding R2 under S3.
func validateStorageProvider(storageType string, params map[string]any) error {
	if storageType == "s3" {
		endpoint, _ := params["s3_endpoint"].(string)
		if storageS3.IsR2Endpoint(endpoint) {
			return errors.New("Cloudflare R2 请使用独立的 Cloudflare R2 存储类型")
		}
	}
	if storageType == "r2" {
		endpoint, _ := params["r2_endpoint"].(string)
		if !storageS3.IsR2Endpoint(endpoint) || !strings.HasPrefix(strings.ToLower(strings.TrimSpace(endpoint)), "https://") {
			return errors.New("请输入 Cloudflare R2 的 HTTPS S3 API 地址")
		}
	}
	return nil
}

func ValidateBucketValues(bucketMap map[string]any) (err error) {
	if domain, ok := bucketMap["r2_cdn_domain"]; ok {
		value, valid := domain.(string)
		mode, _ := bucketMap["r2_cdn_mode"].(string)
		if !valid {
			return errors.New("CDN 地址格式无效")
		}
		if _, _, err := buckets.NormalizeR2CDN(value, mode); err != nil {
			return err
		}
	}
	for key, val := range bucketMap {
		if key == "r2_cdn_domain" || key == "r2_cdn_mode" {
			continue
		}
		if secureconfig.IsBucketSensitiveKey(key) {
			continue
		}
		if val == "" {
			return fmt.Errorf("%s 为必填项", key)
		}
	}
	return nil
}

func mergeBucketConfig(existingConfig map[string]any, incomingConfig map[string]any) (map[string]any, error) {
	decryptedExisting, err := secureconfig.DecryptBucketConfigValues(existingConfig)
	if err != nil {
		return nil, err
	}

	merged := make(map[string]any, len(decryptedExisting)+len(incomingConfig))
	for key, value := range decryptedExisting {
		merged[key] = value
	}

	for key, value := range incomingConfig {
		if secureconfig.IsBucketSensitiveKey(key) && strings.TrimSpace(fmt.Sprintf("%v", value)) == "" {
			continue
		}
		merged[key] = value
	}

	return merged, nil
}

// 工具函数，将FTP端口为Int类型
func ftpBodyBytesPortToInt(bodyBytes []byte) ([]byte, error) {
	var tempMap map[string]any
	if err := json.Unmarshal(bodyBytes, &tempMap); err != nil {
		return nil, err
	}

	if portStr, ok := tempMap["ftp_port"].(string); ok {
		portNum, err := strconv.Atoi(portStr)
		if err != nil {
			return nil, errors.New("ftp_port必须为数字")
		}
		tempMap["ftp_port"] = portNum
	}

	newBody, err := json.Marshal(tempMap)
	if err != nil {
		return nil, err
	}

	return newBody, nil
}
