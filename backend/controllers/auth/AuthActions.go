package auth

import (
	"errors"
	"net/http"
	"oneimg/backend/database"
	"oneimg/backend/middlewares"
	"oneimg/backend/models"
	"oneimg/backend/utils/authsecurity"
	"oneimg/backend/utils/result"
	"oneimg/backend/utils/settings"

	"github.com/gin-gonic/gin"
)

// apiExpiryRequest 是所有凭据有效期接口的公共请求体。
//
// Expiry 为空表示「沿用默认 30 天」，正是本次改动之前的行为，因此旧的
// 调用方（脚本、第三方集成）不传 body 也照旧拿到一张 30 天的凭据。
type apiExpiryRequest struct {
	Expiry string `json:"expiry"`
}

// bindAPITokenExpiry 解析有效期配置。空 body、空字段都落到默认值上，
// 只有语法/范围错误才报 400，避免把「老客户端不带参数」当成错误。
func bindAPITokenExpiry(c *gin.Context) (authsecurity.TokenExpiry, bool) {
	var req apiExpiryRequest
	if c.Request.ContentLength != 0 {
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, result.Error(400, "请求参数错误"))
			return authsecurity.TokenExpiry{}, false
		}
	}
	expiry, err := authsecurity.ParseTokenExpiry(req.Expiry)
	if err != nil {
		c.JSON(http.StatusBadRequest, result.Error(400, err.Error()))
		return authsecurity.TokenExpiry{}, false
	}
	return expiry, true
}

func canManageAPICredential(c *gin.Context, user *models.User) bool {
	return user != nil && user.ID > 0 && c.GetString("auth_method") != "api_token" &&
		(user.ID == models.SuperAdminID || user.Permission.HasPermission("setting:api"))
}

// RegenerateAPIToken returns the generated secret exactly once. GET never calls
// this handler; token material cannot be recovered from settings or its digest.
func RegenerateAPIToken(c *gin.Context) {
	user, ok := middlewares.GetCurrentUser(c)
	if !ok {
		c.JSON(401, result.Error(401, "未登录"))
		return
	}
	if !canManageAPICredential(c, user) {
		c.JSON(403, result.Error(403, "无权生成 API 凭据"))
		return
	}
	expiry, ok := bindAPITokenExpiry(c)
	if !ok {
		return
	}
	raw, row, err := authsecurity.NewAPICredential(database.GetDB().DB, user.ID, expiry)
	if err != nil {
		c.JSON(500, result.Error(500, "API 凭据生成失败"))
		return
	}
	settings.Invalidate()
	c.Header("Cache-Control", "private, no-store")
	c.JSON(http.StatusOK, result.Success("API 凭据仅显示一次，请妥善保存", gin.H{
		"api_token":     raw,
		"expires_at":    row.ExpiresAt,
		"never_expires": row.NeverExpires,
		"expiry_spec":   row.ExpirySpec,
		"scopes":        row.Scopes,
	}))
}

// UpdateAPITokenExpiry 修改当前账号已生效凭据的有效期，不更换凭据本身。
// 这样已经部署在 CI、图床客户端里的 Token 不必为了「改成永久」而全部重配。
func UpdateAPITokenExpiry(c *gin.Context) {
	user, ok := middlewares.GetCurrentUser(c)
	if !ok {
		c.JSON(401, result.Error(401, "未登录"))
		return
	}
	if !canManageAPICredential(c, user) {
		c.JSON(403, result.Error(403, "无权修改 API 凭据"))
		return
	}
	expiry, ok := bindAPITokenExpiry(c)
	if !ok {
		return
	}
	row, err := authsecurity.UpdateAPICredentialExpiry(database.GetDB().DB, user.ID, expiry)
	switch {
	case errors.Is(err, authsecurity.ErrNoActiveCredential):
		c.JSON(http.StatusNotFound, result.Error(404, "当前没有有效 API 凭据，请先生成"))
		return
	case errors.Is(err, authsecurity.ErrCredentialExpired):
		c.JSON(http.StatusConflict, result.Error(409, "凭据已过期，请重新生成后再设置有效期"))
		return
	case err != nil:
		c.JSON(500, result.Error(500, "有效期更新失败"))
		return
	}
	settings.Invalidate()
	c.Header("Cache-Control", "private, no-store")
	c.JSON(http.StatusOK, result.Success("有效期已更新", gin.H{
		"expires_at":    row.ExpiresAt,
		"never_expires": row.NeverExpires,
		"expiry_spec":   row.ExpirySpec,
		"scopes":        row.Scopes,
	}))
}

func RenewSession(c *gin.Context) {
	user, ok := middlewares.GetCurrentUser(c)
	if !ok || c.GetString("auth_method") == "api_token" {
		c.JSON(401, result.Error(401, "需要有效的登录会话"))
		return
	}
	if _, err := saveUserSession(c, user); err != nil {
		c.JSON(401, result.Error(401, "会话已撤销，请重新登录"))
		return
	}
	c.JSON(200, result.Success("会话已续期", nil))
}
