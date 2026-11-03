//go:build windows

package storage

import "golang.org/x/sys/windows"

// statfs 在 Windows 上通过 GetDiskFreeSpaceEx 读取容量。
//
// 与非 Windows 平台保持同一口径：
//   - Total  = 卷总容量
//   - Free   = 调用者（非特权）可用字节
//   - Used   = Total - 卷实际可用（含管理员配额外的已用空间），与 df 的 Used 接近
func statfs(path string) (rawDiskStat, error) {
	ptr, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return rawDiskStat{}, err
	}
	var freeToCaller, total, totalFree uint64
	if err := windows.GetDiskFreeSpaceEx(ptr, &freeToCaller, &total, &totalFree); err != nil {
		return rawDiskStat{}, err
	}
	return rawDiskStat{
		TotalBytes: total,
		UsedBytes:  total - totalFree,
		FreeBytes:  freeToCaller,
	}, nil
}
