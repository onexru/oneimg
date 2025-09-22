package controllers

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"oneimg/backend/database"
	"oneimg/backend/models"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
)

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

	// 关键修复：使用与前端完全一致的浏览器头信息
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

	// 解析响应
	respBody, _ := io.ReadAll(resp.Body)

	var validationResp struct {
		Success bool `json:"success"`
	}
	json.Unmarshal(respBody, &validationResp)
	return validationResp.Success
}

// LoginRequest 登录请求结构
type LoginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
	PowToken string `json:"powToken"` // 去掉omitempty，确保始终能接收到该字段
}

// LoginResponse 登录响应结构
type LoginResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Success bool   `json:"success"`
	Token   string `json:"token,omitempty"`
	User    *User  `json:"user,omitempty"`
}

// User 用户信息结构（不包含密码）
type User struct {
	ID       int    `json:"id"`
	Username string `json:"username"`
}

// Login 用户登录
func Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, LoginResponse{
			Code:    400,
			Message: "请求参数错误: " + err.Error(),
			Success: false,
		})
		return
	}

	// 日志：输出接收到的POW token（可在调试完成后移除）
	log.Printf("接收到的POW token: %s", req.PowToken)

	// 验证POW token - 修复条件判断逻辑
	// 如果服务要求必须提供POW token，则使用下面的判断
	if req.PowToken == "" {
		c.JSON(http.StatusBadRequest, LoginResponse{
			Code:    400,
			Message: "请提供POW token",
			Success: false,
		})
		return
	}

	// 验证POW token
	if !ValidatePowToken(req.PowToken) {
		c.JSON(http.StatusBadRequest, LoginResponse{
			Code:    400,
			Message: "POW验证失败",
			Success: false,
		})
		return
	}

	// 获取数据库实例
	db := database.GetDB()
	if db == nil {
		c.JSON(http.StatusInternalServerError, LoginResponse{
			Code:    500,
			Message: "数据库连接失败",
			Success: false,
		})
		return
	}

	// 查找用户
	var user models.User
	result := db.DB.Where("username = ?", req.Username).First(&user)
	if result.Error != nil {
		c.JSON(http.StatusUnauthorized, LoginResponse{
			Code:    401,
			Message: "用户名或密码错误",
			Success: false,
		})
		return
	}

	// 验证密码
	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password)); err != nil {
		c.JSON(http.StatusUnauthorized, LoginResponse{
			Code:    401,
			Message: "用户名或密码错误",
			Success: false,
		})
		return
	}

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
		c.JSON(http.StatusInternalServerError, LoginResponse{
			Code:    500,
			Message: "保存会话失败",
			Success: false,
		})
		return
	}

	// 返回成功响应
	c.JSON(http.StatusOK, LoginResponse{
		Code:    200,
		Message: "登录成功",
		Success: true,
		User: &User{
			ID:       user.Id,
			Username: user.Username,
		},
	})
}
