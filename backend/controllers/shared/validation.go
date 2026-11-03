package shared

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"

	"oneimg/backend/utils/result"
)

// 请求体校验错误的翻译层。
//
// 为什么需要：gin 默认把 validator 的原始错误直接拼进响应，于是客户端会看到
//
//	参数校验失败：Key: 'CreateUserReq.Password' Error:Field validation for
//	'Password' failed on the 'min' tag
//
// 这句话对终端用户毫无意义 —— 它暴露了 Go 结构体名、字段名和 validator 的
// 内部 tag，却没有说「密码不能少于 8 位」这个唯一有用的信息。
//
// 这里把 (字段, tag) 翻译成中文业务提示：
//   - 字段名优先按结构体字段名查中文表，未登记的走通用句式；
//   - 每个常用字段的每个约束在 fieldRules 里给出具体文案；
//   - 完全不认识的约束也输出「xx 不符合要求」，
//     绝不会把 Key:/Error:/tag 这些内部细节漏给客户端。

// fieldLabels 把结构体字段名映射成面向用户的中文名。
var fieldLabels = map[string]string{
	"Username":        "用户名",
	"Password":        "密码",
	"Role":            "角色",
	"ID":              "用户 ID",
	"Key":             "设置项",
	"Value":           "设置内容",
	"Scope":           "范围",
	"CurrentPassword": "当前密码",
	"NewPassword":     "新密码",
	"NewUsername":     "新用户名",
	"Enabled":         "启用状态",
	"BucketID":        "存储源",
	"ImageIDs":        "图片列表",
	"Urls":            "图片地址",
	"QuotaBytes":      "配额",
	"FolderID":        "文件夹",
	"Name":            "名称",
	"Title":           "标题",
}

// fieldRules 为特定字段的特定约束提供贴切文案，键为「字段名.约束名」。
//
// 通用句式只能做到「密码长度不足」，而真实约束很具体（8-72 位），
// 写出来用户才知道该怎么改。
var fieldRules = map[string]string{
	"Username.required":        "请填写用户名",
	"Username.min":             "用户名不能少于 3 个字符",
	"Username.max":             "用户名不能超过 50 个字符",
	"Password.required":        "请填写密码",
	"Password.min":             "密码不能少于 8 位",
	"Password.max":             "密码不能超过 72 位",
	"CurrentPassword.required": "请填写当前密码",
	"CurrentPassword.max":      "当前密码不能超过 72 位",
	"NewPassword.max":          "新密码不能超过 72 位",
	"NewUsername.max":          "新用户名不能超过 50 个字符",
	"Role.required":            "请选择角色",
	"Role.oneof":               "角色只能是管理员或普通用户",
	"ID.required":              "请指定用户",
	"ID.min":                   "用户 ID 无效",
	"Key.required":             "请指定要修改的设置项",
	"Value.required":           "请填写设置内容",
	"Scope.required":           "请选择范围",
	"Scope.oneof":              "范围只能是全部会话或其它会话",
	"Enabled.required":         "请指定启用状态",
	"BucketID.required":        "请选择存储源",
	"ImageIDs.required":        "请选择图片",
	"Urls.required":            "请填写图片地址",
	"Name.required":            "请填写名称",
}

// tagFallback 是没有专属规则时的通用句式。
var tagFallback = map[string]string{
	"required": "不能为空",
	"min":      "长度不足",
	"max":      "长度超出限制",
	"oneof":    "取值不在允许范围内",
	"email":    "格式不正确",
	"url":      "必须是合法链接",
	"numeric":  "必须是数字",
	"gte":      "数值过小",
	"lte":      "数值过大",
	"len":      "长度不符合要求",
}

func fieldDisplayName(field string) string {
	if name, ok := fieldLabels[field]; ok {
		return name
	}
	return field
}

// translateValidationError 把绑定/校验错误转成一句中文。
// 返回空字符串表示无法识别，调用方应使用自带的兜底文案
// （而不是把原始错误透出去）。
func translateValidationError(err error) string {
	if err == nil {
		return ""
	}

	// JSON 类型不匹配（例如 "role": "abc" 期望数字）。
	// 这类错误带 Field 但没有 validator 的 tag，要单独处理。
	var typeErr *json.UnmarshalTypeError
	if errors.As(err, &typeErr) {
		if typeErr.Field == "" {
			return "请求内容格式不正确"
		}
		return fieldDisplayName(typeErr.Field) + "的格式不正确"
	}

	// 请求体不是合法 JSON。json.SyntaxError 的 Error() 只含位置信息，
	// 不含任何业务细节，可以安全地给一句固定文案。
	var syntaxErr *json.SyntaxError
	if errors.As(err, &syntaxErr) {
		return "请求内容不是合法的 JSON"
	}

	var fieldErrors validator.ValidationErrors
	if errors.As(err, &fieldErrors) {
		return translateFieldErrors(fieldErrors)
	}

	return ""
}

// translateFieldErrors 只报第一条字段错误。
//
// 表单一次通常只改一个字段；列出全部会让提示变成长篇，
// 而用户修正第一条后仍会被第二条挡住 —— 逐条报更好用。
func translateFieldErrors(errs validator.ValidationErrors) string {
	if len(errs) == 0 {
		return ""
	}
	first := errs[0]
	if message, ok := fieldRules[first.Field()+"."+first.Tag()]; ok {
		return message
	}
	name := fieldDisplayName(first.Field())
	if fallback, ok := tagFallback[first.Tag()]; ok {
		return name + fallback
	}
	return name + "不符合要求"
}

// RespondValidationError 是「绑定失败」的统一出口。
//
// fallback 用于无法识别的情形（如空请求体、读 body 出错）——
// 由调用方给出具体文案。
func RespondValidationError(c *gin.Context, err error, fallback string) {
	if message := translateValidationError(err); message != "" {
		c.JSON(http.StatusBadRequest, result.Fail(http.StatusBadRequest, message))
		return
	}
	if strings.TrimSpace(fallback) == "" {
		fallback = "请求参数不正确"
	}
	c.JSON(http.StatusBadRequest, result.Fail(http.StatusBadRequest, fallback))
}

// BindJSON 是 c.ShouldBindJSON 的替代品：失败时自动回中文提示并返回 false。
//
// 用它统一收口，避免每个控制器各写一份 "参数校验失败："+err.Error() ——
// 那种写法正是内部错误外泄的来源（会带上结构体名与 validator tag）。
func BindJSON(c *gin.Context, target any) bool {
	if err := c.ShouldBindJSON(target); err != nil {
		RespondValidationError(c, err, "请求参数不正确，请检查后重试")
		return false
	}
	return true
}
