package auth

import (
	"net/http"
	"strings"

	"oneimg/backend/config"
	"oneimg/backend/models"
	"oneimg/backend/utils/cappow"
	"oneimg/backend/utils/result"
	"oneimg/backend/utils/secureconfig"
	"oneimg/backend/utils/settings"
	"oneimg/backend/utils/verifypolicy"

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

// turnstileConfigured 判定 Turnstile 是否实际可用。
// 状态与判据都住在 utils/verifypolicy —— middlewares 生成 CSP 时也要用同一份，
// 两边各写一套必然走样（曾导致回退后用不了 wasm）。
func turnstileConfigured(s models.Settings) bool {
	return verifypolicy.TurnstileUsable(s)
}

// validTurnstileSiteKey 宽松校验站点公钥格式（0x 前缀 + 足够长度）。
// 判据在 models，便于 CSP 侧复用同一份。
func validTurnstileSiteKey(k string) bool {
	return models.ValidTurnstileSiteKey(k)
}

// EffectiveVerifyMethod 返回最终实际生效的人机验证方式（含回退）。
// 实现在 utils/verifypolicy —— middlewares 生成 CSP 时也要用同一份。
func EffectiveVerifyMethod(s models.Settings) string {
	return verifypolicy.EffectiveMethod(s)
}

// verifyHuman 按当前验证方式校验人机验证 token。
// ok=false 时 errMsg 非空；fallback 非空表示该验证方式不可用，应改用该方式（当前为 cappow）。
// 回退完全由服务端判定，不信任客户端输入。
func verifyHuman(c *gin.Context, setting models.Settings, turnstileToken, capToken string) (ok bool, errMsg, fallback string) {
	switch EffectiveVerifyMethod(setting) {
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
		success, errCodes := verifypolicy.ValidateTurnstileToken(turnstileToken, secret, c.ClientIP())
		if !success {
			if hasTurnstileConfigError(errCodes) {
				// 服务端确认密钥配置错误 → 记录并回退到本地验证
				verifypolicy.MarkTurnstileBroken()
				return false, "人机验证配置异常，请稍后重试", models.VerifyMethodCappow
			}
			return false, "人机验证失败，请重试", ""
		}
		return true, "", ""
	case models.VerifyMethodCappow:
		if capToken == "" {
			return false, "请完成人机验证", ""
		}
		if !cappowStore.ConsumeToken(capToken) {
			return false, "人机验证失败，请重试", ""
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
