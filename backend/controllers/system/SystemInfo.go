package system

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"oneimg/backend/config"
	"oneimg/backend/middlewares"
	"oneimg/backend/models"
	"oneimg/backend/utils/result"

	"github.com/gin-gonic/gin"
)

// releaseRepo 是检查更新时查询的仓库。
const releaseRepo = "onexru/oneimg"

// releaseCacheTTL 是 GitHub 查询结果的服务端缓存时长。
//
// 未缓存的 GitHub 匿名接口限流是 60 次/小时/IP，而「检查更新」是人工点击的
// 低频操作，几分钟的缓存足以避免刷新页面就重复打接口，也能让无外网环境下
// 的失败结果不会每次重试都卡满超时时间。
const releaseCacheTTL = 5 * time.Minute

// releaseFetchTimeout 是请求 GitHub 的超时。部署在没有外网的环境时，
// 这个值决定「检查更新」按钮最多转多久。
const releaseFetchTimeout = 8 * time.Second

// releaseInfo 是返回给前端的发布信息，只保留界面需要的字段。
type releaseInfo struct {
	TagName     string `json:"tag_name"`
	Name        string `json:"name"`
	PublishedAt string `json:"published_at"`
	HTMLURL     string `json:"html_url"`
	Body        string `json:"body"`
}

type releaseCacheEntry struct {
	info    *releaseInfo
	errText string
	expires time.Time
}

var releaseCache struct {
	mu    sync.Mutex
	entry *releaseCacheEntry
}

// GetSystemVersion 返回当前运行版本，可选地检查是否有新版本。
//
// 查询放在服务端而不是浏览器：既能复用缓存避开 GitHub 的匿名限流，
// 也不会把访客 IP 暴露给 GitHub。无外网时只让「检查更新」失败，
// 当前版本号始终可用。
func GetSystemVersion(c *gin.Context) {
	user, exists := middlewares.GetCurrentUser(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, result.Error(401, "未登录"))
		return
	}
	if !canViewSystemInfo(c, user) {
		c.JSON(http.StatusForbidden, result.Error(403, "无权查看系统信息，需要权限 setting:list"))
		return
	}

	payload := map[string]any{
		"current_version": config.Version,
		"repository":      releaseRepo,
	}

	// check=1 才真的去查，避免每次打开设置页都请求外部接口。
	if c.Query("check") == "1" {
		info, errText := fetchLatestRelease()
		if errText != "" {
			payload["release_error"] = errText
		} else if info != nil {
			payload["latest"] = info
			payload["update_available"] = isNewerVersion(config.Version, info.TagName)
		}
	}

	c.JSON(http.StatusOK, result.Success("ok", payload))
}

// canViewSystemInfo 判断当前身份能否查看系统信息。
// 与「查看设置」保持同一门槛：超管/管理员，或被授予 setting:list。
func canViewSystemInfo(c *gin.Context, user *models.User) bool {
	if c.GetString("auth_method") == "api_token" {
		return user.Permission.HasPermission("setting:list")
	}
	if user.ID == models.SuperAdminID || user.Role == models.RoleAdmin {
		return true
	}
	return user.Permission.HasPermission("setting:list")
}

// fetchLatestRelease 读缓存或请求 GitHub 最新 release。
// 返回 (info, errText)，两者互斥：成功时 errText 为空。
func fetchLatestRelease() (*releaseInfo, string) {
	releaseCache.mu.Lock()
	if e := releaseCache.entry; e != nil && e.expires.After(time.Now()) {
		info, errText := e.info, e.errText
		releaseCache.mu.Unlock()
		return info, errText
	}
	releaseCache.mu.Unlock()

	info, errText := requestLatestRelease()

	releaseCache.mu.Lock()
	releaseCache.entry = &releaseCacheEntry{info: info, errText: errText, expires: time.Now().Add(releaseCacheTTL)}
	releaseCache.mu.Unlock()
	return info, errText
}

// requestLatestRelease 真正发起请求。任何失败都转成一句可展示的中文提示，
// 不把上游的原始错误抛给前端。
func requestLatestRelease() (*releaseInfo, string) {
	client := &http.Client{Timeout: releaseFetchTimeout}
	req, err := http.NewRequest(http.MethodGet, "https://api.github.com/repos/"+releaseRepo+"/releases/latest", nil)
	if err != nil {
		return nil, "检查更新失败"
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "oneimg-version-check")

	resp, err := client.Do(req)
	if err != nil {
		return nil, "无法连接更新服务，请检查网络后重试"
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, "尚未发布任何版本"
	}
	if resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusTooManyRequests {
		// GitHub 匿名限流，等缓存过期后重试即可。
		return nil, "更新检查过于频繁，请稍后重试"
	}
	if resp.StatusCode != http.StatusOK {
		return nil, "检查更新失败，请稍后重试"
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 256<<10))
	if err != nil {
		return nil, "检查更新失败，请稍后重试"
	}

	var raw struct {
		TagName     string `json:"tag_name"`
		Name        string `json:"name"`
		PublishedAt string `json:"published_at"`
		HTMLURL     string `json:"html_url"`
		Body        string `json:"body"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, "检查更新失败，请稍后重试"
	}
	if strings.TrimSpace(raw.TagName) == "" {
		return nil, "尚未发布任何版本"
	}

	// release body 可能很长，界面只做摘要展示，截断避免响应体过大。
	const maxBodyRunes = 4000
	note := []rune(raw.Body)
	if len(note) > maxBodyRunes {
		note = append(note[:maxBodyRunes], []rune("\n\n…（内容过长，已截断）")...)
	}

	return &releaseInfo{
		TagName:     strings.TrimSpace(raw.TagName),
		Name:        raw.Name,
		PublishedAt: raw.PublishedAt,
		HTMLURL:     raw.HTMLURL,
		Body:        string(note),
	}, ""
}

// isNewerVersion 判断 latest 是否比 current 新。
//
// 只做最常见的 "vX.Y.Z" 数字比较：非数字后缀（-beta、-rc1）按预发布处理，
// 版本段缺失按 0 补齐。解析不出数字时保守返回 false —— 宁可提示「已是最新」，
// 也不要对用户虚报有更新。
func isNewerVersion(current, latest string) bool {
	cur, ok1 := parseVersionNumbers(current)
	lat, ok2 := parseVersionNumbers(latest)
	if !ok1 || !ok2 {
		return false
	}
	for i := 0; i < len(cur) || i < len(lat); i++ {
		var a, b int
		if i < len(cur) {
			a = cur[i]
		}
		if i < len(lat) {
			b = lat[i]
		}
		if b != a {
			return b > a
		}
	}
	return false
}

// parseVersionNumbers 把 "v3.5.8" / "3.5.8-rc1" 解析成 []int{3,5,8}。
func parseVersionNumbers(v string) ([]int, bool) {
	v = strings.TrimSpace(v)
	v = strings.TrimPrefix(strings.TrimPrefix(v, "v"), "V")
	// 去掉预发布/构建元数据：3.5.8-rc1+build → 3.5.8
	if i := strings.IndexAny(v, "-+"); i >= 0 {
		v = v[:i]
	}
	if v == "" {
		return nil, false
	}
	parts := strings.Split(v, ".")
	nums := make([]int, 0, len(parts))
	for _, p := range parts {
		if p == "" {
			return nil, false
		}
		n := 0
		for _, r := range p {
			if r < '0' || r > '9' {
				return nil, false
			}
			n = n*10 + int(r-'0')
		}
		nums = append(nums, n)
	}
	return nums, len(nums) > 0
}
