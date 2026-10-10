package controllers

import (
	"context"
	"errors"
	"net/http"
	"oneimg/backend/config"
	"oneimg/backend/middlewares"
	"oneimg/backend/utils/authsecurity"
	"oneimg/backend/utils/powverify"
	"oneimg/backend/utils/settings"
	"oneimg/backend/utils/uploadpolicy"
	"strings"
	"time"

	"oneimg/backend/database"
	"oneimg/backend/models"
	"oneimg/backend/utils/result"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

// LoginRequest 登录请求体。
type LoginRequest struct {
	Username           string         `json:"username" binding:"required,max=64"`
	Password           string         `json:"password" binding:"required,max=72"`
	PowToken           string         `json:"powToken"`
	TurnstileToken     string         `json:"turnstileToken"`
	CapToken           string         `json:"capToken"`
	TouristFingerprint string         `json:"touristFingerprint"`
	FusionHash         string         `json:"fusionHash"`
	StableFeatures     map[string]any `json:"stableFeatures"`
}

// LoginResponse contains no session bearer material.
type LoginResponse struct {
	User *models.User `json:"user,omitempty"`
}

// Login uses cookie sessions; guest fingerprints are compatibility markers, never credentials.
func Login(c *gin.Context) {
	db := database.GetDB()
	if db == nil || db.DB == nil {
		c.JSON(503, result.Error(503, "数据库连接失败"))
		return
	}
	loginDB(c, db.DB)
}
func loginDB(c *gin.Context, db *gorm.DB) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, result.Error(400, "请求参数错误"))
		return
	}

	var settings models.Settings
	sqlResult := db.Where("id = ?", 1).First(&settings)
	if sqlResult.Error != nil {
		if strings.Contains(sqlResult.Error.Error(), "record not found") {
			c.JSON(http.StatusInternalServerError, result.Error(500, "系统配置未初始化"))
		} else {
			c.JSON(http.StatusInternalServerError, result.Error(500, "配置信息查询失败"))
		}
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

	// A guest request may retain the old frontend's marker fields, but none of
	// those fields are accepted as proof of identity or as an ownership key.
	isGuestRequest := req.Username == "guest" || strings.HasPrefix(req.Username, "guest_") || req.TouristFingerprint != ""
	if isGuestRequest {
		if !settings.Tourist {
			c.JSON(403, result.Error(403, "游客模式未开启"))
			return
		}
		credential, _ := c.Cookie(authsecurity.GuestCookie)
		guest, raw, recovered, err := authsecurity.RecoverGuest(db, credential)
		if err != nil || guest.ID <= 0 || guest.ID > 2147483647 {
			c.JSON(500, result.Error(500, "游客身份创建失败"))
			return
		}
		http.SetCookie(c.Writer, &http.Cookie{Name: authsecurity.GuestCookie, Value: raw, Path: "/", HttpOnly: true,
			Secure:   config.App != nil && strings.HasPrefix(strings.ToLower(config.App.AppURL), "https://"),
			SameSite: http.SameSiteLaxMode, MaxAge: int(authsecurity.GuestLifetime.Seconds()), Expires: time.Now().Add(authsecurity.GuestLifetime)})
		guestUser := &models.User{ID: -guest.ID, Role: models.RoleGuest, Username: guest.OwnerKey}
		if _, err := SetSession(c, guestUser); err != nil {
			return
		}
		c.JSON(200, result.Success("游客登录成功", gin.H{"user": guestUser, "guest_recovered": recovered,
			"guest_notice": "游客身份由本浏览器的安全 Cookie 保存；清除此 Cookie 将无法恢复历史图片。旧指纹不能用于认领图片。"}))
		return
	}

	var user models.User
	userInfo := db.Where("username = ?", req.Username).First(&user)
	if userInfo.Error != nil {
		c.JSON(http.StatusUnauthorized, result.Error(401, "用户名或密码错误"))
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password)); err != nil {
		c.JSON(http.StatusUnauthorized, result.Error(401, "用户名或密码错误"))
		return
	}

	_, err := SetSession(c, &user)
	if err != nil {
		return
	}
	recordAccountLogin(c, db, user.ID, authsecurity.LoginMethodPassword)

	user.Password = ""
	c.JSON(http.StatusOK, result.Success("登录成功", map[string]any{
		"user": user,
	}))
}

// SetSession 写入用户会话并返回 session 对象。
func SetSession(c *gin.Context, user *models.User) (sessions.Session, error) {
	session, err := saveUserSession(c, user)
	if err != nil {
		if errors.Is(err, middlewares.ErrSessionRevoked) {
			c.JSON(401, result.Error(401, "会话已撤销，请重新登录"))
			return nil, err
		}
		c.JSON(http.StatusInternalServerError, result.Error(500, "会话创建失败，请稍后重试"))
		return nil, err
	}
	return session, nil
}

// saveUserSession 仅保存会话，由调用方决定 JSON 或重定向响应。
func saveUserSession(c *gin.Context, user *models.User) (sessions.Session, error) {
	session := sessions.Default(c)
	epoch := session.Get("_epoch")
	reissue := session.Get("_reissue")
	session.Clear()
	if reissue == true {
		session.Set("_reissue", true)
	}
	session.Set("_epoch", epoch)
	session.Set("_rotate", true)
	session.Set("auth_version", user.AuthVersion)
	session.Set("user_id", user.ID)
	session.Set("user_role", user.Role)
	session.Set("username", user.Username)
	session.Set("logged_in", true)
	session.Options(sessions.Options{
		MaxAge:   24 * 60 * 60,
		HttpOnly: true,
		Secure:   config.App != nil && strings.HasPrefix(strings.ToLower(config.App.AppURL), "https://"),
		SameSite: http.SameSiteLaxMode,
		Path:     "/",
	})
	if err := session.Save(); err != nil {
		return nil, err
	}
	return session, nil
}

// ValidatePowToken retains the legacy Go contract; HTTP request paths pass
// their cancellation context and the already-loaded settings instead.
func ValidatePowToken(token string) bool {
	s, err := settings.GetSettings()
	if err != nil {
		return false
	}
	return ValidatePowTokenWithSettings(context.Background(), s, token)
}
func ValidatePowTokenWithSettings(ctx context.Context, s models.Settings, token string) bool {
	if s.PowLocalFallback {
		return false
	}
	return powverify.Validate(ctx, uploadpolicy.PowURL(s), token, time.Duration(uploadpolicy.PowTimeoutSeconds(s))*time.Second)
}

// Logout 清除当前会话。
func Logout(c *gin.Context) {
	if err := middlewares.DeleteCurrentSession(c); err != nil {
		c.JSON(500, result.Error(500, "退出登录失败"))
		return
	}
	c.JSON(http.StatusOK, result.Success("退出登录成功", nil))
}
