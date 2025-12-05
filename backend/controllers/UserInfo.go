package controllers

import (
	"net/http"

	"oneimg/backend/utils/result"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
)

func CheckLoginStatus(c *gin.Context) {
	// 经过了AuthMiddleware，这里一定已经登录了
	session := sessions.Default(c)
	userID := session.Get("user_id")
	username := session.Get("username")

	// 使用统一返回格式
	c.JSON(http.StatusOK, result.Success(
		"已登录",
		map[string]any{
			"user_id":   userID,
			"username":  username,
			"logged_in": true,
		}))
}
