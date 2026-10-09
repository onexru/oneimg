package controllers

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
