package controllers

import (
	"net/http"

	"oneimg/backend/utils/result"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
)

// CheckLoginStatus 返回当前登录用户的基本会话信息。
// 前置：AuthMiddleware。
func CheckLoginStatus(c *gin.Context) {
	session := sessions.Default(c)
	userID := session.Get("user_id")
	username := session.Get("username")
	role := c.GetInt("user_role")

	c.JSON(http.StatusOK, result.Success("已登录", map[string]any{
		"user_id":   userID,
		"username":  username,
		"logged_in": true,
		"user_role": role,
	}))
}
