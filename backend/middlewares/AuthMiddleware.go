package middlewares

import (
 "net/http"
 "strings"
 "time"
 "oneimg/backend/database"
 "oneimg/backend/models"
 "oneimg/backend/utils/authsecurity"
 "oneimg/backend/utils/settings"
 "github.com/gin-contrib/sessions"
 "github.com/gin-gonic/gin"
 "gorm.io/gorm"
)

type AuthResponse struct { Code int `json:"code"`; Message string `json:"message"` }
func rejectAuth(c *gin.Context,status int,message string) { c.AbortWithStatusJSON(status,AuthResponse{Code:status,Message:message}) }
func setCurrentUser(c *gin.Context,u *models.User) {
 c.Set("user_id",u.ID); c.Set("user_role",u.Role); c.Set("username",u.Username); c.Set("current_user",u)
}

// Explicit Authorization is authoritative, including when API access is off.
// An invalid credential MUST NOT silently fall back to an authenticated cookie.
func AuthMiddleware() gin.HandlerFunc { return authenticate(false) }
func OptionalAuthMiddleware() gin.HandlerFunc { return authenticate(true) }
func authenticate(optional bool) gin.HandlerFunc {
 return authenticateDB(optional,func() *gorm.DB { db:=database.GetDB(); if db==nil{return nil};return db.DB },settings.GetSettings)
}
// Dependencies are injectable for isolated SQLite/httptest regressions without
// mutating the production database singleton or global settings cache.
func authenticateDB(optional bool, getDB func() *gorm.DB, getSettings func()(models.Settings,error)) gin.HandlerFunc {
 return func(c *gin.Context) {
  db:=getDB()
  if db==nil { rejectAuth(c,503,"认证服务暂不可用"); return }
  if headers,present:=c.Request.Header["Authorization"]; present {
   if len(headers)!=1 { rejectAuth(c,401,"API 凭据无效"); return }
   raw,ok:=parseAPIToken(headers[0]); if !ok { rejectAuth(c,401,"API 凭据无效"); return }
   setting,err:=getSettings(); if err!=nil { rejectAuth(c,503,"认证配置暂不可用"); return }
   token,err:=authsecurity.ValidateAPICredential(db,setting,raw)
   if err!=nil { rejectAuth(c,401,"API 凭据无效、已过期或 API 未开启"); return }
   scope:=tokenRouteScope(c.Request.Method,c.Request.URL.Path)
   if scope=="" || !authsecurity.HasScope(token,scope) { rejectAuth(c,403,"API 凭据无此操作权限"); return }
   var user models.User
   if db.First(&user,token.OwnerID).Error!=nil { rejectAuth(c,401,"API 凭据所属用户不存在"); return }
   // Controllers still enforce image ownership/bucket permissions. Tokens never
   // gain the superadmin permission bypass or settings/user/storage endpoints.
   user.Password=""; user.Role=models.RoleUser; user.Permission.Codes=[]string{"image:read","upload:write"}
   setCurrentUser(c,&user); c.Set("auth_method","api_token"); c.Set("api_credential",token)
   c.Next(); return
  }
  session:=sessions.Default(c)
  if session.Get("logged_in")!=true {
   if optional { c.Next(); return }; rejectAuth(c,401,"用户未登录或会话已过期"); return
  }
  id,ok:=session.Get("user_id").(int); if !ok || id==0 { rejectAuth(c,401,"会话信息无效"); return }
  var user models.User
  if id<0 {
   setting,err:=getSettings(); if err!=nil { rejectAuth(c,503,"认证配置暂不可用"); return }
   if !setting.Tourist { rejectAuth(c,401,"游客访问未开启"); return }
   credential,_:=c.Cookie(authsecurity.GuestCookie)
   var guest models.GuestIdentity
   if len(credential)!=43 || db.Where("id = ? AND credential_hash = ? AND expires_at > ?",-id,authsecurity.Digest(credential),time.Now()).First(&guest).Error!=nil {
    rejectAuth(c,401,"游客恢复凭据已失效，请重新进入游客模式"); return
   }
   user=models.User{ID:-guest.ID,Role:models.RoleGuest,Username:guest.OwnerKey}
   c.Set("guest_identity_verified",true)
   // OwnerKey is public. The durable HttpOnly bearer never enters context/JSON.
   c.Set("guest_owner_key",guest.OwnerKey)
  } else {
   if err:=db.First(&user,id).Error; err!=nil { rejectAuth(c,401,"用户不存在或已被禁用"); return }
   version,ok:=session.Get("auth_version").(uint64)
   if !ok || version!=user.AuthVersion { rejectAuth(c,401,"会话已撤销，请重新登录"); return }
  }
  user.Password=""; setCurrentUser(c,&user); c.Set("auth_method","session"); c.Next()
 }
}
func parseAPIToken(header string) (string,bool) {
 // Preserve legacy header syntax and support standard Bearer without accepting
 // duplicate/comma-combined headers or empty/oversized credentials.
 var raw string
 if strings.HasPrefix(header,"oneimg_token=") { raw=strings.TrimPrefix(header,"oneimg_token=")
 } else if strings.HasPrefix(header,"Bearer ") { raw=strings.TrimPrefix(header,"Bearer ") } else { return "",false }
 if raw=="" || len(raw)>256 || strings.ContainsAny(raw," ,\t\r\n") { return "",false }; return raw,true
}
func tokenRouteScope(method,path string) string {
 if method==http.MethodGet {
  switch path { case "/api/user/status","/api/uploadConfig","/api/buckets/list","/api/tags","/api/images":return "image:read" }
  if strings.HasPrefix(path,"/api/images/") && !strings.Contains(strings.TrimPrefix(path,"/api/images/"),"/") { return "image:read" }
 }
 if method==http.MethodPost { switch path { case "/api/upload","/api/upload/images","/api/images/upload","/api/images/url":return "upload:write" } }
 return ""
}
func RequirePermission(required string) gin.HandlerFunc {
 return func(c *gin.Context) {
  u,ok:=GetCurrentUser(c); if !ok { rejectAuth(c,401,"用户信息获取失败"); return }
  if c.GetString("auth_method")=="api_token" {
   if !u.Permission.HasPermission(required) { rejectAuth(c,403,"API 凭据无此操作权限"); return }
  } else if u.ID==models.SuperAdminID || u.Permission.HasPermission("*") { c.Next(); return
  } else if !u.Permission.HasPermission(required) { rejectAuth(c,403,"无操作权限，需要权限: ["+models.GetPermissionName(required)+"]"); return }
  c.Next()
 }
}
func AdminOnlyMiddleware() gin.HandlerFunc {
 return func(c *gin.Context) { if c.GetString("auth_method")=="api_token" || c.GetInt("user_role")!=models.RoleAdmin { rejectAuth(c,403,"无权访问"); return }; c.Next() }
}
func GetCurrentUser(c *gin.Context) (*models.User,bool) { v,ok:=c.Get("current_user"); if !ok { return nil,false }; u,ok:=v.(*models.User); return u,ok && u!=nil }
