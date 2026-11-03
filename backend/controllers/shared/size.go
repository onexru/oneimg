package shared

// 字节数格式化：给终端用户看的容量表达。
//
// 与前端 utils/byteSize.js 的 formatBytes 保持同一口径（1024 进制、一位小数），
// 否则后端的「已用 262.3 KB」和前端卡片上的数字会对不上。

import "strconv"

// humanBytes 把字节数转成便于阅读的二进制单位（与前端展示口径一致）。
func HumanBytes(n int64) string {
	const unit = 1024
	if n < unit {
		return strconv.FormatInt(n, 10) + " B"
	}
	units := []string{"KB", "MB", "GB", "TB", "PB"}
	value := float64(n)
	i := -1
	for value >= unit && i < len(units)-1 {
		value /= unit
		i++
	}
	// 保留一位小数：10.0 GB 比 10 GB 更能让人意识到这是精确额度。
	return strconv.FormatFloat(value, 'f', 1, 64) + " " + units[i]
}
