package middlewares

import (
	"net/http"
	"strings"

	"oneimg/backend/database"
	"oneimg/backend/models"
	"oneimg/backend/utils/secureconfig"
	"oneimg/backend/utils/settings"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
)

// AuthResponse 认证失败时的统一响应体。
type AuthResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// AuthMiddleware 校验 Session 或 API Token，并将当前用户写入上下文。
// 上下文键：user_id、user_role、username、current_user。
func AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		setting, _ := settings.GetSettings()
		apiToken := ""

		if setting.StartAPI {
			authHeader := c.Request.Header.Get("Authorization")
			parts := strings.SplitN(authHeader, "=", 2)
			if len(parts) == 2 && strings.TrimSpace(parts[0]) == "oneimg_token" {
				apiToken = strings.TrimSpace(parts[1])
			}

			if validateToken(setting, apiToken) {
				// API Token 视为超级管理员（通配权限）
				apiAdminUser := &models.User{
					ID:       models.SuperAdminID,
					Role:     models.RoleAdmin,
					Username: "api_admin",
					Permission: models.Permission{
						Codes:   []string{"*"},
						Buckets: []int{},
					},
				}
				c.Set("user_id", models.SuperAdminID)
				c.Set("user_role", models.RoleAdmin)
				c.Set("username", "api_admin")
				c.Set("current_user", apiAdminUser)
				c.Next()
				return
			}
		}

		session := sessions.Default(c)
		loggedIn := session.Get("logged_in")
		if (loggedIn == nil || loggedIn != true) && apiToken == "" {
			c.JSON(http.StatusUnauthorized, AuthResponse{Code: 401, Message: "用户未登录"})
			c.Abort()
			return
		}

		userID := session.Get("user_id")
		userRole := session.Get("user_role")
		username := session.Get("username")
		if userID == nil || username == nil {
			c.JSON(http.StatusUnauthorized, AuthResponse{Code: 401, Message: "会话信息无效"})
			c.Abort()
			return
		}

		userIDValue, userIDOK := userID.(int)
		userRoleValue, userRoleOK := userRole.(int)
		usernameValue, usernameOK := username.(string)
		if !userIDOK || !userRoleOK || !usernameOK {
			c.JSON(http.StatusUnauthorized, AuthResponse{Code: 401, Message: "会话信息无效"})
			c.Abort()
			return
		}

		var currentUser models.User
		// 游客为虚拟账号；正式用户每次从库加载，使删除/改角色即时生效。
		if userRoleValue != models.RoleGuest {
			db := database.GetDB().DB
			if db == nil || db.Select("id", "role", "username", "permission").First(&currentUser, userIDValue).Error != nil {
				session.Clear()
				_ = session.Save()
				c.JSON(http.StatusUnauthorized, AuthResponse{Code: 401, Message: "用户不存在或已被禁用"})
				c.Abort()
				return
			}
			userRoleValue = currentUser.Role
			usernameValue = currentUser.Username
			session.Set("user_role", userRoleValue)
			session.Set("username", usernameValue)
		} else {
			currentUser = models.User{
				ID:       userIDValue,
				Role:     models.RoleGuest,
				Username: usernameValue,
			}
		}

		session.Set("logged_in", true)
		c.Set("user_id", userIDValue)
		c.Set("user_role", userRoleValue)
		c.Set("username", usernameValue)
		c.Set("current_user", &currentUser)
		c.Next()
	}
}

// validateToken 校验 API Token（优先哈希比对，兼容明文遗留字段）。
func validateToken(setting models.Settings, token string) bool {
	token = strings.TrimSpace(token)
	if token == "" {
		return false
	}
	if secureconfig.CompareSecretHash(setting.APITokenHash, token) {
		return true
	}
	return setting.APIToken != "" && secureconfig.ConstantTimeEqual(setting.APIToken, token)
}

// RequirePermission 要求当前用户具备指定权限码；超级管理员与 "*" 直接放行。
func RequirePermission(requiredCode string) gin.HandlerFunc {
	return func(c *gin.Context) {
		userInterface, exists := c.Get("current_user")
		if !exists {
			c.JSON(http.StatusUnauthorized, AuthResponse{Code: 401, Message: "用户信息获取失败"})
			c.Abort()
			return
		}

		user, ok := userInterface.(*models.User)
		if !ok {
			c.JSON(http.StatusInternalServerError, AuthResponse{Code: 500, Message: "上下文类型错误"})
			c.Abort()
			return
		}
		if user.ID == models.SuperAdminID {
			c.Next()
			return
		}
		for _, code := range user.Permission.Codes {
			if code == "*" {
				c.Next()
				return
			}
		}

		if !user.Permission.HasPermission(requiredCode) {
			permName := models.GetPermissionName(requiredCode)
			c.JSON(http.StatusForbidden, AuthResponse{
				Code:    403,
				Message: "无操作权限，需要权限: [" + permName + "]",
			})
			c.Abort()
			return
		}

		c.Next()
	}
}

// AdminOnlyMiddleware 仅允许管理员角色访问。
func AdminOnlyMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.GetInt("user_role") != models.RoleAdmin {
			c.JSON(http.StatusForbidden, AuthResponse{Code: 403, Message: "无权访问"})
			c.Abort()
			return
		}
		c.Next()
	}
}

// OptionalAuthMiddleware 可选登录：已登录则注入 user_id/username，未登录不拦截。
func OptionalAuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		session := sessions.Default(c)
		loggedIn := session.Get("logged_in")
		if loggedIn != nil && loggedIn == true {
			userID := session.Get("user_id")
			username := session.Get("username")
			if userID != nil && username != nil {
				c.Set("user_id", userID)
				c.Set("username", username)
			}
		}
		c.Next()
	}
}

// GetCurrentUser 从上下文读取当前用户对象。
func GetCurrentUser(c *gin.Context) (*models.User, bool) {
	userInterface, exists := c.Get("current_user")
	if !exists {
		return nil, false
	}
	user, ok := userInterface.(*models.User)
	return user, ok
}
