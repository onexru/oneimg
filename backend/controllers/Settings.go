package controllers

// 定义请求参数
type UpdateSettingsRequest struct {
	Key   string `json:"key" binding:"required"`
	Value any    `json:"value"`
}

// 自定义查询参数
type GetSettingsRequest struct {
	Keys []string `json:"keys"`
}
