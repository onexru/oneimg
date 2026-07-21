package result

// Result 统一 API 响应结构。
type Result struct {
	Code int    `json:"code"`
	Msg  string `json:"message"`
	Data any    `json:"data"`
}

// Success 返回成功响应。
func Success(msg string, data any) *Result {
	return &Result{
		Code: 200,
		Msg:  msg,
		Data: data,
	}
}

// Fail 返回失败响应（与 Error 等价，保留以兼容既有调用）。
func Fail(code int, msg string) *Result {
	return &Result{
		Code: code,
		Msg:  msg,
		Data: nil,
	}
}

// Error 返回错误响应（与 Fail 等价，保留以兼容既有调用）。
func Error(code int, msg string) *Result {
	return &Result{
		Code: code,
		Msg:  msg,
		Data: nil,
	}
}
