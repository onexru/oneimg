package controllers

import (
	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"oneimg/backend/database"
	"oneimg/backend/middlewares"
	"oneimg/backend/models"
	"oneimg/backend/utils/authsecurity"
	"oneimg/backend/utils/result"
	"strings"
)

func RevokeAccountSession(c *gin.Context) {
	user, ok := middlewares.GetCurrentUser(c)
	if !ok || user.ID <= 0 || c.GetString("auth_method") == "api_token" {
		c.JSON(403, result.Error(403, "需要已登录账户会话"))
		return
	}
	id := c.Param("id")
	if len(id) != 64 || strings.Trim(id, "0123456789abcdef") != "" {
		c.JSON(400, result.Error(400, "会话ID无效"))
		return
	}
	current := authsecurity.Digest(sessions.Default(c).ID())
	if id == current {
		c.JSON(400, result.Error(400, "请使用退出登录结束当前会话"))
		return
	}
	db := database.GetDB().DB
	var target models.AuthSession
	if err := db.Where("id = ? AND user_id = ?", id, user.ID).First(&target).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(404, result.Error(404, "会话不存在"))
			return
		}
		internalFailure(c, "读取会话失败", err)
		return
	}
	if err := db.Where("id = ? AND user_id = ?", id, user.ID).Delete(&models.AuthSession{}).Error; err != nil {
		internalFailure(c, "撤销会话失败", err)
		return
	}
	c.JSON(200, result.Success("会话已撤销", nil))
}
