package controllers

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"oneimg/backend/config"
	"strings"
	"time"

	"oneimg/backend/database"
	"oneimg/backend/models"
	"oneimg/backend/utils/result"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
)

// LoginRequest 登录请求体。
type LoginRequest struct {
	Username           string         `json:"username" binding:"required"`
	Password           string         `json:"password" binding:"required"`
	PowToken           string         `json:"powToken"`
	TouristFingerprint string         `json:"touristFingerprint"`
	FusionHash         string         `json:"fusionHash"`
	StableFeatures     map[string]any `json:"stableFeatures"`
}

// LoginResponse 登录响应体（兼容旧字段）。
type LoginResponse struct {
	Token string       `json:"token,omitempty"`
	User  *models.User `json:"user,omitempty"`
}

// Login 处理账号密码登录；开启游客时支持指纹/UUID 游客会话。
func Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, result.Error(400, "请求参数错误"))
		return
	}

	db := database.GetDB()
	if db == nil {
		c.JSON(http.StatusInternalServerError, result.Error(500, "数据库连接失败"))
		return
	}

	var settings models.Settings
	sqlResult := db.DB.First(&settings)
	if sqlResult.Error != nil {
		if strings.Contains(sqlResult.Error.Error(), "record not found") {
			c.JSON(http.StatusInternalServerError, result.Error(500, "系统配置未初始化"))
		} else {
			c.JSON(http.StatusInternalServerError, result.Error(500, "配置信息查询失败"))
		}
		return
	}

	if settings.PowVerify {
		if req.PowToken == "" {
			c.JSON(http.StatusBadRequest, result.Error(400, "请输入pow token"))
			return
		}
		if !ValidatePowToken(req.PowToken) {
			c.JSON(http.StatusBadRequest, result.Error(400, "pow token验证失败"))
			return
		}
	}

	// 游客：指纹 UUID / guest_ 前缀 / 固定 guest
	if settings.Tourist {
		isTourist := len(req.TouristFingerprint) == 36 ||
			strings.HasPrefix(req.Username, "guest_") ||
			req.Username == "guest"

		if isTourist {
			touristUUID := req.TouristFingerprint
			if touristUUID == "" {
				touristUUID = req.Username
				if touristUUID == "guest" {
					touristUUID = generateRandomUUID()
				}
			}

			touristID := int(generateTouristID(touristUUID))
			touristUser := &models.User{
				ID:       touristID,
				Role:     models.RoleGuest,
				Username: touristUUID,
			}

			session, err := SetSession(c, touristUser)
			if err != nil {
				c.JSON(http.StatusInternalServerError, result.Error(500, "游客登录失败："+err.Error()))
				return
			}

			c.JSON(http.StatusOK, result.Success("游客登录成功", map[string]any{
				"token": session.ID(),
				"user": &models.User{
					ID:       touristUser.ID,
					Role:     models.RoleGuest,
					Username: touristUser.Username,
				},
			}))
			return
		}
	}

	var user models.User
	userInfo := db.DB.Where("username = ?", req.Username).First(&user)
	if userInfo.Error != nil {
		c.JSON(http.StatusBadRequest, result.Error(401, "用户名或密码错误"))
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password)); err != nil {
		c.JSON(http.StatusBadRequest, result.Error(401, "用户名或密码错误"))
		return
	}

	session, err := SetSession(c, &user)
	if err != nil {
		return
	}

	user.Password = ""
	c.JSON(http.StatusOK, result.Success("登录成功", map[string]any{
		"token": session.ID(),
		"user":  user,
	}))
}

// generateTouristID 由 UUID 派生稳定游客数字 ID（避开 1 号超管）。
func generateTouristID(uuid string) uint {
	var id uint = 2
	for _, c := range uuid {
		id = id*31 + uint(c)
	}
	if id <= 2 {
		id += 100000
	}
	return id
}

// generateRandomUUID 生成 UUID v4。
func generateRandomUUID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "guest_" + time.Now().Format("20060102150405.000000000")
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

// SetSession 写入用户会话并返回 session 对象。
func SetSession(c *gin.Context, user *models.User) (sessions.Session, error) {
	session, err := saveUserSession(c, user)
	if err != nil {
		c.JSON(http.StatusInternalServerError, result.Error(500, "session保存失败："+err.Error()))
		return nil, err
	}
	return session, nil
}

// saveUserSession 仅保存会话，由调用方决定 JSON 或重定向响应。
func saveUserSession(c *gin.Context, user *models.User) (sessions.Session, error) {
	session := sessions.Default(c)
	session.Set("user_id", user.ID)
	session.Set("user_role", user.Role)
	session.Set("username", user.Username)
	session.Set("logged_in", true)
	session.Options(sessions.Options{
		MaxAge:   24 * 60 * 60,
		HttpOnly: true,
		Secure:   strings.HasPrefix(strings.ToLower(config.App.AppURL), "https://"),
		SameSite: http.SameSiteStrictMode,
		Path:     "/",
	})
	if err := session.Save(); err != nil {
		return nil, err
	}
	return session, nil
}

// ValidatePowToken 向 PoW 服务校验 token。
func ValidatePowToken(token string) bool {
	if token == "" {
		return false
	}

	type reqBody struct {
		Token string `json:"token"`
	}
	body, err := json.Marshal(reqBody{Token: token})
	if err != nil {
		return false
	}

	req, err := http.NewRequest("POST", "https://cha.eta.im/api/validate", strings.NewReader(string(body)))
	if err != nil {
		return false
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/128.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "application/json, text/plain, */*")
	req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9")

	client := &http.Client{
		Timeout: 5 * time.Second,
		Transport: &http.Transport{
			DisableCompression: true,
		},
	}
	resp, err := client.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return false
	}

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return false
	}
	var validationResp struct {
		Success bool `json:"success"`
	}
	if err := json.Unmarshal(respBody, &validationResp); err != nil {
		return false
	}
	return validationResp.Success
}

// Logout 清除当前会话。
func Logout(c *gin.Context) {
	session := sessions.Default(c)
	session.Clear()
	if err := session.Save(); err != nil {
		c.JSON(http.StatusInternalServerError, result.Error(500, "退出登录失败"))
		return
	}
	c.JSON(http.StatusOK, result.Success("退出登录成功", nil))
}
