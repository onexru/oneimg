package controllers

import (
	"net/http"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
)

// LogoutResponse 退出登录响应结构
type LogoutResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// Logout 用户退出登录
func Logout(c *gin.Context) {
	session := sessions.Default(c)
	session.Clear()
	if err := session.Save(); err != nil {
		c.JSON(http.StatusInternalServerError, LogoutResponse{
			Code:    500,
			Message: "退出登录失败",
		})
		return
	}

	c.JSON(http.StatusOK, LogoutResponse{
		Code:    200,
		Message: "退出登录成功",
	})
}
