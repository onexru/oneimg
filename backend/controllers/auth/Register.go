package auth

import (
	"net/http"
	"oneimg/backend/database"
	"oneimg/backend/models"
	"oneimg/backend/utils/result"
	"oneimg/backend/utils/settings"
	"strings"
	"time"

	shared "oneimg/backend/controllers/shared"
	usersctrl "oneimg/backend/controllers/users"

	"github.com/gin-gonic/gin"
)

// Register 用户自助注册（需开启注册开关）。
func Register(c *gin.Context) {
	settings, err := settings.GetSettings()
	if err != nil {
		c.JSON(http.StatusInternalServerError, result.Fail(500, "获取设置失败"))
		return
	}

	if !settings.StartRegister {
		c.JSON(http.StatusForbidden, result.Fail(403, "暂未开放注册"))
		return
	}

	type RegisterReq struct {
		Username       string `json:"username" binding:"required,min=3,max=50"`
		Password       string `json:"password" binding:"required,min=8,max=72"`
		TurnstileToken string `json:"turnstileToken"`
		CapToken       string `json:"capToken"`
	}
	var req RegisterReq
	if !shared.BindJSON(c, &req) {
		return
	}

	if ok, errMsg, fallback := verifyHuman(c, settings, req.TurnstileToken, req.CapToken); !ok {
		if fallback != "" {
			c.JSON(http.StatusForbidden, gin.H{
				"code":    403,
				"message": errMsg,
				"data":    gin.H{"verify_fallback": fallback},
			})
			return
		}
		c.JSON(http.StatusForbidden, result.Error(403, errMsg))
		return
	}

	if req.Username != strings.TrimSpace(req.Username) || shared.IsTouristUsername(req.Username) || len(req.Password) > 72 {
		c.JSON(400, result.Error(400, "用户名为保留名称或密码长度无效"))
		return
	}
	db := database.GetDB().DB

	if db.Where("username = ?", req.Username).First(&models.User{}).Error == nil {
		c.JSON(http.StatusBadRequest, result.Error(400, "用户名已存在"))
		return
	}

	// 密码加密
	hashedPwd, err := usersctrl.HashPassword(req.Password)
	if err != nil {
		c.JSON(http.StatusInternalServerError, result.Fail(500, "密码加密失败"))
		return
	}

	newUser := models.User{
		Username: req.Username,
		Password: hashedPwd,
		Role:     3,
		Permission: models.Permission{
			Buckets: []int{},
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	// 创建用户
	if err := db.Create(&newUser).Error; err != nil {
		errMsg := err.Error()
		if strings.Contains(errMsg, "unique constraint") || strings.Contains(errMsg, "duplicate key") {
			c.JSON(http.StatusBadRequest, result.Fail(400, "用户已存在"))
			return
		}
		// 原始 DB 错误（表名/列名/驱动信息）只进服务端日志，不回给客户端。
		shared.InternalFailure(c, "注册失败", err)
		return
	}

	resp := map[string]any{
		"id":       newUser.ID,
		"username": newUser.Username,
		"role":     newUser.Role,
	}
	c.JSON(http.StatusOK, result.Success("注册成功", resp))
}
