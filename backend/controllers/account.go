package controllers

import (
	"net/http"

	"oneimg/backend/database"
	"oneimg/backend/models"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
)

// ChangeAccountInfoRequest 修改登录信息请求结构
type ChangeAccountInfoRequest struct {
	CurrentPassword string `json:"current_password" binding:"required"`
	NewPassword     string `json:"new_password" min=6"`
	NewUsername     string `json:"new_username" min=3,max=20"`
}

// AccountResponse 账户响应结构
type AccountResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Success bool   `json:"success"`
}

// ChangeAccountInfo 修改密码
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

	// 获取当前用户ID
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

	// 获取数据库实例
	db := database.GetDB().DB

	// 查找用户
	var user models.User
	if err := db.First(&user, userID).Error; err != nil {
		c.JSON(http.StatusNotFound, AccountResponse{
			Code:    404,
			Message: "用户不存在",
			Success: false,
		})
		return
	}

	// 验证当前密码
	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.CurrentPassword)); err != nil {
		c.JSON(http.StatusBadRequest, AccountResponse{
			Code:    400,
			Message: "当前密码错误",
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

	// 开启事务
	tx := db.Begin()
	if err := tx.Error; err != nil {
		c.JSON(http.StatusInternalServerError, AccountResponse{
			Code:    500,
			Message: "数据库操作失败: " + err.Error(),
			Success: false,
		})
		return
	}

	// 如果用户名存在修改用户
	if req.NewUsername != "" {
		var existingUser models.User
		if err := db.Where("username = ? AND id != ?", req.NewUsername, userID).First(&existingUser).Error; err == nil {
			c.JSON(http.StatusBadRequest, AccountResponse{
				Code:    400,
				Message: "用户名已存在",
				Success: false,
			})
			return
		}

		// 更新用户名
		if err := db.Model(&user).Update("username", req.NewUsername).Error; err != nil {
			c.JSON(http.StatusInternalServerError, AccountResponse{
				Code:    500,
				Message: "用户名更新失败",
				Success: false,
			})
			// 回滚事务
			tx.Rollback()
			return
		}
	}

	if req.NewPassword != "" {
		// 加密新密码
		hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
		if err != nil {
			c.JSON(http.StatusInternalServerError, AccountResponse{
				Code:    500,
				Message: "密码加密失败",
				Success: false,
			})
			// 回滚事务
			tx.Rollback()
			return
		}

		// 更新密码
		if err := db.Model(&user).Update("password", string(hashedPassword)).Error; err != nil {
			c.JSON(http.StatusInternalServerError, AccountResponse{
				Code:    500,
				Message: "密码更新失败",
				Success: false,
			})
			// 回滚事务
			tx.Rollback()
			return
		}
	}

	// 提交事务
	if err := tx.Commit().Error; err != nil {
		c.JSON(http.StatusInternalServerError, AccountResponse{
			Code:    500,
			Message: "数据库操作失败: " + err.Error(),
			Success: false,
		})
		return
	}

	// 退出登录
	session.Clear()
	session.Save()

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

// ClearAllSessions 清除所有会话
func ClearAllSessions(c *gin.Context) {
	// 获取当前session
	session := sessions.Default(c)

	// 清除当前session
	session.Clear()
	session.Save()
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
