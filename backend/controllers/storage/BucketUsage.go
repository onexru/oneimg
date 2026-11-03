package storage

import (
	"fmt"
	"os"
	"path/filepath"

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

// resolveUsagePath 返回用于统计容量的绝对路径。
//
// statfs 是按「路径所在挂载点」统计的，相对路径依赖进程 cwd，cwd 未必是挂载点本身，
// 因此统一转绝对值。上传目录可能尚未创建（首次启动 / 刚切存储），向上找存在的父目录，
// 否则 statfs 直接失败、界面显示「获取失败」。
func resolveUsagePath(raw string) string {
	path := raw
	if path == "" {
		path = config.UploadRoot()
	}
	if abs, err := filepath.Abs(path); err == nil {
		path = abs
	}
	for {
		if _, err := os.Stat(path); err == nil {
			return path
		}
		parent := filepath.Dir(path)
		if parent == path {
			return path
		}
		path = parent
	}
}

// rawDiskStat 是平台无关的容量快照，由各平台的 statfs 填充。
type rawDiskStat struct {
	TotalBytes uint64
	UsedBytes  uint64
	FreeBytes  uint64
}

// getDiskUsage 统计上传目录所在磁盘的容量。
//
// 不用 disk.Usage：gopsutil v3.20.10 在 Docker bind mount 上会把 Used 算成 0
// （它是用 Total-Free-Used 反推，挂载点边界处失效），表现为「磁盘已用 0 B、使用率 0%」，
// 但 Free 又是对的。直接读 statfs 的 blocks/bavail 自己算，与 df 同口径：
//
//	total = blocks * bsize
//	used  = (blocks - bavail) * bsize   // 含 root 预留块，等于 df 的 Used
//	free  = bavail * bsize              // 非特权用户可用，等于 df 的 Avail
//
// used + free 不一定等于 total（差值是被 root 预留的块），这是 df 的既有口径。
func getDiskUsage() (diskInfo DiskUsageDetail, err error) {
	return getDiskUsageFor(resolveUsagePath(""))
}

// getDiskUsageFor 统计指定路径所在磁盘的容量；抽出来便于测试指定目录。
func getDiskUsageFor(path string) (diskInfo DiskUsageDetail, err error) {
	stat, err := statfs(path)
	if err != nil {
		return DiskUsageDetail{}, err
	}
	return newDiskUsageDetail(stat.TotalBytes, stat.UsedBytes, stat.FreeBytes), nil
}

func newDiskUsageDetail(total, used, free uint64) DiskUsageDetail {
	percent := 0.0
	if total > 0 {
		percent = float64(used) / float64(total) * 100
	}
	return DiskUsageDetail{
		TotalBytes: total,
		UsedBytes:  used,
		FreeBytes:  free,
		Total:      formatSize(total),
		Used:       formatSize(used),
		Free:       formatSize(free),
		Percent:    percent,
	}
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
