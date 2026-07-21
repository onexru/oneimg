package controllers

import (
	"net/http"
	"regexp"
	"strings"

	"oneimg/backend/database"
	"oneimg/backend/models"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
)

// ChangeAccountInfoRequest 修改账户信息请求体。
type ChangeAccountInfoRequest struct {
	CurrentPassword string `json:"current_password" binding:"required"`
	NewPassword     string `json:"new_password" binding:"min=6"`
	NewUsername     string `json:"new_username" binding:"max=64"`
}

// AccountResponse 账户相关接口响应体。
type AccountResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Success bool   `json:"success"`
}

// uuidRegex 匹配标准 UUID（游客用户名格式）。
var uuidRegex = regexp.MustCompile("^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$")

// ChangeAccountInfo 修改当前登录用户的用户名/密码；成功后强制重新登录。
func ChangeAccountInfo(c *gin.Context) {
	var req ChangeAccountInfoRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, AccountResponse{
			Code:    400,
			Message: "请求参数错误: " + err.Error(),
			Success: false,
		})
		return
	}

	session := sessions.Default(c)
	userID := session.Get("user_id")
	if userID == nil {
		c.JSON(http.StatusUnauthorized, AccountResponse{
			Code:    401,
			Message: "未登录",
			Success: false,
		})
		return
	}

	// Session 角色可能是 int/float64/uint，统一转 int
	role := 0
	if userRole := session.Get("user_role"); userRole != nil {
		switch v := userRole.(type) {
		case int:
			role = v
		case float64:
			role = int(v)
		case uint:
			role = int(v)
		}
	}

	// 仅管理员可改用户名
	if role != models.RoleAdmin && req.NewUsername != "" {
		c.JSON(http.StatusForbidden, AccountResponse{
			Code:    403,
			Message: "无权修改用户名",
			Success: false,
		})
		return
	}

	db := database.GetDB().DB
	var user models.User
	if err := db.First(&user, userID).Error; err != nil {
		c.JSON(http.StatusNotFound, AccountResponse{
			Code:    404,
			Message: "用户不存在",
			Success: false,
		})
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.CurrentPassword)); err != nil {
		c.JSON(http.StatusBadRequest, AccountResponse{
			Code:    400,
			Message: "当前密码错误",
			Success: false,
		})
		return
	}

	if req.NewUsername != "" && len(req.NewUsername) < 3 {
		c.JSON(http.StatusBadRequest, AccountResponse{
			Code:    400,
			Message: "用户名长度不能小于3",
			Success: false,
		})
		return
	}

	if req.NewUsername == "" && req.NewPassword == "" {
		c.JSON(http.StatusBadRequest, AccountResponse{
			Code:    400,
			Message: "请至少修改一项",
			Success: false,
		})
		return
	}

	tx := db.Begin()
	if err := tx.Error; err != nil {
		c.JSON(http.StatusInternalServerError, AccountResponse{
			Code:    500,
			Message: "数据库操作失败: " + err.Error(),
			Success: false,
		})
		return
	}

	if req.NewUsername != "" {
		if isTouristUsername(req.NewUsername) {
			tx.Rollback()
			c.JSON(http.StatusBadRequest, AccountResponse{
				Code:    400,
				Message: "游客保留用户名",
				Success: false,
			})
			return
		}

		var existingUser models.User
		if err := db.Where("username = ? AND id != ?", req.NewUsername, userID).First(&existingUser).Error; err == nil {
			tx.Rollback()
			c.JSON(http.StatusBadRequest, AccountResponse{
				Code:    400,
				Message: "用户名已存在",
				Success: false,
			})
			return
		}

		if err := tx.Model(&user).Update("username", req.NewUsername).Error; err != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, AccountResponse{
				Code:    500,
				Message: "用户名更新失败",
				Success: false,
			})
			return
		}
	}

	if req.NewPassword != "" {
		hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
		if err != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, AccountResponse{
				Code:    500,
				Message: "密码加密失败",
				Success: false,
			})
			return
		}

		if err := tx.Model(&user).Update("password", string(hashedPassword)).Error; err != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, AccountResponse{
				Code:    500,
				Message: "密码更新失败",
				Success: false,
			})
			return
		}
	}

	if err := tx.Commit().Error; err != nil {
		c.JSON(http.StatusInternalServerError, AccountResponse{
			Code:    500,
			Message: "数据库操作失败: " + err.Error(),
			Success: false,
		})
		return
	}

	// 凭证变更后清除会话，要求重新登录
	session.Clear()
	if err := session.Save(); err != nil {
		c.JSON(http.StatusInternalServerError, AccountResponse{
			Code:    500,
			Message: "会话失效失败: " + err.Error(),
			Success: false,
		})
		return
	}

	c.JSON(http.StatusOK, AccountResponse{
		Code:    200,
		Message: "修改成功",
		Success: true,
	})
}

// isTouristUsername 判断是否为游客/保留用户名。
func isTouristUsername(username string) bool {
	return strings.HasPrefix(username, "guest_") || username == "guest" || uuidRegex.MatchString(username)
}

// ClearAllSessions 清除当前请求的 Session。
func ClearAllSessions(c *gin.Context) {
	session := sessions.Default(c)
	session.Clear()
	if err := session.Save(); err != nil {
		c.JSON(http.StatusInternalServerError, AccountResponse{
			Code:    500,
			Message: "清除会话失败",
			Success: false,
		})
		return
	}

	c.JSON(http.StatusOK, AccountResponse{
		Code:    200,
		Message: "所有会话已清除",
		Success: true,
	})
}

// GetUUID 返回游客标识：优先会话中的 username（多为 UUID）。
func GetUUID(c *gin.Context) string {
	return c.GetString("username")
}
