package controllers

import (
	"errors"
	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"oneimg/backend/database"
	"oneimg/backend/middlewares"
	"oneimg/backend/models"
	"oneimg/backend/utils/authsecurity"
	"oneimg/backend/utils/result"
	"regexp"
	"strings"
	"time"
)

type ChangeAccountInfoRequest struct {
	CurrentPassword string `json:"current_password" binding:"required,max=72"`
	NewPassword     string `json:"new_password" binding:"max=72"`
	NewUsername     string `json:"new_username" binding:"max=50"`
}
type AccountResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Success bool   `json:"success"`
}

var uuidRegex = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

func ChangeAccountInfo(c *gin.Context) {
	user, ok := middlewares.GetCurrentUser(c)
	if !ok || user.ID <= 0 || user.Role == models.RoleGuest || c.GetString("auth_method") == "api_token" {
		c.JSON(403, result.Error(403, "游客或 API 凭据不能修改账户"))
		return
	}
	var persisted models.User
	if err := database.GetDB().DB.First(&persisted, user.ID).Error; err != nil {
		c.JSON(401, result.Error(401, "用户已不存在"))
		return
	}
	var req ChangeAccountInfoRequest
	if c.ShouldBindJSON(&req) != nil {
		c.JSON(400, result.Error(400, "请求参数错误"))
		return
	}
	if bcrypt.CompareHashAndPassword([]byte(persisted.Password), []byte(req.CurrentPassword)) != nil {
		c.JSON(401, result.Error(401, "当前密码错误"))
		return
	}
	updates := map[string]any{}
	if req.NewUsername != "" {
		if req.NewUsername != strings.TrimSpace(req.NewUsername) || len([]rune(req.NewUsername)) < 3 || isTouristUsername(req.NewUsername) {
			c.JSON(400, result.Error(400, "用户名格式无效或为保留名称"))
			return
		}
		updates["username"] = req.NewUsername
	}
	if req.NewPassword != "" {
		if len(req.NewPassword) < 8 {
			c.JSON(400, result.Error(400, "新密码至少 8 个字符"))
			return
		}
		hash, err := hashPassword(req.NewPassword)
		if err != nil {
			c.JSON(500, result.Error(500, "密码处理失败"))
			return
		}
		updates["password"] = hash
	}
	if len(updates) == 0 {
		c.JSON(400, result.Error(400, "没有需要更新的内容"))
		return
	}
	db := database.GetDB().DB
	err := db.Transaction(func(tx *gorm.DB) error {
		var latest models.User
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&latest, user.ID).Error; err != nil {
			return err
		}
		if latest.AuthVersion != persisted.AuthVersion {
			return middlewares.ErrSessionRevoked
		}
		if err := tx.Model(&latest).Updates(updates).Error; err != nil {
			return err
		}
		if err := authsecurity.RevokeUserSessions(tx, user.ID); err != nil {
			return err
		}
		return tx.Model(&models.APICredential{}).Where("owner_id = ?", user.ID).Update("revoked", true).Error
	})
	if err != nil {
		if errors.Is(err, middlewares.ErrSessionRevoked) {
			c.JSON(401, result.Error(401, "会话已撤销，请重新登录"))
			return
		}
		c.JSON(409, result.Error(409, "账户更新失败，请检查用户名是否已存在"))
		return
	}
	if err := middlewares.DeleteCurrentSession(c); err != nil {
		c.JSON(500, result.Error(500, "账户已更新，但浏览器会话清理失败"))
		return
	}
	c.JSON(200, result.Success("账户已更新，所有登录会话已撤销，请重新登录", nil))
}

// GetUUID is a public owner key, never identity proof. Only AuthMiddleware sets
// username; guest UUID ownership additionally requires the negative server ID.
func CurrentOwnerKey(c *gin.Context) string { return c.GetString("username") }
func GetUUID(c *gin.Context) string         { return CurrentOwnerKey(c) }

func isTouristUsername(username string) bool {
	return username == "guest" || strings.HasPrefix(username, "guest_") || uuidRegex.MatchString(username)
}

// Administrative global invalidation, not merely clearing the caller's cookie.
func ClearAllSessions(c *gin.Context) {
	if err := database.GetDB().DB.Transaction(authsecurity.RevokeAllSessions); err != nil {
		c.JSON(500, result.Error(500, "清除所有会话失败"))
		return
	}
	if err := middlewares.DeleteCurrentSession(c); err != nil {
		c.JSON(500, result.Error(500, "会话已撤销，浏览器 Cookie 清理失败"))
		return
	}
	c.JSON(200, result.Success("所有登录会话已清除；游客安全恢复凭据仍可用于重新登录", nil))
}

// ListSessions exposes a one-way row handle, never the bearer cookie or payload.
func ListSessions(c *gin.Context) {
	if c.GetString("auth_method") == "api_token" {
		c.JSON(403, result.Error(403, "需要登录会话"))
		return
	}
	user, ok := middlewares.GetCurrentUser(c)
	if !ok {
		c.JSON(401, result.Error(401, "未登录"))
		return
	}
	var rows []models.AuthSession
	var state models.AuthState
	db := database.GetDB().DB
	if err := db.First(&state, 1).Error; err != nil {
		internalFailure(c, "读取会话状态失败", err)
		return
	}
	c.Header("Cache-Control", "private, no-store")
	if err := db.Where("user_id = ? AND user_version = ? AND epoch = ? AND expires_at > ?", user.ID, user.AuthVersion, state.Epoch, time.Now()).Order("created_at desc").Limit(100).Find(&rows).Error; err != nil {
		c.JSON(500, result.Error(500, "获取会话失败"))
		return
	}
	current := authsecurity.Digest(sessions.Default(c).ID())
	items := make([]gin.H, 0, len(rows))
	for _, row := range rows {
		items = append(items, gin.H{"created_at": row.CreatedAt, "expires_at": row.ExpiresAt, "id": row.ID, "current": row.ID == current})
	}
	c.JSON(200, result.Success("ok", items))
}

// RevokeOwnSessions supports all and others; both advance the credential epoch
// so a concurrently executing login using the old password cannot resurrect it.
func RevokeOwnSessions(c *gin.Context) {
	user, ok := middlewares.GetCurrentUser(c)
	if !ok || user.ID <= 0 || c.GetString("auth_method") == "api_token" {
		c.JSON(403, result.Error(403, "游客请使用退出登录"))
		return
	}
	var req struct {
		Scope string `json:"scope" binding:"required,oneof=all others"`
	}
	if c.ShouldBindJSON(&req) != nil {
		c.JSON(400, result.Error(400, "scope 必须为 all 或 others"))
		return
	}
	err := database.GetDB().DB.Transaction(func(tx *gorm.DB) error {
		var latest models.User
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&latest, user.ID).Error; err != nil {
			return err
		}
		if latest.AuthVersion != user.AuthVersion {
			return middlewares.ErrSessionRevoked
		}
		if err := authsecurity.RevokeUserSessions(tx, user.ID); err != nil {
			return err
		}
		latest.AuthVersion++
		*user = latest
		return nil
	})
	if err != nil {
		c.JSON(500, result.Error(500, "撤销会话失败"))
		return
	}
	if req.Scope == "others" {
		sessions.Default(c).Set("_reissue", true)
		if _, err := saveUserSession(c, user); err != nil {
			c.JSON(500, result.Error(500, "会话已撤销，请重新登录"))
			return
		}
	} else if middlewares.DeleteCurrentSession(c) != nil {
		c.JSON(500, result.Error(500, "会话已撤销，Cookie 清理失败"))
		return
	}
	c.JSON(200, result.Success("会话已撤销", nil))
}
