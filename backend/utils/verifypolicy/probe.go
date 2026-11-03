// Package verifypolicy 的 HTTP 客户端部分：向 Cloudflare 发起
// siteverify 与 widget 查询。
//
// 这三个函数原本在 controllers/auth，但它们
//   - 只依赖 net/http 与 models，不碰 gin 上下文；
//   - 被设置页的「保存前校验」和登录/注册的「运行时校验」共同需要。
// 放在 utils 层，两侧都能用，且不会让 system 包反向依赖 auth 包。
package verifypolicy

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"oneimg/backend/models"
)

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

// ProbeTurnstileSecret 在保存 Turnstile 密钥配置时发起一次模拟 siteverify 请求：
// 使用无效 token，若 Cloudflare 返回 invalid/missing-input-secret 则说明密钥未被识别，
// 判定配置无效并拒绝保存；网络异常或密钥被识别（invalid-input-response 等）则放行。
func ProbeTurnstileSecret(secret string) error {
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

// ValidateTurnstileSiteKeyAPI 通过 Cloudflare 管理 API 校验站点公钥是否真实存在。
// 返回 nil 表示公钥有效或暂时无法验证（网络/5xx 不阻断保存）；否则返回错误拒绝保存。
func ValidateTurnstileSiteKeyAPI(sitekey, token, accountID string) error {
	sitekey = strings.TrimSpace(sitekey)
	if !models.ValidTurnstileSiteKey(sitekey) {
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
