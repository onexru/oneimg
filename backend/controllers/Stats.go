package controllers

import (
	"net/http"
	"time"

	"oneimg/backend/database"
	"oneimg/backend/models"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// StatsResponse 统计响应结构
type StatsResponse struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
}

// DashboardStats 仪表板统计数据
type DashboardStats struct {
	TotalImages      int64                  `json:"total_images"`
	TotalSize        int64                  `json:"total_size"`
	TodayUploads     int64                  `json:"today_uploads"`
	MonthUploads     int64                  `json:"month_uploads"`
	RecentImages     []models.Image         `json:"recent_images"`
	UploadTrend      []UploadTrendItem      `json:"upload_trend"`
	FormatStats      []FormatStatsItem      `json:"format_stats"`
	SizeDistribution []SizeDistributionItem `json:"size_distribution"`
}

// UploadTrendItem 上传趋势项
type UploadTrendItem struct {
	Date  string `json:"date"`
	Count int64  `json:"count"`
}

// FormatStatsItem 格式统计项
type FormatStatsItem struct {
	Format string `json:"format"`
	Count  int64  `json:"count"`
	Size   int64  `json:"size"`
}

// SizeDistributionItem 大小分布项
type SizeDistributionItem struct {
	Range string `json:"range"`
	Count int64  `json:"count"`
}

// GetDashboardStats 获取仪表板统计数据
func GetDashboardStats(c *gin.Context) {
	db := database.GetDB().DB

	var stats DashboardStats

	// 获取总图片数量
	db.Model(&models.Image{}).Count(&stats.TotalImages)

	// 获取总大小
	var totalSize struct {
		Total int64
	}
	db.Model(&models.Image{}).Select("COALESCE(SUM(file_size), 0) as total").Scan(&totalSize)
	stats.TotalSize = totalSize.Total

	// 获取今日上传数量
	today := time.Now().Format("2006-01-02")
	db.Model(&models.Image{}).Where("DATE(created_at) = ?", today).Count(&stats.TodayUploads)

	// 获取本月上传数量
	now := time.Now()
	year := now.Year()
	month := now.Month()

	startTime := time.Date(year, time.Month(month), 1, 0, 0, 0, 0, time.UTC)
	endTime := startTime.AddDate(0, 1, 0)

	db.Model(&models.Image{}).Where("created_at >= ? AND created_at < ?", startTime, endTime).Count(&stats.MonthUploads)

	// 获取最近上传的图片
	db.Order("created_at DESC").Limit(10).Find(&stats.RecentImages)

	// 获取最近7天的上传趋势
	stats.UploadTrend = getUploadTrend(db, 7)

	// 获取格式统计
	stats.FormatStats = getFormatStats(db)

	// 获取大小分布
	stats.SizeDistribution = getSizeDistribution(db)

	c.JSON(http.StatusOK, StatsResponse{
		Code:    200,
		Message: "获取统计数据成功",
		Success: true,
		Data:    stats,
	})
}

// getUploadTrend 获取上传趋势
func getUploadTrend(db *gorm.DB, days int) []UploadTrendItem {
	var trend []UploadTrendItem

	for i := days - 1; i >= 0; i-- {
		date := time.Now().AddDate(0, 0, -i).Format("2006-01-02")

		var count int64
		db.Model(&models.Image{}).Where("DATE(created_at) = ?", date).Count(&count)

		trend = append(trend, UploadTrendItem{
			Date:  date,
			Count: count,
		})
	}

	return trend
}

// getFormatStats 获取格式统计
func getFormatStats(db *gorm.DB) []FormatStatsItem {
	var stats []FormatStatsItem

	rows, err := db.Model(&models.Image{}).
		Select("mime_type as format, COUNT(*) as count, COALESCE(SUM(file_size), 0) as size").
		Group("mime_type").
		Rows()

	if err != nil {
		return stats
	}
	defer rows.Close()

	for rows.Next() {
		var item FormatStatsItem
		rows.Scan(&item.Format, &item.Count, &item.Size)
		stats = append(stats, item)
	}

	return stats
}

// getSizeDistribution 获取大小分布
func getSizeDistribution(db *gorm.DB) []SizeDistributionItem {
	var distribution []SizeDistributionItem

	// 定义大小范围
	ranges := []struct {
		name string
		min  int64
		max  int64
	}{
		{"< 100KB", 0, 100 * 1024},
		{"100KB - 500KB", 100 * 1024, 500 * 1024},
		{"500KB - 1MB", 500 * 1024, 1024 * 1024},
		{"1MB - 5MB", 1024 * 1024, 5 * 1024 * 1024},
		{"5MB - 10MB", 5 * 1024 * 1024, 10 * 1024 * 1024},
		{"> 10MB", 10 * 1024 * 1024, 0},
	}

	for _, r := range ranges {
		var count int64
		query := db.Model(&models.Image{})

		if r.max == 0 {
			// 最后一个范围，只有最小值
			query = query.Where("file_size >= ?", r.min)
		} else {
			query = query.Where("file_size >= ? AND file_size < ?", r.min, r.max)
		}

		query.Count(&count)

		distribution = append(distribution, SizeDistributionItem{
			Range: r.name,
			Count: count,
		})
	}

	return distribution
}

// GetImageStats 获取图片详细统计
func GetImageStats(c *gin.Context) {
	db := database.GetDB().DB

	// 获取查询参数
	period := c.DefaultQuery("period", "month") // day, week, month, year

	var stats any

	switch period {
	case "day":
		stats = getDailyStats(db)
	case "week":
		stats = getWeeklyStats(db)
	case "month":
		stats = getMonthlyStats(db)
	case "year":
		stats = getYearlyStats(db)
	default:
		stats = getMonthlyStats(db)
	}

	c.JSON(http.StatusOK, StatsResponse{
		Code:    200,
		Message: "获取图片统计成功",
		Success: true,
		Data:    stats,
	})
}

// getDailyStats 获取每日统计
func getDailyStats(db *gorm.DB) []UploadTrendItem {
	var stats []UploadTrendItem

	// 获取最近30天的数据
	for i := 29; i >= 0; i-- {
		date := time.Now().AddDate(0, 0, -i).Format("2006-01-02")

		var count int64
		db.Model(&models.Image{}).Where("DATE(created_at) = ?", date).Count(&count)

		stats = append(stats, UploadTrendItem{
			Date:  date,
			Count: count,
		})
	}

	return stats
}

// getWeeklyStats 获取每周统计
func getWeeklyStats(db *gorm.DB) []UploadTrendItem {
	var stats []UploadTrendItem

	// 获取最近12周的数据
	for i := 11; i >= 0; i-- {
		// 计算周的开始日期
		weekStart := time.Now().AddDate(0, 0, -i*7-int(time.Now().Weekday())+1)
		weekEnd := weekStart.AddDate(0, 0, 6)

		var count int64
		db.Model(&models.Image{}).
			Where("created_at >= ? AND created_at <= ?",
				weekStart.Format("2006-01-02"),
				weekEnd.Format("2006-01-02 23:59:59")).
			Count(&count)

		stats = append(stats, UploadTrendItem{
			Date:  weekStart.Format("2006-01-02"),
			Count: count,
		})
	}

	return stats
}

// getMonthlyStats 获取每月统计
func getMonthlyStats(db *gorm.DB) []UploadTrendItem {
	var stats []UploadTrendItem

	// 获取最近12个月的数据
	for i := 11; i >= 0; i-- {
		date := time.Now().AddDate(0, -i, 0)
		monthStr := date.Format("2006-01")

		var count int64
		db.Model(&models.Image{}).
			Where("strftime('%Y-%m', created_at) = ?", monthStr).
			Count(&count)

		stats = append(stats, UploadTrendItem{
			Date:  monthStr,
			Count: count,
		})
	}

	return stats
}

// getYearlyStats 获取每年统计
func getYearlyStats(db *gorm.DB) []UploadTrendItem {
	var stats []UploadTrendItem

	// 获取最近5年的数据
	for i := 4; i >= 0; i-- {
		year := time.Now().AddDate(-i, 0, 0).Format("2006")

		var count int64
		db.Model(&models.Image{}).
			Where("strftime('%Y', created_at) = ?", year).
			Count(&count)

		stats = append(stats, UploadTrendItem{
			Date:  year,
			Count: count,
		})
	}

	return stats
}
