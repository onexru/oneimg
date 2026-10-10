package controllers

import (
	"bytes"
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"github.com/gin-gonic/gin"
	"io"
	"net/http"
	"net/url"
	"oneimg/backend/models"
	settingsutil "oneimg/backend/utils/settings"
	"strings"
	"time"
)

// StartCASLogin 以含 state 的精确 service URL 启动 CAS 登录。
func StartCASLogin(c *gin.Context) {
	setting, err := settingsutil.GetSettings()
	if err != nil || !casSettingsReady(setting) {
		externalAuthFailure(c, "cas", "not_configured", err)
		return
	}
	serverURL, _ := normalizeCASServerURL(setting.CASServerURL)
	serviceBase, err := casCallbackURL(setting)
	if err != nil {
		externalAuthFailure(c, "cas", "not_configured", err)
		return
	}
	state, err := randomURLToken(32)
	if err != nil {
		externalAuthFailure(c, "cas", "internal_error", err)
		return
	}
	serviceURL, err := addQueryValue(serviceBase, "state", state)
	if err != nil {
		externalAuthFailure(c, "cas", "internal_error", err)
		return
	}
	flow := models.ExternalAuthFlow{
		StateHash:  hashExternalAuthState(state),
		Provider:   "cas",
		Issuer:     serverURL,
		ServiceURL: serviceURL,
		ExpiresAt:  time.Now().Add(externalAuthFlowTTL),
	}
	if err := saveExternalAuthFlow(c, state, &flow); err != nil {
		externalAuthFailure(c, "cas", "internal_error", err)
		return
	}
	loginURL, err := casProtocolURL(serverURL, "/login", url.Values{"service": []string{serviceURL}})
	if err != nil {
		externalAuthFailure(c, "cas", "internal_error", err)
		return
	}
	c.Redirect(http.StatusFound, loginURL)
}

// CASCallback 固定使用 CAS 3.0 /p3/serviceValidate 并解析带命名空间的 XML。
func CASCallback(c *gin.Context) {
	state, stateOK := singleQueryValue(c, "state", 512)
	ticket, ticketOK := singleQueryValue(c, "ticket", 4096)
	c.Request.URL.RawQuery = ""
	if !stateOK {
		externalAuthFailure(c, "cas", "invalid_state", nil)
		return
	}
	flow, err := consumeExternalAuthFlow(c, state, "cas")
	if err != nil {
		externalAuthFailure(c, "cas", "invalid_state", err)
		return
	}
	if !ticketOK || strings.TrimSpace(ticket) == "" {
		externalAuthFailure(c, "cas", "missing_ticket", nil)
		return
	}

	setting, err := settingsutil.GetSettings()
	if err != nil || !setting.CASEnable {
		externalAuthFailure(c, "cas", "not_configured", err)
		return
	}
	currentServer, serverErr := normalizeCASServerURL(setting.CASServerURL)
	if serverErr != nil || currentServer != flow.Issuer {
		externalAuthFailure(c, "cas", "not_configured", errors.New("CAS 配置在登录过程中已变更"))
		return
	}
	casResult, err := validateCAS3Ticket(c.Request.Context(), flow.Issuer, flow.ServiceURL, ticket)
	if err != nil {
		externalAuthFailure(c, "cas", "invalid_ticket", err)
		return
	}

	username := casResult.User
	email := firstCASAttribute(casResult.Attributes, "email", "mail")
	displayName := firstCASAttribute(casResult.Attributes, "displayName", "name", "cn")
	preferredUsername := firstCASAttribute(casResult.Attributes, "preferred_username", "username", "uid")
	if preferredUsername != "" {
		username = preferredUsername
	}
	user, err := resolveExternalCallbackUser(externalIdentityProfile{
		Provider:    "cas",
		Issuer:      flow.Issuer,
		Subject:     casResult.User,
		Username:    username,
		Email:       email,
		DisplayName: displayName,
	}, casResult.User, setting.CASSuperAdminUsername, setting.CASAutoProvision)
	if err != nil {
		externalAuthFailure(c, "cas", "account_error", err)
		return
	}
	if _, err := saveUserSession(c, user); err != nil {
		externalAuthFailure(c, "cas", "internal_error", err)
		return
	}
	externalAuthSuccess(c, "cas")
}

func casProtocolURL(serverURL, endpoint string, query url.Values) (string, error) {
	parsed, err := url.Parse(serverURL)
	if err != nil {
		return "", err
	}
	parsed.Path = strings.TrimRight(parsed.Path, "/") + endpoint
	parsed.RawPath = ""
	parsed.RawQuery = query.Encode()
	return parsed.String(), nil
}

type casAuthenticationSuccess struct {
	XMLName    xml.Name
	User       casXMLValue   `xml:"user"`
	Attributes casAttributes `xml:"attributes"`
}

type casXMLValue struct {
	XMLName xml.Name
	Value   string `xml:",chardata"`
}

type casAuthenticationFailure struct {
	XMLName xml.Name
	Code    string `xml:"code,attr"`
}

type casServiceResponse struct {
	XMLName xml.Name
	Success *casAuthenticationSuccess `xml:"authenticationSuccess"`
	Failure *casAuthenticationFailure `xml:"authenticationFailure"`
}

type casAttributes map[string][]string

func (attributes *casAttributes) UnmarshalXML(decoder *xml.Decoder, start xml.StartElement) error {
	result := make(casAttributes)
	for {
		token, err := decoder.Token()
		if err != nil {
			return err
		}
		switch value := token.(type) {
		case xml.StartElement:
			var content string
			if err := decoder.DecodeElement(&content, &value); err != nil {
				return err
			}
			if value.Name.Space != casXMLNamespace {
				continue
			}
			content = strings.TrimSpace(content)
			if content != "" {
				result[value.Name.Local] = append(result[value.Name.Local], content)
			}
		case xml.EndElement:
			if value.Name == start.Name {
				*attributes = result
				return nil
			}
		}
	}
}

type casValidatedIdentity struct {
	User       string
	Attributes casAttributes
}

func validateCAS3Ticket(ctx context.Context, serverURL, serviceURL, ticket string) (casValidatedIdentity, error) {
	var validated casValidatedIdentity
	validateURL, err := casProtocolURL(serverURL, "/p3/serviceValidate", url.Values{
		"service": []string{serviceURL},
		"ticket":  []string{ticket},
	})
	if err != nil {
		return validated, err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, validateURL, nil)
	if err != nil {
		return validated, err
	}
	request.Header.Set("Accept", "application/xml, text/xml")
	response, err := externalHTTPClient(true, casMaxResponseBody).Do(request)
	if err != nil {
		return validated, fmt.Errorf("CAS3 票据校验请求失败: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return validated, fmt.Errorf("CAS3 票据校验返回 HTTP %d", response.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, casMaxResponseBody+1))
	if err != nil {
		return validated, fmt.Errorf("读取 CAS3 XML 失败: %w", err)
	}
	if int64(len(body)) > casMaxResponseBody {
		return validated, errors.New("CAS3 XML 响应超过 1 MiB")
	}
	return parseCAS3ServiceResponse(body)
}

func parseCAS3ServiceResponse(body []byte) (casValidatedIdentity, error) {
	var validated casValidatedIdentity
	upperBody := bytes.ToUpper(body)
	if bytes.Contains(upperBody, []byte("<!DOCTYPE")) || bytes.Contains(upperBody, []byte("<!ENTITY")) {
		return validated, errors.New("CAS3 XML 不允许 DTD 或自定义实体")
	}
	decoder := xml.NewDecoder(bytes.NewReader(body))
	decoder.Strict = true
	var response casServiceResponse
	if err := decoder.Decode(&response); err != nil {
		return validated, fmt.Errorf("CAS3 XML 解析失败: %w", err)
	}
	for {
		token, err := decoder.Token()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return validated, fmt.Errorf("CAS3 XML 尾部数据无效: %w", err)
		}
		switch value := token.(type) {
		case xml.CharData:
			if strings.TrimSpace(string(value)) != "" {
				return validated, errors.New("CAS3 XML 包含额外尾部数据")
			}
		case xml.Comment:
			// XML 允许根元素后存在注释。
		default:
			return validated, errors.New("CAS3 XML 包含额外根元素")
		}
	}
	if response.XMLName.Local != "serviceResponse" || response.XMLName.Space != casXMLNamespace {
		return validated, errors.New("CAS3 XML serviceResponse 命名空间无效")
	}
	if response.Success != nil && response.Failure != nil {
		return validated, errors.New("CAS3 XML 同时包含成功与失败响应")
	}
	if response.Failure != nil {
		if response.Failure.XMLName.Space != casXMLNamespace {
			return validated, errors.New("CAS3 XML authenticationFailure 命名空间无效")
		}
		return validated, errors.New("CAS3 票据校验失败")
	}
	if response.Success == nil || response.Success.XMLName.Space != casXMLNamespace {
		return validated, errors.New("CAS3 XML 缺少有效 authenticationSuccess")
	}
	if response.Success.User.XMLName.Space != casXMLNamespace {
		return validated, errors.New("CAS3 XML user 命名空间无效")
	}
	user := strings.TrimSpace(response.Success.User.Value)
	if user == "" || len(user) > 2048 {
		return validated, errors.New("CAS3 XML 缺少有效 user")
	}
	validated.User = user
	validated.Attributes = response.Success.Attributes
	return validated, nil
}

func firstCASAttribute(attributes casAttributes, keys ...string) string {
	for _, wanted := range keys {
		for key, values := range attributes {
			if !strings.EqualFold(key, wanted) {
				continue
			}
			for _, value := range values {
				if trimmed := strings.TrimSpace(value); trimmed != "" {
					return trimmed
				}
			}
		}
	}
	return ""
}
