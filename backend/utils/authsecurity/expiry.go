package authsecurity

import (
 "errors"
 "fmt"
 "strconv"
 "strings"
 "time"
)

// 凭据有效期的可配置边界。上限取 10 年而不是「随便填」：ExpiresAt 是普通
// DATETIME，填一个 999 年的值在部分数据库上会被截断成不合法时间；真正的
// 「永久」走 NeverExpires 标记，不靠极端时间戳来表达。
const (
 MinTokenLifetime = 5 * time.Minute
 MaxTokenLifetime = 3650 * 24 * time.Hour
)

// PermanentExpiresAt 是「永久有效」凭据落库的哨兵时间。它只用于让历史查询
// （expires_at > now、expires_at 索引）继续成立，权限判断一律看 NeverExpires。
var PermanentExpiresAt = time.Date(9999, 12, 31, 23, 59, 59, 0, time.UTC)

// ErrTokenExpiry 表示有效期配置无法解析或超出允许范围。
var ErrTokenExpiry = errors.New("API 凭据有效期无效")

// TokenExpiry 是一次「有效期配置」的解析结果：既包含绝对时间所需的时长，
// 也包含可以原样存回数据库、再回显给用户的规范化字符串。
type TokenExpiry struct {
 Permanent bool
 Duration  time.Duration
 Spec      string
}

// ExpiresAt 把配置换算成绝对过期时间。永久有效时返回哨兵值。
func (e TokenExpiry) ExpiresAt(now time.Time) time.Time {
 if e.Permanent {
  return PermanentExpiresAt
 }
 return now.Add(e.Duration)
}

// ParseTokenExpiry 解析有效期配置：
//
//   - ""            默认 30 天，保持「不传参数」的老调用方行为不变
//   - "permanent" / "never" / "forever"  永久有效
//   - "30d" / "12h" / "90m" / "45s"      带单位的时长
//   - "2592000"                          纯数字按秒
//
// 结果里的 Spec 是规范化后的字符串，避免 "7D"、" 7d " 之类写法在库里出现多份。
func ParseTokenExpiry(spec string) (TokenExpiry, error) {
 trimmed := strings.TrimSpace(spec)
 if trimmed == "" {
  return TokenExpiry{Duration: TokenLifetime, Spec: FormatTokenExpiry(TokenLifetime)}, nil
 }
 switch strings.ToLower(trimmed) {
 case "permanent", "never", "forever", "never_expires":
  return TokenExpiry{Permanent: true, Spec: "permanent"}, nil
 }
 duration, err := parseTokenDuration(strings.ToLower(trimmed))
 if err != nil {
  return TokenExpiry{}, err
 }
 return TokenExpiry{Duration: duration, Spec: FormatTokenExpiry(duration)}, nil
}

func parseTokenDuration(value string) (time.Duration, error) {
 number := value
 unit := time.Duration(0)
 if last := value[len(value)-1]; last < '0' || last > '9' {
  number = value[:len(value)-1]
  switch last {
  case 's':
   unit = time.Second
  case 'm':
   unit = time.Minute
  case 'h':
   unit = time.Hour
  case 'd':
   unit = 24 * time.Hour
  case 'w':
   unit = 7 * 24 * time.Hour
  default:
   return 0, fmt.Errorf("%w: 未知单位 %q", ErrTokenExpiry, string(last))
  }
 } else {
  unit = time.Second
 }
 if number == "" {
  return 0, fmt.Errorf("%w: 缺少数值", ErrTokenExpiry)
 }
 amount, err := strconv.ParseInt(number, 10, 64)
 if err != nil || amount <= 0 {
  return 0, fmt.Errorf("%w: 数值必须为正整数", ErrTokenExpiry)
 }
 if amount > int64(time.Duration(1<<62)/unit) {
  return 0, fmt.Errorf("%w: 时长过大", ErrTokenExpiry)
 }
 duration := time.Duration(amount) * unit
 if duration < MinTokenLifetime {
  return 0, fmt.Errorf("%w: 最短 %s", ErrTokenExpiry, formatDuration(MinTokenLifetime))
 }
 if duration > MaxTokenLifetime {
  return 0, fmt.Errorf("%w: 最长 %s", ErrTokenExpiry, formatDuration(MaxTokenLifetime))
 }
 return duration, nil
}

// FormatTokenExpiry 把时长写成尽量简洁的规范形式（7d 而非 168h）。
func FormatTokenExpiry(duration time.Duration) string {
 switch {
 case duration%(24*time.Hour) == 0:
  return fmt.Sprintf("%dd", int64(duration/(24*time.Hour)))
 case duration%time.Hour == 0:
  return fmt.Sprintf("%dh", int64(duration/time.Hour))
 case duration%time.Minute == 0:
  return fmt.Sprintf("%dm", int64(duration/time.Minute))
 default:
  return fmt.Sprintf("%ds", int64(duration/time.Second))
 }
}

func formatDuration(duration time.Duration) string {
 return FormatTokenExpiry(duration)
}
