package controllers

import (
	"net/http"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
)

// UserInfoResponse 用户信息响应结构
type UserInfoResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

func CheckLoginStatus(c *gin.Context) {
	// 经过了OptionalAuthMiddleware，这里一定已经登录了
	session := sessions.Default(c)
	userID := session.Get("user_id")
	c.JSON(http.StatusOK, UserInfoResponse{
		Code:    200,
		Message: "已登录",
		Data: map[string]any{
			"user_id":   userID,
			"logged_in": true,
		},
	})
}
