package middlewares

import (
	"net/http"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
)

// AuthResponse 认证失败响应结构
type AuthResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// AuthMiddleware Session认证中间件
func AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 获取session
		session := sessions.Default(c)

		// 检查是否已登录
		loggedIn := session.Get("logged_in")
		if loggedIn == nil || loggedIn != true {
			c.JSON(http.StatusUnauthorized, AuthResponse{
				Code:    401,
				Message: "用户未登录",
			})
			c.Abort()
			return
		}

		// 获取用户信息
		userID := session.Get("user_id")
		username := session.Get("username")

		if userID == nil || username == nil {
			c.JSON(http.StatusUnauthorized, AuthResponse{
				Code:    401,
				Message: "会话信息无效",
			})
			c.Abort()
			return
		}

		// 将用户信息存储到上下文中，供后续处理使用
		session.Set("logged_in", true)

		c.Set("user_id", userID)
		c.Set("username", username)

		// 继续处理请求
		c.Next()
	}
}

func AdminOnlyMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 获取用户ID
		userID := c.GetInt("user_id")

		if userID != 1 {
			c.JSON(http.StatusForbidden, AuthResponse{
				Code:    403,
				Message: "无权访问",
			})
			c.Abort()
			return
		}

		// 继续处理请求
		c.Next()
	}
}

// OptionalAuthMiddleware 可选认证中间件（不强制要求认证）
func OptionalAuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 获取session
		session := sessions.Default(c)

		// 检查是否已登录
		loggedIn := session.Get("logged_in")
		if loggedIn != nil && loggedIn == true {
			// 获取用户信息
			userID := session.Get("user_id")
			username := session.Get("username")

			if userID != nil && username != nil {
				// 将用户信息存储到上下文中
				c.Set("user_id", userID)
				c.Set("username", username)
			}
		}

		// 继续处理请求（无论是否登录）
		c.Next()
	}
}

// GetCurrentUser 从上下文中获取当前用户信息
func GetCurrentUser(c *gin.Context) (userID int, username string, exists bool) {
	userIDInterface, userIDExists := c.Get("user_id")
	usernameInterface, usernameExists := c.Get("username")

	if !userIDExists || !usernameExists {
		return 0, "", false
	}

	userID, ok1 := userIDInterface.(int)
	username, ok2 := usernameInterface.(string)

	if !ok1 || !ok2 {
		return 0, "", false
	}

	return userID, username, true
}
