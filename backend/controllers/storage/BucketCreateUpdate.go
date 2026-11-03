package storage

import (
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	shared "oneimg/backend/controllers/shared"
	"oneimg/backend/database"
	"oneimg/backend/models"
	"oneimg/backend/utils/buckets"
	"oneimg/backend/utils/result"
	"oneimg/backend/utils/secureconfig"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

// AddBuckets 新增存储桶。
func AddBuckets(c *gin.Context) {
	// 读取原始 body，便于后续 JSON 预处理（如 FTP 端口类型）
	bodyBytes, err := c.GetRawData()
	if err != nil {
		c.JSON(http.StatusBadRequest, result.Error(400, "读取请求内容失败，请重试"))
		return
	}

	// 第一次解析：解析为动态map，提取name和type
	var params map[string]any
	if err := json.Unmarshal(bodyBytes, &params); err != nil {
		shared.RespondValidationError(c, err, "请求参数格式不正确")
		return
	}

	// 基础参数校验
	if params["name"] == nil || params["type"] == nil {
		c.JSON(http.StatusBadRequest, result.Error(400, "name和type为必填参数"))
		return
	}

	// 类型断言并校验
	name, okName := params["name"].(string)
	type_, okType := params["type"].(string)
	if !okName || !okType || name == "" || type_ == "" {
		c.JSON(http.StatusBadRequest, result.Error(400, "name和type必须为非空字符串"))
		return
	}

	// 校验type合法性
	validTypes := []string{"s3", "r2", "ftp", "webdav", "telegram"}
	if !sliceContains(validTypes, type_) {
		c.JSON(http.StatusBadRequest, result.Error(400, "type参数错误，合法值：s3/r2/ftp/webdav/telegram"))
		return
	}
	if err := validateStorageProvider(type_, params); err != nil {
		c.JSON(http.StatusBadRequest, result.Error(400, err.Error()))
		return
	}

	var capacity float64
	var capacitybytes uint64
	if type_ != "telegram" {
		capacityStr, ok := params["capacity"].(string)
		if !ok {
			c.JSON(http.StatusBadRequest, result.Error(400, "capacity必须是非负数值字符串"))
			return
		}
		// 将参数转化为int
		if capacityStr == "" {
			c.JSON(http.StatusBadRequest, result.Error(400, "capacity为必填参数"))
			return
		}
		capacity, err = strconv.ParseFloat(capacityStr, 64)
		if err != nil || math.IsNaN(capacity) || math.IsInf(capacity, 0) || capacity < 0 || capacity > float64(^uint64(0))/(1024*1024*1024) {
			c.JSON(http.StatusBadRequest, result.Error(400, "capacity参数错误"))
			return
		}
		if capacity <= 0 {
			c.JSON(http.StatusBadRequest, result.Error(400, "capacity必须大于0"))
			return
		}
		// 保留两位小数
		capacity = keepTwoDecimal(capacity)
		// GB -> B
		capacitybytes = uint64(capacity * 1024 * 1024 * 1024)
	} else {
		capacitybytes = 0
	}

	// 第二次解析：根据type解析为对应结构体
	var bucketConfig map[string]any
	switch type_ {
	case "s3":
		var s3Bucket models.S3Bucket
		if err := json.Unmarshal(bodyBytes, &s3Bucket); err != nil {
			shared.RespondValidationError(c, err, "S3参数格式不正确")
			return
		}
		bucketConfig = buckets.S3BucketToMap(s3Bucket)
	case "r2":
		var r2Bucket models.R2Bucket
		if err := json.Unmarshal(bodyBytes, &r2Bucket); err != nil {
			shared.RespondValidationError(c, err, "R2参数格式不正确")
			return
		}
		bucketConfig = buckets.R2BucketToMap(r2Bucket)
	case "ftp":
		var ftpBucket models.FTPBucket
		newBodyBytes, err := ftpBodyBytesPortToInt(bodyBytes)
		if err != nil {
			shared.RespondValidationError(c, err, "FTP 端口必须是 1-65535 之间的整数")
			return
		}
		if err := json.Unmarshal(newBodyBytes, &ftpBucket); err != nil {
			shared.RespondValidationError(c, err, "FTP参数格式不正确")
			return
		}
		bucketConfig = buckets.FTPBucketToMap(ftpBucket)
	case "webdav":
		var webdavBucket models.WebDavBucket
		if err := json.Unmarshal(bodyBytes, &webdavBucket); err != nil {
			shared.RespondValidationError(c, err, "WebDAV参数格式不正确")
			return
		}
		bucketConfig = buckets.WebDavBucketToMap(webdavBucket)
	case "telegram":
		var telegramBucket models.TelegramBucket
		if err := json.Unmarshal(bodyBytes, &telegramBucket); err != nil {
			shared.RespondValidationError(c, err, "Telegram参数格式不正确")
			return
		}
		bucketConfig = buckets.TelegramBucketToMap(telegramBucket)
	default:
		c.JSON(http.StatusBadRequest, result.Error(400, "不支持的存储类型"))
		return
	}

	err = ValidateBucketValues(bucketConfig)
	if err != nil {
		c.JSON(http.StatusBadRequest, result.Error(400, err.Error()))
		return
	}

	// 保存前拦住内网端点：否则攻击者可先把内网地址存成一个存储桶，
	// 再借「测试连接」或后续上传把请求打到内网（保存后再校验等于已埋雷）。
	if err := validateStorageEndpoint(type_, bucketConfig); err != nil {
		c.JSON(http.StatusBadRequest, result.Error(400, err.Error()))
		return
	}

	encryptedConfig, err := secureconfig.EncryptBucketConfigValues(bucketConfig)
	if err != nil {
		c.JSON(http.StatusInternalServerError, result.Error(500, "敏感配置加密失败"))
		return
	}

	// 插入数据库
	db := database.GetDB()
	bucket := models.Buckets{
		Name:     name,
		Type:     type_,
		Capacity: capacitybytes,
		Config:   encryptedConfig,
		Usage:    0,
	}
	if err := db.DB.Create(&bucket).Error; err != nil {
		// 判断是否已存在同名存储桶
		if strings.Contains(err.Error(), "UNIQUE constraint failed: buckets.name") {
			c.JSON(http.StatusConflict, result.Error(409, "存储桶已存在"))
			return
		}
		shared.InternalFailure(c, "添加存储失败", err)
		return
	}

	responseBucket := bucket
	responseBucket.Config = secureconfig.MaskBucketConfigValues(bucket.Config)
	c.JSON(http.StatusOK, result.Success("添加成功", responseBucket))
}

// UpdateBuckets 更新存储桶配置。
func UpdateBuckets(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, result.Error(400, "id不能为空"))
		return
	}

	if id == "1" {
		c.JSON(http.StatusBadRequest, result.Error(400, "默认存储桶不能编辑"))
		return
	}

	// 查询存储桶信息
	db := database.GetDB()
	var bucket models.Buckets
	if err := db.DB.Where("id = ?", id).First(&bucket).Error; err != nil {
		c.JSON(http.StatusNotFound, result.Error(404, "存储桶不存在"))
		return
	}

	// 读取原始 body，便于后续 JSON 预处理（如 FTP 端口类型）
	bodyBytes, err := c.GetRawData()
	if err != nil {
		c.JSON(http.StatusBadRequest, result.Error(400, "读取请求内容失败，请重试"))
		return
	}

	// 第一次解析：解析为动态map，提取name和type
	var params map[string]any
	if err := json.Unmarshal(bodyBytes, &params); err != nil {
		shared.RespondValidationError(c, err, "请求参数格式不正确")
		return
	}

	// 类型断言并校验
	name, okName := params["name"].(string)
	type_, okType := params["type"].(string)
	if !okName || !okType || name == "" || type_ == "" {
		c.JSON(http.StatusBadRequest, result.Error(400, "name和type必须为非空字符串"))
		return
	}

	var capacity float64
	var capacitybytes uint64
	if type_ != "telegram" {
		capacityStr, ok := params["capacity"].(string)
		if !ok {
			c.JSON(http.StatusBadRequest, result.Error(400, "capacity必须是非负数值字符串"))
			return
		}
		// 将参数转化为int
		if capacityStr == "" {
			c.JSON(http.StatusBadRequest, result.Error(400, "capacity为必填参数"))
			return
		}
		capacity, err = strconv.ParseFloat(capacityStr, 64)
		if err != nil || math.IsNaN(capacity) || math.IsInf(capacity, 0) || capacity < 0 || capacity > float64(^uint64(0))/(1024*1024*1024) {
			c.JSON(http.StatusBadRequest, result.Error(400, "capacity参数错误"))
			return
		}
		if capacity <= 0 {
			c.JSON(http.StatusBadRequest, result.Error(400, "capacity必须大于0"))
			return
		}
		// 保留两位小数
		capacity = keepTwoDecimal(capacity)
		// GB -> B
		capacitybytes = uint64(capacity * 1024 * 1024 * 1024)
	} else {
		capacitybytes = 0
	}

	if type_ != bucket.Type {
		c.JSON(http.StatusBadRequest, result.Error(400, "存储类型不可直接修改"))
		return
	}
	if err := validateStorageProvider(type_, params); err != nil {
		c.JSON(http.StatusBadRequest, result.Error(400, err.Error()))
		return
	}

	if capacitybytes < bucket.Usage && bucket.Type != "telegram" {
		c.JSON(http.StatusBadRequest, result.Error(400, "总容量不能小于已使用容量"))
		return
	}

	// 第二次解析：根据type解析为对应结构体
	var bucketConfig map[string]any
	switch type_ {
	case "s3":
		var s3Bucket models.S3Bucket
		if err := json.Unmarshal(bodyBytes, &s3Bucket); err != nil {
			shared.RespondValidationError(c, err, "S3参数格式不正确")
			return
		}
		bucketConfig = buckets.S3BucketToMap(s3Bucket)
	case "r2":
		var r2Bucket models.R2Bucket
		if err := json.Unmarshal(bodyBytes, &r2Bucket); err != nil {
			shared.RespondValidationError(c, err, "R2参数格式不正确")
			return
		}
		bucketConfig = buckets.R2BucketToMap(r2Bucket)
	case "ftp":
		var ftpBucket models.FTPBucket
		newBodyBytes, err := ftpBodyBytesPortToInt(bodyBytes)
		if err != nil {
			shared.RespondValidationError(c, err, "FTP 端口必须是 1-65535 之间的整数")
			return
		}
		if err := json.Unmarshal(newBodyBytes, &ftpBucket); err != nil {
			shared.RespondValidationError(c, err, "FTP参数格式不正确")
			return
		}
		bucketConfig = buckets.FTPBucketToMap(ftpBucket)
	case "webdav":
		var webdavBucket models.WebDavBucket
		if err := json.Unmarshal(bodyBytes, &webdavBucket); err != nil {
			shared.RespondValidationError(c, err, "WebDAV参数格式不正确")
			return
		}
		bucketConfig = buckets.WebDavBucketToMap(webdavBucket)
	case "telegram":
		var telegramBucket models.TelegramBucket
		if err := json.Unmarshal(bodyBytes, &telegramBucket); err != nil {
			shared.RespondValidationError(c, err, "Telegram参数格式不正确")
			return
		}
		bucketConfig = buckets.TelegramBucketToMap(telegramBucket)
	default:
		c.JSON(http.StatusBadRequest, result.Error(400, "不支持的存储类型"))
		return
	}

	err = ValidateBucketValues(bucketConfig)
	if err != nil {
		c.JSON(http.StatusBadRequest, result.Error(400, err.Error()))
		return
	}

	// Older clients omit new optional fields; preserve them unless explicitly cleared.
	if type_ == "r2" {
		for _, key := range []string{"r2_cdn_domain", "r2_cdn_mode"} {
			if _, supplied := params[key]; !supplied {
				delete(bucketConfig, key)
			}
		}
	}
	// 端点变更必须重新授权：不允许「改地址 + 留空口令」沿用库内凭据，
	// 否则等于把读不到的口令发往新地址，且后续上传被持久导向攻击者。
	if err := ensureEndpointChangeReauthorized(type_, bucket.Config, bucketConfig); err != nil {
		c.JSON(http.StatusBadRequest, result.Error(400, err.Error()))
		return
	}
	mergedConfig, err := mergeBucketConfig(bucket.Config, bucketConfig)
	if err != nil {
		var requiredSensitiveKeys []string
		switch type_ {
		case "s3":
			requiredSensitiveKeys = []string{"s3_access_key", "s3_secret_key"}
		case "r2":
			requiredSensitiveKeys = []string{"r2_access_key", "r2_secret_key"}
		case "ftp":
			requiredSensitiveKeys = []string{"ftp_user", "ftp_pass"}
		case "webdav":
			requiredSensitiveKeys = []string{"webdav_user", "webdav_pass"}
		case "telegram":
			requiredSensitiveKeys = []string{"tg_bot_token"}
		}

		canOverride := true
		for _, key := range requiredSensitiveKeys {
			val, exists := bucketConfig[key]
			if !exists || strings.TrimSpace(fmt.Sprintf("%v", val)) == "" {
				canOverride = false
				break
			}
		}

		if canOverride {
			mergedConfig = bucketConfig
		} else {
			c.JSON(http.StatusBadRequest, result.Error(400, "原有敏感配置解密失败，请重新填写完整的认证信息以覆盖"))
			return
		}
	}

	if err := ValidateBucketValues(mergedConfig); err != nil {
		c.JSON(http.StatusBadRequest, result.Error(400, err.Error()))
		return
	}
	// 更新后同样不许留下内网端点（原有桶可能是历史上存进去的）。
	if err := validateStorageEndpoint(type_, mergedConfig); err != nil {
		c.JSON(http.StatusBadRequest, result.Error(400, err.Error()))
		return
	}
	encryptedConfig, err := secureconfig.EncryptBucketConfigValues(mergedConfig)
	if err != nil {
		c.JSON(http.StatusInternalServerError, result.Error(500, "敏感配置加密失败"))
		return
	}

	newBucket := models.Buckets{
		Name:     name,
		Capacity: capacitybytes,
		Config:   encryptedConfig,
	}

	// 更新数据库
	if err := db.DB.Model(&bucket).Updates(newBucket).Error; err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint failed: buckets.name") {
			c.JSON(http.StatusConflict, result.Error(409, "存储桶已存在"))
			return
		}
		shared.InternalFailure(c, "更新存储失败", err)
		return
	}

	bucket.Name = newBucket.Name
	bucket.Capacity = newBucket.Capacity
	bucket.Config = secureconfig.MaskBucketConfigValues(newBucket.Config)
	c.JSON(http.StatusOK, result.Success("更新成功", bucket))
}
