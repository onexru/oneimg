package shared

// 严格整数解析：拒绝小数、NaN、无穷与溢出。
//
// 为什么不用 reflect 的 Convert：它会把 12.7 静默截断成 12、
// 把超大数绕回负数，设置项据此写入会得到「看起来合法但完全不对」的值。
// 这里逐类型处理，任何不符合整数的输入都返回错误。

import (
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"
)

// Reject fractions/NaN/overflows rather than reflect's silent numeric truncation.
func StrictSettingInt(value any) (int, error) {
	var text string
	switch v := value.(type) {
	case int:
		return v, nil
	case int64:
		text = strconv.FormatInt(v, 10)
	case json.Number:
		text = v.String()
	case float64:
		if math.IsNaN(v) || math.IsInf(v, 0) || math.Trunc(v) != v {
			return 0, fmt.Errorf("必须是有限整数")
		}
		text = strconv.FormatFloat(v, 'f', -1, 64)
	case string:
		text = strings.TrimSpace(v)
	default:
		return 0, fmt.Errorf("必须是整数")
	}
	n, err := strconv.ParseInt(text, 10, strconv.IntSize)
	return int(n), err
}

// strictSettingInt64 与 StrictSettingInt 同义，但保留 int64 宽度。
// 用于超过 int32 范围的设置项（如以字节计的配额）。
func StrictInt64(value any) (int64, error) {
	var text string
	switch v := value.(type) {
	case int:
		return int64(v), nil
	case int64:
		return v, nil
	case json.Number:
		text = v.String()
	case float64:
		if math.IsNaN(v) || math.IsInf(v, 0) || math.Trunc(v) != v {
			return 0, fmt.Errorf("必须是有限整数")
		}
		text = strconv.FormatFloat(v, 'f', -1, 64)
	case string:
		text = strings.TrimSpace(v)
	default:
		return 0, fmt.Errorf("必须是整数")
	}
	return strconv.ParseInt(text, 10, 64)
}
