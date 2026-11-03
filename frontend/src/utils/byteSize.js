// 容量格式化：统一「字节 → 人类可读」的口径。
//
// 与后端 humanBytes 保持一致（1024 进制、一位小数），这样配额提示里
// 后端给的数字和前端展示不会出现 10.0 GB / 10 GB 这类细微差异。
const UNITS = ['B', 'KB', 'MB', 'GB', 'TB', 'PB']

export function formatBytes(bytes, fractionDigits = 1) {
  const n = Number(bytes)
  if (!Number.isFinite(n) || n <= 0) return '0 B'
  let value = n
  let index = 0
  while (value >= 1024 && index < UNITS.length - 1) {
    value /= 1024
    index++
  }
  // B 级不带小数：512.0 B 看着比 512 B 啰嗦且没有意义。
  return index === 0
    ? `${Math.round(value)} B`
    : `${value.toFixed(fractionDigits)} ${UNITS[index]}`
}

// 配额输入框用 GB 作为单位（与存储桶容量一致）。
// 0 是特殊值：表示不限，不参与换算。
export function bytesToGb(bytes, fractionDigits = 2) {
  const n = Number(bytes)
  if (!Number.isFinite(n) || n <= 0) return 0
  return Number((n / 1024 / 1024 / 1024).toFixed(fractionDigits))
}

export function gbToBytes(gb) {
  const n = Number(gb)
  if (!Number.isFinite(n) || n <= 0) return 0
  return Math.round(n * 1024 * 1024 * 1024)
}

// 使用率百分比，裁剪到 0-100，避免超额时进度条溢出。
export function usagePercent(used, quota) {
  const u = Number(used) || 0
  const q = Number(quota) || 0
  if (q <= 0) return 0
  return Math.min(100, Math.max(0, (u / q) * 100))
}
