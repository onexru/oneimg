package controllers

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"oneimg/backend/config"
	"oneimg/backend/models"
	"oneimg/backend/utils/cappow"
	"oneimg/backend/utils/result"
	"oneimg/backend/utils/secureconfig"
	"oneimg/backend/utils/settings"

	"github.com/gin-gonic/gin"
)

// cappowStore 持有 cap-pow 签发的一次性 redeem token 与挑战重放防护状态。
var cappowStore = cappow.NewStore()

// cappowSecret 派生 cap-pow 的 HMAC 密钥（稳定、跨重启一致，无需后台配置）。
func cappowSecret() []byte {
	if config.App == nil {
		return nil
	}
	return cappow.DeriveSecret(config.App.ConfigSecret)
}

// turnstileBroken 记录服务端已观测到 Turnstile 密钥配置错误（invalid/missing-input-secret）。
// 置位后在 TTL 内将 Turnstile 视为不可用并回退到 cap-pow——回退完全由服务端判定，
// 不信任任何客户端标记，攻击者无法借此绕过 Turnstile。
var turnstileBroken struct {
	sync.RWMutex
	until time.Time
}

const turnstileBrokenTTL = 10 * time.Minute

func markTurnstileBroken() {
	turnstileBroken.Lock()
	turnstileBroken.until = time.Now().Add(turnstileBrokenTTL)
	turnstileBroken.Unlock()
}

func turnstileBrokenNow() bool {
	turnstileBroken.RLock()
	defer turnstileBroken.RUnlock()
	return time.Now().Before(turnstileBroken.until)
}

// validTurnstileSiteKey 宽松校验 Cloudflare Turnstile 站点公钥格式（0x 前缀 + 足够长度）。
// 只用于识别明显错误的占位/乱填公钥，阈值放宽以避免误判合法公钥。
func validTurnstileSiteKey(k string) bool {
	k = strings.TrimSpace(k)
	return strings.HasPrefix(k, "0x") && len(k) >= 16
}

// probeTurnstileSecret 在保存 Turnstile 密钥配置时发起一次模拟 siteverify 请求：
// 使用无效 token，若 Cloudflare 返回 invalid/missing-input-secret 则说明密钥未被识别，
// 判定配置无效并拒绝保存；网络异常或密钥被识别（invalid-input-response 等）则放行。
func probeTurnstileSecret(secret string) error {
	if strings.TrimSpace(secret) == "" {
		return nil
	}
	success, errCodes := ValidateTurnstileToken("__oneimg_config_probe__", secret, "")
	if !success {
		for _, code := range errCodes {
			switch code {
			case "invalid-input-secret", "missing-input-secret":
				return errors.New("Turnstile 密钥无效，Cloudflare 未识别，请检查后重试")
			}
		}
	}
	return nil
}

// validateTurnstileSiteKeyAPI 通过 Cloudflare 管理 API 校验站点公钥是否真实存在。
// 返回 nil 表示公钥有效或暂时无法验证（网络/5xx 不阻断保存）；否则返回错误拒绝保存。
func validateTurnstileSiteKeyAPI(sitekey, token, accountID string) error {
	sitekey = strings.TrimSpace(sitekey)
	if !validTurnstileSiteKey(sitekey) {
		return errors.New("Turnstile 站点公钥格式不正确，应以 0x 开头且长度不小于 16")
	}
	if strings.TrimSpace(token) == "" || strings.TrimSpace(accountID) == "" {
		return errors.New("保存公钥前请先在后台配置 Cloudflare API Token 和账号 ID")
	}

	u := "https://api.cloudflare.com/client/v4/accounts/" + url.PathEscape(strings.TrimSpace(accountID)) + "/challenges/widgets/" + url.PathEscape(sitekey)
	req, err := http.NewRequest("GET", u, nil)
	if err != nil {
		return nil
	}
	req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(token))

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		// 网络异常无法确认，不阻断保存
		return nil
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 64<<10))

	var apiResp struct {
		Success bool `json:"success"`
		Errors  []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}
	_ = json.Unmarshal(body, &apiResp)

	if resp.StatusCode == http.StatusOK && apiResp.Success {
		return nil // 公钥存在
	}
	if resp.StatusCode >= 500 {
		return nil // Cloudflare 服务端临时错误，不阻断保存
	}

	msg := "Turnstile 站点公钥校验失败，请检查公钥、API Token 与账号 ID"
	if len(apiResp.Errors) > 0 && strings.TrimSpace(apiResp.Errors[0].Message) != "" {
		msg += "：" + apiResp.Errors[0].Message
	}
	return errors.New(msg)
}

// turnstileConfigured 判定 Turnstile 是否实际可用：
// 公钥格式合法、密钥非空，且服务端未观测到配置错误。
func turnstileConfigured(s models.Settings) bool {
	if strings.TrimSpace(s.TurnstileSecret) == "" {
		return false
	}
	if !validTurnstileSiteKey(s.TurnstileSiteKey) {
		return false
	}
	if turnstileBrokenNow() {
		return false
	}
	return true
}

// effectiveVerifyMethodWithFallback 返回实际生效的验证方式。
// 仅在服务端可确认 Turnstile 不可用（未配置 / 公钥明显错误 / 密钥配置错误已被观测到）
// 时回退到 cap-pow 本地验证。
func effectiveVerifyMethodWithFallback(s models.Settings) string {
	method := models.EffectiveVerifyMethod(s)
	if method == models.VerifyMethodPOW && s.PowLocalFallback {
		return models.VerifyMethodCappow
	}
	if method == models.VerifyMethodTurnstile && !turnstileConfigured(s) {
		return models.VerifyMethodCappow
	}
	return method
}

// verifyHuman 按当前验证方式校验人机验证 token。
// ok=false 时 errMsg 非空；fallback 非空表示该验证方式不可用，应改用该方式（当前为 cappow）。
// 回退完全由服务端判定，不信任客户端输入。
func verifyHuman(c *gin.Context, setting models.Settings, powToken, turnstileToken, capToken string) (ok bool, errMsg, fallback string) {
	switch effectiveVerifyMethodWithFallback(setting) {
	case models.VerifyMethodTurnstile:
		if turnstileToken == "" {
			return false, "请完成人机验证", ""
		}
		secret := strings.TrimSpace(setting.TurnstileSecret)
		if secureconfig.IsEncryptedValue(secret) {
			if decrypted, err := secureconfig.DecryptSettingValue("turnstile_secret_key", secret); err == nil {
				secret = strings.TrimSpace(decrypted)
			}
		}
		success, errCodes := ValidateTurnstileToken(turnstileToken, secret, c.ClientIP())
		if !success {
			if hasTurnstileConfigError(errCodes) {
				// 服务端确认密钥配置错误 → 记录并回退到本地验证
				markTurnstileBroken()
				return false, "人机验证配置异常，请稍后重试", models.VerifyMethodCappow
			}
			return false, "人机验证失败，请重试", ""
		}
		return true, "", ""
	case models.VerifyMethodCappow:
		if capToken == "" {
			if models.EffectiveVerifyMethod(setting) == models.VerifyMethodPOW && setting.PowLocalFallback {
				return false, "请完成本地人机验证", models.VerifyMethodCappow
			}
			return false, "请完成人机验证", ""
		}
		if !cappowStore.ConsumeToken(capToken) {
			return false, "人机验证失败，请重试", ""
		}
		return true, "", ""
	case models.VerifyMethodPOW:
		if powToken == "" {
			return false, "请输入pow token", ""
		}
		if !ValidatePowTokenWithSettings(c.Request.Context(), setting, powToken) {
			return false, "pow token验证失败", ""
		}
		return true, "", ""
	default: // VerifyMethodNone
		return true, "", ""
	}
}

// hasTurnstileConfigError 判断 siteverify 失败是否源于 Turnstile 密钥配置错误。
func hasTurnstileConfigError(errCodes []string) bool {
	for _, code := range errCodes {
		switch code {
		case "invalid-input-secret", "missing-input-secret":
			return true
		}
	}
	return false
}

// ValidateTurnstileToken 向 Cloudflare Turnstile siteverify 校验前端 token。
// 返回是否通过及 Cloudflare 提供的 error-codes（用于区分配置错误与真实失败）。
func ValidateTurnstileToken(token, secret, remoteIP string) (bool, []string) {
	if token == "" || secret == "" {
		return false, []string{"invalid-input-secret"}
	}

	form := url.Values{}
	form.Set("secret", secret)
	form.Set("response", token)
	if strings.TrimSpace(remoteIP) != "" {
		form.Set("remoteip", remoteIP)
	}

	req, err := http.NewRequest("POST", "https://challenges.cloudflare.com/turnstile/v0/siteverify", strings.NewReader(form.Encode()))
	if err != nil {
		return false, nil
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return false, nil
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return false, nil
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
	if err != nil {
		return false, nil
	}
	var validationResp struct {
		Success    bool     `json:"success"`
		ErrorCodes []string `json:"error-codes"`
	}
	if err := json.Unmarshal(body, &validationResp); err != nil {
		return false, nil
	}
	return validationResp.Success, validationResp.ErrorCodes
}

// CapPowChallenge 生成一个 cap-pow 挑战，供前端 cap-widget 拉取。
func CapPowChallenge(c *gin.Context) {
	settingModel, err := settings.GetSettings()
	if err != nil {
		c.JSON(http.StatusInternalServerError, result.Error(500, "获取设置失败"))
		return
	}

	difficulty := settingModel.CappowDifficulty
	if difficulty < 1 || difficulty > cappow.MaxChallengeDifficulty {
		difficulty = 4
	}

	res, err := cappow.GenerateChallenge(cappowSecret(), difficulty, 50, 32, 0)
	if err != nil {
		c.JSON(http.StatusInternalServerError, result.Error(500, "生成验证挑战失败"))
		return
	}
	c.JSON(http.StatusOK, res)
}

// CapPowRedeem 校验前端提交的谜题解答，成功则签发一次性 redeem token。
// 同时消费挑战签名，防止同一挑战被重复兑换。
func CapPowRedeem(c *gin.Context) {
	var req struct {
		Token     string `json:"token"`
		Solutions []int  `json:"solutions"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "请求参数错误", "success": false, "error": "invalid_body"})
		return
	}

	secret := cappowSecret()
	ok, _ := cappow.ValidateChallenge(secret, req.Token, req.Solutions)
	if !ok {
		c.JSON(http.StatusForbidden, gin.H{"code": 403, "message": "人机验证失败，请重试", "success": false})
		return
	}

	// 一次性挑战重放防护：消费失败说明该挑战已被兑换过。
	if !cappowStore.ConsumeChallengeSig(cappow.JwtSigHex(req.Token), 0) {
		c.JSON(http.StatusForbidden, gin.H{"code": 403, "message": "人机验证失败，请重试", "success": false})
		return
	}

	token, expires := cappowStore.IssueToken(0)
	c.JSON(http.StatusOK, gin.H{"success": true, "token": token, "expires": expires})
}
