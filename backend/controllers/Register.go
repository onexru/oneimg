package controllers

import (
	"net/http"
	"oneimg/backend/database"
	"oneimg/backend/models"
	"oneimg/backend/utils/result"
	"oneimg/backend/utils/settings"
	"strings"
	"time"

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
		PowToken       string `json:"powToken"`
		TurnstileToken string `json:"turnstileToken"`
		CapToken       string `json:"capToken"`
	}
	var req RegisterReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, result.Fail(400, "参数校验失败："+err.Error()))
		return
	}

	if ok, errMsg, fallback := verifyHuman(c, settings, req.PowToken, req.TurnstileToken, req.CapToken); !ok {
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

	if req.Username!=strings.TrimSpace(req.Username) || isTouristUsername(req.Username) || len(req.Password)>72 {
  c.JSON(400,result.Error(400,"用户名为保留名称或密码长度无效")); return
 }
 db := database.GetDB().DB

 if db.Where("username = ?", req.Username).First(&models.User{}).Error == nil {
		c.JSON(http.StatusBadRequest, result.Error(400, "用户名已存在"))
		return
	}

	// 密码加密
	hashedPwd, err := hashPassword(req.Password)
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
		c.JSON(http.StatusInternalServerError, result.Fail(500, "注册失败："+errMsg))
		return
	}

	resp := map[string]any{
		"id":       newUser.ID,
		"username": newUser.Username,
		"role":     newUser.Role,
	}
	c.JSON(http.StatusOK, result.Success("注册成功", resp))
}
