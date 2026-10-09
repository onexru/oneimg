package controllers

import (
	"fmt"
	"github.com/shirou/gopsutil/v3/disk"
	"oneimg/backend/config"
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

// keepTwoDecimal 将 float64 四舍五入到两位小数。
func keepTwoDecimal(num float64) float64 {
	return float64(int(num*100+0.5)) / 100
}

// 辅助函数：获取磁盘使用情况
func getDiskUsage() (diskInfo DiskUsageDetail, err error) {
	// Report the actual upload mount, never the container's working directory.
	usage, err := disk.Usage(config.UploadRoot())
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
