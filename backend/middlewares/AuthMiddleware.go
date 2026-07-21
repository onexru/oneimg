package middlewares

import (
	"net/http"
	"oneimg/backend/database"
	"oneimg/backend/models"
	"oneimg/backend/utils/secureconfig"
	"oneimg/backend/utils/settings"
	"strings"

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
		setting, _ := settings.GetSettings()
		apiToken := ""

		if setting.StartAPI {
			authHeader := c.Request.Header.Get("Authorization")
			parts := strings.SplitN(authHeader, "=", 2)
			if len(parts) == 2 && strings.TrimSpace(parts[0]) == "oneimg_token" {
				apiToken = strings.TrimSpace(parts[1])
			}

			if validateToken(setting, apiToken) {
				// API Token 验证通过，注入一个拥有全部权限的虚拟超级管理员对象
				apiAdminUser := &models.User{
					ID:       1,
					Role:     models.RoleAdmin,
					Username: "api_admin",
					Permission: models.Permission{
						// 给个通配符，或者你可以在这里把 AllPermissionMap 的所有 key 塞进去
						Codes:   []string{"*"},
						Buckets: []int{},
					},
				}
				c.Set("user_id", 1)
				c.Set("user_role", 1)
				c.Set("username", "api_admin")
				c.Set("current_user", apiAdminUser) // 【新增】存入完整对象
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
		// 游客是虚拟账号；其他会话每次核对用户，使删除/角色变更立即生效。
		if userRoleValue != models.RoleGuest {
			db := database.GetDB().DB
			// 【关键修改】这里必须查出 permission 字段
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
			// 如果是游客，构造一个空权限的游客对象
			currentUser = models.User{ID: userIDValue, Role: models.RoleGuest, Username: usernameValue}
		}

		session.Set("logged_in", true)
		c.Set("user_id", userIDValue)
		c.Set("user_role", userRoleValue)
		c.Set("username", usernameValue)
		c.Set("current_user", &currentUser)

		c.Next()
	}
}

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

// 细粒度权限校验中间件
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

func AdminOnlyMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		userRole := c.GetInt("user_role")
		if userRole != 1 {
			c.JSON(http.StatusForbidden, AuthResponse{Code: 403, Message: "无权访问"})
			c.Abort()
			return
		}
		c.Next()
	}
}

// OptionalAuthMiddleware 可选认证中间件
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

// 从上下文中获取当前用户完整信息
func GetCurrentUser(c *gin.Context) (*models.User, bool) {
	userInterface, exists := c.Get("current_user")
	if !exists {
		return nil, false
	}
	user, ok := userInterface.(*models.User)
	return user, ok
}
