package middlewares

import (
 "bytes"
 "encoding/json"
 "io"
 "net"
 "net/http"
 "net/url"
 "strconv"
 "strings"
 "sync"
 "time"
 "oneimg/backend/config"
 "oneimg/backend/utils/authsecurity"
 "github.com/gin-gonic/gin"
)

// Never trust arbitrary X-Forwarded-For. The reverse proxy must strip and
// restore forwarding headers before it is explicitly configured as trusted.
func TrustedClientIP(r *http.Request) string {
 host,_,err:=net.SplitHostPort(r.RemoteAddr); if err!=nil { return r.RemoteAddr }; return host
}
type rateEntry struct { Count int; Reset time.Time }
type AuthLimiter struct { mu sync.Mutex; entries map[string]rateEntry; now func() time.Time; MaxEntries int }
func NewAuthLimiter() *AuthLimiter { return &AuthLimiter{entries:make(map[string]rateEntry),now:time.Now,MaxEntries:8192} }
func (l *AuthLimiter) allow(key string,limit int,window time.Duration) (bool,int) {
 l.mu.Lock(); defer l.mu.Unlock(); now:=l.now()
 entry,exists:=l.entries[key]
 if !exists || !now.Before(entry.Reset) {
  if len(l.entries)>=l.MaxEntries {
   for k,v:=range l.entries { if !now.Before(v.Reset) { delete(l.entries,k) } }
  }
  if _,exists=l.entries[key]; !exists && len(l.entries)>=l.MaxEntries { return false,60 }
  entry=rateEntry{Reset:now.Add(window)}
 }
 seconds:=int(entry.Reset.Sub(now).Seconds())+1
 if entry.Count>=limit { return false,seconds }
 entry.Count++; l.entries[key]=entry; return true,0
}
func limitedAuthPath(path string) bool {
 return path=="/api/login" || path=="/api/register" || strings.HasPrefix(path,"/api/auth/") || strings.HasPrefix(path,"/api/verify/")
}
func (l *AuthLimiter) Middleware() gin.HandlerFunc {
 return func(c *gin.Context) {
  if !limitedAuthPath(c.Request.URL.Path) { c.Next(); return }
  c.Header("Cache-Control","private, no-store")
  ip:=c.ClientIP()
  // One shared IP budget prevents rotating across login/SSO/PoW endpoints.
  if ok,wait:=l.allow("ip:"+ip,30,time.Minute); !ok { rateLimited(c,wait); return }
  if c.Request.Method==http.MethodPost {
   c.Request.Body=http.MaxBytesReader(c.Writer,c.Request.Body,32<<10)
   data,err:=io.ReadAll(c.Request.Body)
   if err!=nil { c.AbortWithStatusJSON(413,AuthResponse{Code:413,Message:"请求内容过大"}); return }
   c.Request.Body=io.NopCloser(bytes.NewReader(data))
   if c.Request.URL.Path=="/api/login" || c.Request.URL.Path=="/api/register" {
    var body struct { Username string `json:"username"` }
    if json.Unmarshal(data,&body)==nil && body.Username!="" {
     // Hash account names to bound key length; include an IP+account budget
     // and a global account budget to limit distributed online guessing.
     account:=authsecurity.Digest(strings.ToLower(strings.TrimSpace(body.Username)))
     if ok,wait:=l.allow("pair:"+ip+":"+account,5,time.Minute); !ok { rateLimited(c,wait); return }
     if ok,wait:=l.allow("account:"+account,20,5*time.Minute); !ok { rateLimited(c,wait); return }
    }
   }
  }
  c.Next()
 }
}
func rateLimited(c *gin.Context,seconds int) {
 c.Header("Retry-After",strconv.Itoa(seconds)); c.AbortWithStatusJSON(429,gin.H{"code":429,"message":"尝试过于频繁，请稍后再试","retry_after":seconds})
}

// Origin checks complement HttpOnly/SameSite cookies. Non-browser API clients
// may omit Origin; requests with an explicit foreign Origin are always denied.
func SameOriginWrites(cfg *config.Config) gin.HandlerFunc {
 return func(c *gin.Context) {
  if !strings.HasPrefix(c.Request.URL.Path,"/api/") || c.Request.Method==http.MethodGet || c.Request.Method==http.MethodHead || c.Request.Method==http.MethodOptions { c.Next(); return }
  if c.GetHeader("Sec-Fetch-Site")=="cross-site" { rejectAuth(c,403,"不允许跨站请求"); return }
  origin:=c.GetHeader("Origin")
  if origin!="" && !sameOrigin(origin,cfg.AppURL) { rejectAuth(c,403,"请求来源无效"); return }
  c.Next()
 }
}
func sameOrigin(a,b string) bool {
 x,err:=url.Parse(a); if err!=nil { return false }; y,err:=url.Parse(b); if err!=nil { return false }
 return x.Scheme!="" && x.User==nil && x.RawQuery=="" && x.Fragment=="" && (x.Path=="" || x.Path=="/") && strings.EqualFold(x.Scheme,y.Scheme) && strings.EqualFold(x.Host,y.Host)
}
