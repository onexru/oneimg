package controllers

import (
	"github.com/gin-gonic/gin"
	"net/http"
	"oneimg/backend/database"
	"oneimg/backend/models"
	"oneimg/backend/utils/result"
	"oneimg/backend/utils/secureconfig"
)

// GetBuckets 获取全部存储桶及容量使用情况（敏感配置已脱敏）。
func GetBuckets(c *gin.Context) {
	var buckets []models.Buckets
	db := database.GetDB()
	if err := db.DB.Model(&models.Buckets{}).Find(&buckets).Error; err != nil {
		c.JSON(http.StatusInternalServerError, result.Error(500, "获取存储桶失败"))
		return
	}

	// 返回结构体
	type BucketResponse struct {
		models.Buckets
		UsageReadable          string           `json:"usage_readable"`           // 已用容量
		TotalReadable          string           `json:"total_readable"`           // 总容量
		UsagePercent           float64          `json:"usage_percent"`            // 使用率（保留两位小数）
		UsageFree              string           `json:"usage_free"`               // 可用容量
		AccountedUsageReadable string           `json:"accounted_usage_readable"` // 已登记图床副本，不混同整盘占用
		DiskUsage              *DiskUsageDetail `json:"disk_usage,omitempty"`
	}
	var bucketRes []BucketResponse

	for _, bucket := range buckets {
		maskedConfig := secureconfig.MaskBucketConfigValues(bucket.Config)
		bucket.Config = maskedConfig
		res := BucketResponse{Buckets: bucket, AccountedUsageReadable: formatSize(bucket.Usage)}
		// 根据存储类型计算/转换容量和使用量
		switch bucket.Type {
		case "default": // 本地磁盘
			diskInfo, err := getDiskUsage()
			if err != nil {
				res.UsageReadable = "获取失败"
				bucketRes = append(bucketRes, res)
				continue
			}
			res.DiskUsage = &diskInfo
			res.TotalReadable = diskInfo.Total
			res.UsageReadable = diskInfo.Used
			res.UsageFree = diskInfo.Free
			res.UsagePercent = keepTwoDecimal(diskInfo.Percent) // 保留两位小数
		case "s3", "r2", "ftp", "webdav":
			res.UsageReadable = formatSize(bucket.Usage)
			if bucket.Capacity == 0 {
				res.TotalReadable = "无限"
				res.UsageFree = "无限"
				res.UsagePercent = 0
			} else {
				res.TotalReadable = formatSize(bucket.Capacity)
				res.UsageFree = formatSize(bucketRemaining(bucket.Capacity, bucket.Usage))
				res.UsagePercent = keepTwoDecimal(min(100, float64(bucket.Usage)/float64(bucket.Capacity)*100))
			}
		case "telegram": // Telegram 不限容量
			res.TotalReadable = "无限"
			res.UsageReadable = formatSize(bucket.Usage)
			res.UsageFree = "无限"
			res.UsagePercent = 0
		default:
			res.UsageReadable = "未知类型"
			res.TotalReadable = "未知类型"
			res.UsageFree = "未知类型"
			res.UsagePercent = 0
		}
		bucketRes = append(bucketRes, res)
	}

	c.JSON(http.StatusOK, result.Success("ok", bucketRes))
}

// GetBucketsList 获取启用中的存储桶简要列表（上传选择用）。
func GetBucketsList(c *gin.Context) {
	var buckets []models.Buckets
	db := database.GetDB()
	if err := db.DB.Model(&models.Buckets{}).Find(&buckets).Error; err != nil {
		c.JSON(http.StatusInternalServerError, result.Error(500, "获取存储桶列表失败"))
		return
	}

	var bucketRes []map[string]any
	for _, bucket := range buckets {
		res := map[string]any{
			"id":       bucket.Id,
			"name":     bucket.Name,
			"type":     bucket.Type,
			"enabled":  !bucket.Disabled,
			"disabled": bucket.Disabled,
		}
		bucketRes = append(bucketRes, res)
	}

	c.JSON(http.StatusOK, result.Success("ok", bucketRes))
}

type updateBucketEnabledRequest struct {
	Enabled *bool `json:"enabled" binding:"required"`
}
