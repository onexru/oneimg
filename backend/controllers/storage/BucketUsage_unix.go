//go:build !windows

package storage

import "syscall"

// statfs 用底层系统调用取容量，不经过 gopsutil 的推算。
// 仅在非 Windows 平台编译（Linux 容器 / macOS 开发机）。
func statfs(path string) (rawDiskStat, error) {
	var st syscall.Statfs_t
	if err := syscall.Statfs(path, &st); err != nil {
		return rawDiskStat{}, err
	}
	bsize := uint64(st.Bsize)
	return rawDiskStat{
		TotalBytes: uint64(st.Blocks) * bsize,
		// 与 df 同口径：Used 含 root 预留块，Free 取非特权用户可用块
		UsedBytes: (uint64(st.Blocks) - uint64(st.Bavail)) * bsize,
		FreeBytes: uint64(st.Bavail) * bsize,
	}, nil
}
