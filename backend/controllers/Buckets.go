package controllers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"oneimg/backend/database"
	"oneimg/backend/models"
	"oneimg/backend/utils/buckets"
	"oneimg/backend/utils/result"
	"oneimg/backend/utils/settings"
	"os"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/shirou/gopsutil/v3/disk"
)

type DiskUsageDetail struct {
	TotalBytes uint64  `json:"-"`       // 总容量（字节，不返回前端）
	UsedBytes  uint64  `json:"-"`       // 已用容量（字节，不返回前端）
	FreeBytes  uint64  `json:"-"`       // 可用容量（字节，不返回前端）
	Total      string  `json:"total"`   // 总容量
	Used       string  `json:"used"`    // 已用容量
	Free       string  `json:"free"`    // 可用容量
	Percent    float64 `json:"percent"` // 使用率
}

// 工具函数：保留float64类型数值的两位小数
func keepTwoDecimal(num float64) float64 {
	return float64(int(num*100+0.5)) / 100
}

// 获取所有存储桶
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
		UsageReadable string  `json:"usage_readable"` // 已用容量
		TotalReadable string  `json:"total_readable"` // 总容量
		UsagePercent  float64 `json:"usage_percent"`  // 使用率（保留两位小数）
		UsageFree     string  `json:"usage_free"`     // 可用容量
	}
	var bucketRes []BucketResponse

	for _, bucket := range buckets {
		res := BucketResponse{Buckets: bucket}
		// 根据存储类型计算/转换容量和使用量
		switch bucket.Type {
		case "default": // 本地磁盘
			diskInfo, err := getDiskUsage()
			if err != nil {
				res.UsageReadable = "获取失败"
				bucketRes = append(bucketRes, res)
				continue
			}
			db.DB.Model(&models.Buckets{}).Where("id = ?", bucket.Id).Update("usage", diskInfo.UsedBytes)
			res.TotalReadable = diskInfo.Total
			res.UsageReadable = diskInfo.Used
			res.UsageFree = diskInfo.Free
			res.UsagePercent = keepTwoDecimal(diskInfo.Percent) // 保留两位小数
		case "s3", "r2", "ftp", "webdav":
			// 计算使用量
			res.TotalReadable = formatSize(bucket.Capacity)
			res.UsageReadable = formatSize(bucket.Usage)
			res.UsageFree = formatSize(bucket.Capacity - bucket.Usage)
			usagePercent := keepTwoDecimal(float64(bucket.Usage) / float64(bucket.Capacity) * 100)
			if usagePercent < 0 {
				res.UsagePercent = 0
			} else {
				res.UsagePercent = usagePercent
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

// 获取存储桶列表
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
			"id":   bucket.Id,
			"name": bucket.Name,
			"type": bucket.Type,
		}
		bucketRes = append(bucketRes, res)
	}

	c.JSON(http.StatusOK, result.Success("ok", bucketRes))
}

func AddBuckets(c *gin.Context) {
	// 一次性读取请求体字节，解决EOF核心问题
	bodyBytes, err := c.GetRawData()
	if err != nil {
		c.JSON(http.StatusBadRequest, result.Error(400, "读取请求体失败："+err.Error()))
		return
	}

	// 第一次解析：解析为动态map，提取name和type
	var params map[string]any
	if err := json.Unmarshal(bodyBytes, &params); err != nil {
		c.JSON(http.StatusBadRequest, result.Error(400, "参数解析失败："+err.Error()))
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

	var capacity float64
	var capacitybytes uint64
	if type_ != "telegram" {
		capacityStr := params["capacity"].(string)
		// 将参数转化为int
		if capacityStr == "" {
			c.JSON(http.StatusBadRequest, result.Error(400, "capacity为必填参数"))
			return
		}
		capacity, err = strconv.ParseFloat(capacityStr, 64)
		if err != nil {
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
			c.JSON(http.StatusBadRequest, result.Error(400, "S3参数解析失败："+err.Error()))
			return
		}
		bucketConfig = buckets.S3BucketToMap(s3Bucket)
	case "r2":
		var r2Bucket models.R2Bucket
		if err := json.Unmarshal(bodyBytes, &r2Bucket); err != nil {
			c.JSON(http.StatusBadRequest, result.Error(400, "R2参数解析失败："+err.Error()))
			return
		}
		bucketConfig = buckets.R2BucketToMap(r2Bucket)
	case "ftp":
		var ftpBucket models.FTPBucket
		if err := json.Unmarshal(bodyBytes, &ftpBucket); err != nil {
			c.JSON(http.StatusBadRequest, result.Error(400, "FTP参数解析失败："+err.Error()))
			return
		}
		bucketConfig = buckets.FTPBucketToMap(ftpBucket)
	case "webdav":
		var webdavBucket models.WebDavBucket
		if err := json.Unmarshal(bodyBytes, &webdavBucket); err != nil {
			c.JSON(http.StatusBadRequest, result.Error(400, "WebDAV参数解析失败："+err.Error()))
			return
		}
		bucketConfig = buckets.WebDavBucketToMap(webdavBucket)
	case "telegram":
		var telegramBucket models.TelegramBucket
		if err := json.Unmarshal(bodyBytes, &telegramBucket); err != nil {
			c.JSON(http.StatusBadRequest, result.Error(400, "Telegram参数解析失败："+err.Error()))
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

	// 插入数据库
	db := database.GetDB()
	bucket := models.Buckets{
		Name:     name,
		Type:     type_,
		Capacity: capacitybytes,
		Config:   bucketConfig,
		Usage:    0,
	}
	if err := db.DB.Create(&bucket).Error; err != nil {
		// 判断是否已存在同名存储桶
		if strings.Contains(err.Error(), "UNIQUE constraint failed: buckets.name") {
			c.JSON(http.StatusConflict, result.Error(409, "存储桶已存在"))
			return
		}
		c.JSON(http.StatusInternalServerError, result.Error(500, "添加存储失败："+err.Error()))
		return
	}

	c.JSON(http.StatusOK, result.Success("添加成功", bucket))
}

// 更新存储桶
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

	// 一次性读取请求体字节，解决EOF核心问题
	bodyBytes, err := c.GetRawData()
	if err != nil {
		c.JSON(http.StatusBadRequest, result.Error(400, "读取请求体失败："+err.Error()))
		return
	}

	// 第一次解析：解析为动态map，提取name和type
	var params map[string]any
	if err := json.Unmarshal(bodyBytes, &params); err != nil {
		c.JSON(http.StatusBadRequest, result.Error(400, "参数解析失败："+err.Error()))
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
		capacityStr := params["capacity"].(string)
		// 将参数转化为int
		if capacityStr == "" {
			c.JSON(http.StatusBadRequest, result.Error(400, "capacity为必填参数"))
			return
		}
		capacity, err = strconv.ParseFloat(capacityStr, 64)
		if err != nil {
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
			c.JSON(http.StatusBadRequest, result.Error(400, "S3参数解析失败："+err.Error()))
			return
		}
		bucketConfig = buckets.S3BucketToMap(s3Bucket)
	case "r2":
		var r2Bucket models.R2Bucket
		if err := json.Unmarshal(bodyBytes, &r2Bucket); err != nil {
			c.JSON(http.StatusBadRequest, result.Error(400, "R2参数解析失败："+err.Error()))
			return
		}
		bucketConfig = buckets.R2BucketToMap(r2Bucket)
	case "ftp":
		var ftpBucket models.FTPBucket
		if err := json.Unmarshal(bodyBytes, &ftpBucket); err != nil {
			c.JSON(http.StatusBadRequest, result.Error(400, "FTP参数解析失败："+err.Error()))
			return
		}
		bucketConfig = buckets.FTPBucketToMap(ftpBucket)
	case "webdav":
		var webdavBucket models.WebDavBucket
		if err := json.Unmarshal(bodyBytes, &webdavBucket); err != nil {
			c.JSON(http.StatusBadRequest, result.Error(400, "WebDAV参数解析失败："+err.Error()))
			return
		}
		bucketConfig = buckets.WebDavBucketToMap(webdavBucket)
	case "telegram":
		var telegramBucket models.TelegramBucket
		if err := json.Unmarshal(bodyBytes, &telegramBucket); err != nil {
			c.JSON(http.StatusBadRequest, result.Error(400, "Telegram参数解析失败："+err.Error()))
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

	newBucket := models.Buckets{
		Name:     name,
		Capacity: capacitybytes,
		Config:   bucketConfig,
	}

	// 更新数据库
	if err := db.DB.Model(&bucket).Updates(newBucket).Error; err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint failed: buckets.name") {
			c.JSON(http.StatusConflict, result.Error(409, "存储桶已存在"))
			return
		}
		c.JSON(http.StatusInternalServerError, result.Error(500, "更新存储失败"+err.Error()))
		return
	}

	c.JSON(http.StatusOK, result.Success("更新成功", bucket))
}

// 删除存储桶
func DeleteBuckets(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, result.Error(400, "id不能为空"))
		return
	}

	if id == "1" {
		c.JSON(http.StatusBadRequest, result.Error(400, "默认存储桶不能删除"))
		return
	}

	db := database.GetDB()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	deleteTagsBatch := func() error {
		for {
			result := db.DB.WithContext(ctx).
				Table("image_to_tags").
				Where("image_id IN (SELECT id FROM images WHERE bucket_id = ? LIMIT ?)", id, 1000).
				Delete(&models.ImageToTags{})

			err := result.Error
			rowsAffected := result.RowsAffected

			if err != nil {
				if errors.Is(err, context.DeadlineExceeded) {
					log.Printf("删除tags超时（bucket_id=%s），已删除%d条关联记录", id, rowsAffected)
					break
				}
				log.Printf("批次删除tags失败（bucket_id=%s）：%v", id, err)
				break
			}

			if rowsAffected == 0 {
				log.Printf("bucket_id=%s的关联tags已全部删除完成", id)
				break
			}

			log.Printf("bucket_id=%s：本次删除%d条image_to_tags记录", id, rowsAffected)
		}
		return nil
	}

	deleteImagesBatch := func() error {
		for {
			result := db.DB.WithContext(ctx).
				Table("images").
				Where("bucket_id = ?", id).
				Limit(1000).
				Delete(&models.Image{})

			err := result.Error
			rowsAffected := result.RowsAffected

			if err != nil {
				if errors.Is(err, context.DeadlineExceeded) {
					log.Printf("删除图片超时（bucket_id=%s），已删除%d条图片记录", id, rowsAffected)
					break
				}
				log.Printf("批次删除图片失败（bucket_id=%s）：%v", id, err)
				return err
			}

			if rowsAffected == 0 {
				log.Printf("bucket_id=%s的图片已全部删除完成", id)
				break
			}

			log.Printf("bucket_id=%s：本次删除%d条图片记录", id, rowsAffected)
		}
		return nil
	}

	if err := deleteTagsBatch(); err != nil {
		log.Printf("分批删除image_to_tags失败：%v（bucket_id=%s）", err, id)
	}

	if err := deleteImagesBatch(); err != nil {
		c.JSON(http.StatusInternalServerError, result.Error(500, "删除图片失败："+err.Error()))
		return
	}

	if err := db.DB.Delete(&models.Buckets{}, id).Error; err != nil {
		c.JSON(http.StatusInternalServerError, result.Error(500, "删除存储桶失败："+err.Error()))
		return
	}

	// 检查是否为默认存储桶，如果是，则更新默认存储桶
	setting, err := settings.GetSettings()
	idInt, _ := strconv.ParseInt(id, 10, 32)
	if err != nil {
		log.Printf("获取默认存储桶失败：%v", err)
	} else if setting.DefaultStorage == int(idInt) {
		if err := db.DB.Model(&models.Settings{}).Where("default_storage = ?", id).Update("default_storage", 1).Error; err != nil {
			log.Printf("更新默认存储桶失败：%v", err)
		}
	}

	c.JSON(http.StatusOK, result.Success("删除成功", nil))
}

// 辅助函数：检查是否包含某个元素
func sliceContains(slice []string, target string) bool {
	return slices.Contains(slice, target)
}

// 辅助函数：获取磁盘使用情况
func getDiskUsage() (diskInfo DiskUsageDetail, err error) {
	path, err := os.Getwd()
	if err != nil {
		return DiskUsageDetail{}, err
	}
	usage, err := disk.Usage(path)
	if err != nil {
		return DiskUsageDetail{}, err
	}

	return DiskUsageDetail{
		TotalBytes: usage.Total,
		UsedBytes:  usage.Used,
		FreeBytes:  usage.Free,
		Total:      formatSize(usage.Total),
		Used:       formatSize(usage.Used),
		Free:       formatSize(usage.Free),
		Percent:    usage.UsedPercent,
	}, nil
}

// 辅助函数：将字节单位转换为易读的格式
func formatSize(bytes uint64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := uint64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.2f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

// 辅助函数：校验空值
func ValidateBucketValues(bucketMap map[string]any) (err error) {
	for key, val := range bucketMap {
		if val == "" {
			return fmt.Errorf("%s 为必填项", key)
		}
	}
	return nil
}
