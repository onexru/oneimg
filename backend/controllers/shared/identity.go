package shared

import (
	"regexp"
	"strings"
)

// uuidRegex 匹配标准 UUID 字面量。
// 游客用户名可以是一个 UUID（前端为匿名访客生成）。
var uuidRegex = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

// IsTouristUsername 判断用户名是否为保留的游客身份。
//
// 三种形态：固定名 guest、带前缀的 guest_xxx、以及前端生成的 UUID。
// 这些名字不能用于注册或改用户名 —— 否则真实用户会与匿名访客的身份空间
// 撞车，游客凭据可能借此访问到他人上传的图片。
//
// 放在 shared 包是因为「注册」「改用户名」「管理员创建用户」三个入口
// 都要做同一判断，各写一份必然会走样（漏掉 UUID 那一支就等于留了后门）。
func IsTouristUsername(username string) bool {
	return username == "guest" || strings.HasPrefix(username, "guest_") || uuidRegex.MatchString(username)
}
