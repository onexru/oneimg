package controllers

import (
 "net/http"
 "oneimg/backend/database"
 "oneimg/backend/middlewares"
 "oneimg/backend/models"
 "oneimg/backend/utils/authsecurity"
 "oneimg/backend/utils/result"
 "oneimg/backend/utils/settings"
 "github.com/gin-gonic/gin"
)

// RegenerateAPIToken returns the generated secret exactly once. GET never calls
// this handler; token material cannot be recovered from settings or its digest.
func RegenerateAPIToken(c *gin.Context) {
 user,ok:=middlewares.GetCurrentUser(c)
 if !ok { c.JSON(401,result.Error(401,"未登录")); return }
 if user.ID<=0 || c.GetString("auth_method")=="api_token" || (user.ID!=models.SuperAdminID && !user.Permission.HasPermission("setting:api")) {
  c.JSON(403,result.Error(403,"无权生成 API 凭据")); return
 }
 raw,row,err:=authsecurity.NewAPICredential(database.GetDB().DB,user.ID)
 if err!=nil { c.JSON(500,result.Error(500,"API 凭据生成失败")); return }
 settings.Invalidate()
 c.Header("Cache-Control","private, no-store")
 c.JSON(http.StatusOK,result.Success("API 凭据仅显示一次，请妥善保存",gin.H{"api_token":raw,"expires_at":row.ExpiresAt,"scopes":row.Scopes}))
}

func RenewSession(c *gin.Context) {
 user,ok:=middlewares.GetCurrentUser(c)
 if !ok || c.GetString("auth_method")=="api_token" { c.JSON(401,result.Error(401,"需要有效的登录会话")); return }
 if _,err:=saveUserSession(c,user); err!=nil { c.JSON(401,result.Error(401,"会话已撤销，请重新登录")); return }
 c.JSON(200,result.Success("会话已续期",nil))
}
