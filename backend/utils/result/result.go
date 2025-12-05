package result

/**
 * @Description: 统一返回结果
 **/

type Result struct {
	Code int    `json:"code"`
	Msg  string `json:"message"`
	Data any    `json:"data"`
}

func Success(msg string, data any) *Result {
	return &Result{
		Code: 200,
		Msg:  msg,
		Data: data,
	}
}

func Fail(code int, msg string) *Result {
	return &Result{
		Code: code,
		Msg:  msg,
		Data: nil,
	}
}

func Error(code int, msg string) *Result {
	return &Result{
		Code: code,
		Msg:  msg,
		Data: nil,
	}
}
