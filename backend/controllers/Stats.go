package controllers

import (
	"fmt"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"net/http"
	"oneimg/backend/database"
	"oneimg/backend/models"
	"oneimg/backend/utils/settings"
	"strings"
	"time"
)

type StatsResponse struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
}
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
type UploadTrendItem struct {
	Date  string `json:"date"`
	Count int64  `json:"count"`
}
type FormatStatsItem struct {
	Format string `json:"format"`
	Count  int64  `json:"count"`
	Size   int64  `json:"size"`
}
type SizeDistributionItem struct {
	Range string `json:"range"`
	Count int64  `json:"count"`
}

// Only trusted server-assigned IDs establish ownership, including negative
// persistent guest IDs. A public username/UUID is never an ownership proof.
func scopeStatsImages(db *gorm.DB, roleID, userID int, _ string) *gorm.DB {
	q := db.Model(&models.Image{})
	if roleID == models.RoleAdmin && userID > 0 {
		return q
	}
	if (roleID == models.RoleUser && userID > 0) || (roleID == models.RoleGuest && userID != 0) {
		return q.Where("user_id = ?", userID)
	}
	return q.Where("1 = 0")
}

var statsSizeRanges = []struct {
	name     string
	min, max int64
}{
	{"< 100KB", 0, 100 * 1024}, {"100KB - 500KB", 100 * 1024, 500 * 1024}, {"500KB - 1MB", 500 * 1024, 1024 * 1024},
	{"1MB - 5MB", 1024 * 1024, 5 * 1024 * 1024}, {"5MB - 10MB", 5 * 1024 * 1024, 10 * 1024 * 1024}, {"> 10MB", 10 * 1024 * 1024, 0},
}

func dayStart(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
}

// Dashboard summary plus six size bins in one portable conditional aggregation.
func loadDashboardStats(db *gorm.DB, role, userID int, now time.Time) (DashboardStats, error) {
	stats := DashboardStats{RecentImages: []models.Image{}, FormatStats: []FormatStatsItem{}}
	today := dayStart(now)
	month := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	selectSQL := "COUNT(*) AS total_images, COALESCE(SUM(file_size),0) AS total_size, COALESCE(SUM(CASE WHEN created_at >= ? AND created_at < ? THEN 1 ELSE 0 END),0) AS today_uploads, COALESCE(SUM(CASE WHEN created_at >= ? AND created_at < ? THEN 1 ELSE 0 END),0) AS month_uploads"
	args := []interface{}{today, today.AddDate(0, 0, 1), month, month.AddDate(0, 1, 0)}
	for i, r := range statsSizeRanges {
		expr := "file_size >= ?"
		args = append(args, r.min)
		if r.max > 0 {
			expr += " AND file_size < ?"
			args = append(args, r.max)
		}
		selectSQL += fmt.Sprintf(", COALESCE(SUM(CASE WHEN %s THEN 1 ELSE 0 END),0) AS bin_%d", expr, i)
	}
	var summary struct {
		TotalImages, TotalSize, TodayUploads, MonthUploads int64
		Bin0                                               int64 `gorm:"column:bin_0"`
		Bin1                                               int64 `gorm:"column:bin_1"`
		Bin2                                               int64 `gorm:"column:bin_2"`
		Bin3                                               int64 `gorm:"column:bin_3"`
		Bin4                                               int64 `gorm:"column:bin_4"`
		Bin5                                               int64 `gorm:"column:bin_5"`
	}
	if err := scopeStatsImages(db, role, userID, "").Select(selectSQL, args...).Scan(&summary).Error; err != nil {
		return stats, err
	}
	stats.TotalImages, stats.TotalSize, stats.TodayUploads, stats.MonthUploads = summary.TotalImages, summary.TotalSize, summary.TodayUploads, summary.MonthUploads
	bins := []int64{summary.Bin0, summary.Bin1, summary.Bin2, summary.Bin3, summary.Bin4, summary.Bin5}
	for i, r := range statsSizeRanges {
		stats.SizeDistribution = append(stats.SizeDistribution, SizeDistributionItem{Range: r.name, Count: bins[i]})
	}
	if err := scopeStatsImages(db, role, userID, "").Order("created_at DESC").Order("id DESC").Limit(10).Find(&stats.RecentImages).Error; err != nil {
		return stats, err
	}
	trend, err := loadPeriodStats(db, "dashboard", role, userID, now)
	if err != nil {
		return stats, err
	}
	stats.UploadTrend = trend
	if err := scopeStatsImages(db, role, userID, "").Select("COALESCE(mime_type,'') AS format, COUNT(*) AS count, COALESCE(SUM(file_size),0) AS size").Group("mime_type").Order("mime_type ASC").Scan(&stats.FormatStats).Error; err != nil {
		return stats, err
	}
	return stats, nil
}

func statsError(c *gin.Context, message string) {
	c.JSON(http.StatusInternalServerError, StatsResponse{Code: 500, Message: message, Success: false})
}
func GetDashboardStats(c *gin.Context) {
	db := database.GetDB()
	if db == nil || db.DB == nil {
		statsError(c, "数据库不可用")
		return
	}
	stats, err := loadDashboardStats(db.DB.WithContext(c.Request.Context()), c.GetInt("user_role"), c.GetInt("user_id"), time.Now())
	if err != nil {
		statsError(c, "获取统计数据失败")
		return
	}
	setting, err := settings.GetSettings()
	if err != nil {
		statsError(c, "获取系统配置失败")
		return
	}
	for i := range stats.RecentImages {
		rewriteImageURLs(setting, &stats.RecentImages[i])
	}
	c.JSON(http.StatusOK, StatsResponse{Code: 200, Message: "获取统计数据成功", Success: true, Data: stats})
}

type statsPeriod struct {
	label      string
	start, end time.Time
}

func statsPeriods(period string, now time.Time) []statsPeriod {
	periods := []statsPeriod{}
	switch period {
	case "day", "dashboard":
		days := 30
		if period == "dashboard" {
			days = 7
		}
		today := dayStart(now)
		for i := days - 1; i >= 0; i-- {
			start := today.AddDate(0, 0, -i)
			periods = append(periods, statsPeriod{start.Format("2006-01-02"), start, start.AddDate(0, 0, 1)})
		}
	case "week":
		// Monday start; Sunday belongs to the preceding Monday, not the next week.
		monday := dayStart(now).AddDate(0, 0, -(int(now.Weekday())+6)%7)
		for i := 11; i >= 0; i-- {
			start := monday.AddDate(0, 0, -i*7)
			periods = append(periods, statsPeriod{start.Format("2006-01-02"), start, start.AddDate(0, 0, 7)})
		}
	case "year":
		current := time.Date(now.Year(), 1, 1, 0, 0, 0, 0, now.Location())
		for i := 4; i >= 0; i-- {
			start := current.AddDate(-i, 0, 0)
			periods = append(periods, statsPeriod{start.Format("2006"), start, start.AddDate(1, 0, 0)})
		}
	default:
		current := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
		for i := 11; i >= 0; i-- {
			start := current.AddDate(0, -i, 0)
			periods = append(periods, statsPeriod{start.Format("2006-01"), start, start.AddDate(0, 1, 0)})
		}
	}
	return periods
}

// One bounded query for 7/30 days, 12 weeks/months or 5 years. Range predicates
// preserve timestamp index use; no DATE(column) casts or query-per-period loops.
func loadPeriodStats(db *gorm.DB, period string, role, userID int, now time.Time) ([]UploadTrendItem, error) {
	periods := statsPeriods(period, now)
	items := make([]UploadTrendItem, len(periods))
	expressions := []string{}
	args := []interface{}{}
	for i, p := range periods {
		items[i].Date = p.label
		expressions = append(expressions, fmt.Sprintf("COALESCE(SUM(CASE WHEN created_at >= ? AND created_at < ? THEN 1 ELSE 0 END),0) AS count_%d", i))
		args = append(args, p.start, p.end)
	}
	rows, err := scopeStatsImages(db, role, userID, "").Where("created_at >= ? AND created_at < ?", periods[0].start, periods[len(periods)-1].end).Select(strings.Join(expressions, ", "), args...).Rows()
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	if rows.Next() {
		dest := make([]interface{}, len(items))
		for i := range items {
			dest[i] = &items[i].Count
		}
		if err := rows.Scan(dest...); err != nil {
			return nil, err
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}
func GetImageStats(c *gin.Context) {
	db := database.GetDB()
	if db == nil || db.DB == nil {
		statsError(c, "数据库不可用")
		return
	}
	stats, err := loadPeriodStats(db.DB.WithContext(c.Request.Context()), c.DefaultQuery("period", "month"), c.GetInt("user_role"), c.GetInt("user_id"), time.Now())
	if err != nil {
		statsError(c, "获取图片统计失败")
		return
	}
	c.JSON(http.StatusOK, StatsResponse{Code: 200, Message: "获取图片统计成功", Success: true, Data: stats})
}
