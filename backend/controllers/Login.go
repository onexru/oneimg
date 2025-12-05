package controllers

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	"oneimg/backend/database"
	"oneimg/backend/models"
	"oneimg/backend/utils/result"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
)

// LoginRequest 登录请求结构
type LoginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
	PowToken string `json:"powToken"`
}

// LoginResponse 登录响应结构
type LoginResponse struct {
	Token string       `json:"token,omitempty"`
	User  *models.User `json:"user,omitempty"`
}

func Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, result.Error(
			400,
			"请求参数错误",
		))
		return
	}

	// 获取数据库实例
	db := database.GetDB()
	if db == nil {
		c.JSON(http.StatusInternalServerError, result.Error(500, "数据库连接失败"))
		return
	}

	var settings models.Settings
	// 检查记录是否存在
	sqlResult := db.DB.First(&settings)
	if sqlResult.Error != nil {
		// 区分记录不存在和数据库错误
		if strings.Contains(sqlResult.Error.Error(), "record not found") {
			c.JSON(http.StatusInternalServerError, result.Error(500, "系统配置未初始化"))
		} else {
			c.JSON(http.StatusInternalServerError, result.Error(500, "配置信息查询失败"))
		}
		return
	}

	// 检查是否开启了pow验证
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

	// 检查是否开启了游客登录
	if settings.Tourist && req.Username == "guest" {
		// 直接使用游客登录
		session, err := SetSession(c, &models.User{
			Id:       2,
			Username: "guest",
		})

		if err != nil {
			c.JSON(http.StatusInternalServerError, result.Error(500, "游客登录失败："+err.Error()))
			return
		}

		c.JSON(http.StatusOK, result.Success("游客登录成功", map[string]any{
			"token": session.ID(),
			"user": &models.User{
				Id:       2,
				Username: "guest",
			},
		}))
		return
	}

	// 查询用户信息
	var user models.User
	userInfo := db.DB.Where("username = ?", req.Username).First(&user)

	// 用户不存在
	if userInfo.Error != nil {
		// 统一返回相同提示，避免用户名枚举
		c.JSON(http.StatusBadRequest, result.Error(401, "用户名或密码错误"))
		return
	}

	// 验证密码
	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password)); err != nil {
		c.JSON(http.StatusBadRequest, result.Error(401, "用户名或密码错误"))
		return
	}

	// 设置session
	session, err := SetSession(c, &user)
	if err != nil {
		return // 错误已在SetSession中处理
	}

	// 返回结果去除密码
	user.Password = ""
	// 返回结果
	c.JSON(http.StatusOK, result.Success("登录成功", map[string]any{
		"token": session.ID(),
		"user":  user,
	}))
}

// 设置Session
func SetSession(c *gin.Context, user *models.User) (sessions.Session, error) {
	// 获取session
	session := sessions.Default(c)

	// 设置session数据
	session.Set("user_id", user.Id)
	session.Set("username", user.Username)
	session.Set("logged_in", true)

	// 设置session选项
	session.Options(sessions.Options{
		MaxAge:   24 * 60 * 60,            // 24小时，单位秒
		HttpOnly: true,                    // 防止XSS攻击
		Secure:   false,                   // 生产环境应设为true（需要HTTPS）
		SameSite: http.SameSiteStrictMode, // 防止CSRF攻击
		Path:     "/",                     // cookie路径
	})

	// 保存session
	if err := session.Save(); err != nil {
		errMsg := "session保存失败：" + err.Error()
		c.JSON(http.StatusInternalServerError, result.Error(500, errMsg))
		return nil, err
	}

	return session, nil
}

func ValidatePowToken(token string) bool {
	if token == "" {
		return false
	}

	// 构建请求体
	type reqBody struct {
		Token string `json:"token"`
	}
	body, err := json.Marshal(reqBody{Token: token})
	if err != nil {
		return false
	}

	// 创建请求
	req, err := http.NewRequest("POST", "https://cha.eta.im/api/validate", strings.NewReader(string(body)))
	if err != nil {
		return false
	}

	// 使用与前端完全一致的浏览器头信息
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/128.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "application/json, text/plain, */*")
	req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9")

	// 发送请求
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

	// 检查HTTP响应状态码
	if resp.StatusCode != http.StatusOK {
		return false
	}

	// 解析响应
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return false
	}

	var validationResp struct {
		Success bool `json:"success"`
	}
	// 处理JSON解析
	if err := json.Unmarshal(respBody, &validationResp); err != nil {
		return false
	}

	return validationResp.Success
}

// 退出登录
func Logout(c *gin.Context) {
	session := sessions.Default(c)
	session.Clear()
	if err := session.Save(); err != nil {
		c.JSON(http.StatusInternalServerError, result.Error(500, "退出登录失败"))
		return
	}

	c.JSON(http.StatusOK, result.Success("退出登录成功", nil))
}
