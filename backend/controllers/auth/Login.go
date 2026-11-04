package auth

import (
	"errors"
	"net/http"
	"oneimg/backend/config"
	"oneimg/backend/middlewares"
	"oneimg/backend/utils/authsecurity"
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
// Username/Password 不再用 binding:"required"：游客登录只带 action=guest，
// 结构体校验会先于业务逻辑把这种请求拒成 400。必填性在 loginDB 里手动校验，
// 账号登录缺用户名或密码同样返回 400，行为与之前一致。
type LoginRequest struct {
	Action             string         `json:"action"`
	Username           string         `json:"username" binding:"max=64"`
	Password           string         `json:"password" binding:"max=72"`
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
	// 游客登录（action=guest）不带账号密码；账号登录两者必填。
	// required 校验无法区分这两种形态，所以在绑定后手动分流。
	if req.Action != "guest" && (req.Username == "" || req.Password == "") {
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

	// A guest request may retain the old frontend's marker fields, but none of
	// those fields are accepted as proof of identity or as an ownership key.
	// 显式 action=guest 是新前端的游客登录形态（只带这一个字段）。
	isGuestRequest := req.Action == "guest" || req.Username == "guest" || strings.HasPrefix(req.Username, "guest_") || req.TouristFingerprint != ""
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
		// 用户不存在时也执行一次等价的 bcrypt 比较，抹平响应时间差，
		// 防止通过时序侧信道枚举有效用户名。
		bcrypt.CompareHashAndPassword([]byte(authsecurity.DummyBcryptHash), []byte(req.Password))
		// 计一次账号失败预算，避免「换 IP 猜同一账号」完全不受账号维度约束。
		middlewares.DefaultAuthLimiter().NoteLoginFailure(req.Username)
		c.JSON(http.StatusUnauthorized, result.Error(401, "用户名或密码错误"))
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password)); err != nil {
		middlewares.DefaultAuthLimiter().NoteLoginFailure(req.Username)
		c.JSON(http.StatusUnauthorized, result.Error(401, "用户名或密码错误"))
		return
	}

	// 登录成功即清空该账号的失败预算。
	// 这一步是「定向锁死」防线的关键：攻击者再怎么刷错误密码，
	// 只要真实用户成功登录一次，预算就归零，不会长期卡在 429。
	middlewares.DefaultAuthLimiter().ClearLoginFailures(req.Username)

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

// Logout 清除当前会话。
func Logout(c *gin.Context) {
	if err := middlewares.DeleteCurrentSession(c); err != nil {
		c.JSON(500, result.Error(500, "退出登录失败"))
		return
	}
	c.JSON(http.StatusOK, result.Success("退出登录成功", nil))
}
