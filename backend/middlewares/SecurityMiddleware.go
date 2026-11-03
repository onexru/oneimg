package middlewares

import (
	"bytes"
	"encoding/json"
	"github.com/gin-gonic/gin"
	"io"
	"net"
	"net/http"
	"net/url"
	"oneimg/backend/config"
	"oneimg/backend/utils/authsecurity"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Never trust arbitrary X-Forwarded-For. The reverse proxy must strip and
// restore forwarding headers before it is explicitly configured as trusted.
func TrustedClientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// 进程内共享的登录限流器实例。控制器需要在「登录成功」时清零该账号的失败预算、
// 在「密码错误」时单独记账，因此中间件与控制器必须操作同一个对象。
var defaultAuthLimiter = NewAuthLimiter()

// DefaultAuthLimiter 返回全局登录限流器，供控制器回写成功/失败状态。
func DefaultAuthLimiter() *AuthLimiter { return defaultAuthLimiter }

// Reset 清掉某个键的计数（用于登录成功后解除该账号的失败预算）。
func (l *AuthLimiter) Reset(key string) {
	l.mu.Lock()
	delete(l.entries, key)
	l.mu.Unlock()
}

// AccountLoginKey 生成账号维度的限流键。
// 用摘要而非明文：键长度有上界，也不会在内存里堆积用户名。
func AccountLoginKey(username string) string {
	return "acct-login:" + authsecurity.Digest(strings.ToLower(strings.TrimSpace(username)))
}

// NoteLoginFailure 为某账号的失败预算记一次。
// 只在密码确实错误时调用 —— 这样「正确的用户名 + 随便的密码」由 IP 维度兜底，
// 而合法用户即使被别人针对，也不会因为对方一直在打而登不进来。
func (l *AuthLimiter) NoteLoginFailure(username string) {
	l.allow(AccountLoginKey(username), accountFailureLimit, accountFailureWindow)
}

// CheckAccountFailureBudget 只读检查该账号的失败预算是否已耗尽，不自增。
// 与 allow 的区别很关键：预检若也自增，等于把「尝试」当成「失败」记账，
// 正常用户的每次成功登录都会消耗自己的预算。
func (l *AuthLimiter) CheckAccountFailureBudget(username string) (bool, int) {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now()
	entry, exists := l.entries[AccountLoginKey(username)]
	if !exists || !now.Before(entry.Reset) {
		return true, 0
	}
	if entry.Count >= accountFailureLimit {
		return false, int(entry.Reset.Sub(now).Seconds()) + 1
	}
	return true, 0
}

// ClearLoginFailures 登录成功后清空该账号的失败预算。
func (l *AuthLimiter) ClearLoginFailures(username string) {
	l.Reset(AccountLoginKey(username))
	l.Reset("account:" + authsecurity.Digest(strings.ToLower(strings.TrimSpace(username))))
}

// 账号维度失败预算：比原来的 20 次/5 分钟更宽松，因为它现在只统计「密码错误」，
// 不再被正确请求与注册请求白白消耗。
const (
	accountFailureLimit  = 20
	accountFailureWindow = 5 * time.Minute
)

type rateEntry struct {
	Count int
	Reset time.Time
}
type AuthLimiter struct {
	mu         sync.Mutex
	entries    map[string]rateEntry
	now        func() time.Time
	MaxEntries int
}

func NewAuthLimiter() *AuthLimiter {
	return &AuthLimiter{entries: make(map[string]rateEntry), now: time.Now, MaxEntries: 8192}
}
func (l *AuthLimiter) allow(key string, limit int, window time.Duration) (bool, int) {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now()
	entry, exists := l.entries[key]
	if !exists || !now.Before(entry.Reset) {
		if len(l.entries) >= l.MaxEntries {
			for k, v := range l.entries {
				if !now.Before(v.Reset) {
					delete(l.entries, k)
				}
			}
		}
		if _, exists = l.entries[key]; !exists && len(l.entries) >= l.MaxEntries {
			return false, 60
		}
		entry = rateEntry{Reset: now.Add(window)}
	}
	seconds := int(entry.Reset.Sub(now).Seconds()) + 1
	if entry.Count >= limit {
		return false, seconds
	}
	entry.Count++
	l.entries[key] = entry
	return true, 0
}
func limitedAuthPath(path string) bool {
	return path == "/api/login" || path == "/api/register" || strings.HasPrefix(path, "/api/auth/") || strings.HasPrefix(path, "/api/verify/")
}
func (l *AuthLimiter) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !limitedAuthPath(c.Request.URL.Path) {
			// /api/account/change 的"当前密码"确认可被会话劫持者当作密码
			// 预言机无限尝试：单独限流（不与登录共享 IP 预算，避免误伤登录流量）。
			if c.Request.URL.Path == "/api/account/change" && c.Request.Method == http.MethodPost {
				c.Header("Cache-Control", "private, no-store")
				ip := c.ClientIP()
				if ok, wait := l.allow("acct:"+ip, 10, time.Minute); !ok {
					rateLimited(c, wait)
					return
				}
			}
			c.Next()
			return
		}
		c.Header("Cache-Control", "private, no-store")
		ip := c.ClientIP()
		// One shared IP budget prevents rotating across login/SSO/PoW endpoints.
		if ok, wait := l.allow("ip:"+ip, 30, time.Minute); !ok {
			rateLimited(c, wait)
			return
		}
		if c.Request.Method == http.MethodPost {
			c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 32<<10)
			data, err := io.ReadAll(c.Request.Body)
			if err != nil {
				c.AbortWithStatusJSON(413, AuthResponse{Code: 413, Message: "请求内容过大"})
				return
			}
			c.Request.Body = io.NopCloser(bytes.NewReader(data))
			if c.Request.URL.Path == "/api/login" || c.Request.URL.Path == "/api/register" {
				var body struct {
					Username string `json:"username"`
				}
				if json.Unmarshal(data, &body) == nil && body.Username != "" {
					// 每对 (IP, 账号) 一个预算，限制单机对同一账号的在线猜解。
					account := authsecurity.Digest(strings.ToLower(strings.TrimSpace(body.Username)))
					if ok, wait := l.allow("pair:"+ip+":"+account, 5, time.Minute); !ok {
						rateLimited(c, wait)
						return
					}

					// 全局账号预算**只在登录、且只按失败记**，不再在这里无脑扣费。
					//
					// 原实现对所有 /api/login 与 /api/register 请求都在 handler 之前扣
					// "account:"+hash 这一桶（20 次/5 分钟）。问题是它按用户名全局计数、
					// 与 IP 无关，于是单个 IP 以 5 次/分钟就能在 4 分钟内耗尽**任意账号**
					// （含唯一管理员）的预算，之后任何 IP 登录该账号都是 429，攻击者只要
					// 持续低速请求就能永久锁死该账号。注册与登录还共用同一个键，
					// 注册关闭时同样能被用来消耗管理员预算；游客固定用户名 guest 也可被锁死。
					//
					// 现在：注册完全不碰登录预算；登录只在密码错误时由控制器记账，
					// 成功则清零（见 AuthLimiter.NoteLoginFailure / ClearLoginFailures）。
					if c.Request.URL.Path == "/api/login" {
						if ok, wait := l.CheckAccountFailureBudget(body.Username); !ok {
							rateLimited(c, wait)
							return
						}
					}
				}
			}
		}
		c.Next()
	}
}
func rateLimited(c *gin.Context, seconds int) {
	c.Header("Retry-After", strconv.Itoa(seconds))
	c.AbortWithStatusJSON(429, gin.H{"code": 429, "message": "尝试过于频繁，请稍后再试", "retry_after": seconds})
}

// Origin checks complement HttpOnly/SameSite cookies. Non-browser API clients
// may omit Origin; requests with an explicit foreign Origin are always denied.
func SameOriginWrites(cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !strings.HasPrefix(c.Request.URL.Path, "/api/") || c.Request.Method == http.MethodGet || c.Request.Method == http.MethodHead || c.Request.Method == http.MethodOptions {
			c.Next()
			return
		}
		if c.GetHeader("Sec-Fetch-Site") == "cross-site" {
			rejectAuth(c, 403, "不允许跨站请求")
			return
		}
		origin := c.GetHeader("Origin")
		if origin != "" && !sameOrigin(origin, cfg.AppURL) {
			rejectAuth(c, 403, "请求来源无效")
			return
		}
		c.Next()
	}
}
func sameOrigin(a, b string) bool {
	x, err := url.Parse(a)
	if err != nil {
		return false
	}
	y, err := url.Parse(b)
	if err != nil {
		return false
	}
	return x.Scheme != "" && x.User == nil && x.RawQuery == "" && x.Fragment == "" && (x.Path == "" || x.Path == "/") && strings.EqualFold(x.Scheme, y.Scheme) && strings.EqualFold(x.Host, y.Host)
}
