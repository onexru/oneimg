package controllers

import (
 "net/http"
 "oneimg/backend/middlewares"
 "oneimg/backend/models"
 "github.com/gin-gonic/gin"
)

func CheckLoginStatus(c *gin.Context) {
 u,ok:=middlewares.GetCurrentUser(c)
 if !ok { c.JSON(401,gin.H{"code":401,"message":"未登录"}); return }
 c.JSON(http.StatusOK,gin.H{"code":200,"message":"已登录","data":gin.H{
  "id":u.ID,"username":u.Username,"role":u.Role,"isLoggedIn":true,
 "user_id":u.ID,"user_role":u.Role,"logged_in":true,
  "isSuperAdmin":u.ID==models.SuperAdminID && c.GetString("auth_method")!="api_token",
  "isAdmin":u.Role==models.RoleAdmin,"permissions":u.Permission.Codes,
  "guest_identity_verified":c.GetBool("guest_identity_verified"),
  "auth_method":c.GetString("auth_method"),
 }})
}
