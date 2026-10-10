package controllers

import (
	"errors"
	"net/http"
	"strings"

	"oneimg/backend/config"
	"oneimg/backend/models"
	"oneimg/backend/services"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// DirectUploadGuard keeps task responses (especially short-lived upload URLs)
// out of caches and makes this first version explicitly non-guest.
func DirectUploadGuard() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Cache-Control", "private, no-store")
		if c.GetInt("user_id") <= 0 || c.GetInt("user_role") == models.RoleGuest {
			directUploadHTTPError(c, http.StatusForbidden, "direct_upload_unavailable", "游客请使用普通上传", false)
			return
		}
		if c.Request.Method != http.MethodGet && c.Request.Method != http.MethodHead {
			origin := strings.TrimRight(strings.TrimSpace(c.GetHeader("Origin")), "/")
			if origin != "" && (config.App == nil || origin != strings.TrimRight(config.App.AppURL, "/")) {
				directUploadHTTPError(c, http.StatusForbidden, "origin_rejected", "请求来源不允许", false)
				return
			}
		}
		if id := c.Param("id"); id != "" {
			if _, err := uuid.Parse(id); err != nil {
				directUploadHTTPError(c, http.StatusBadRequest, "invalid_task_id", "上传任务编号无效", false)
				return
			}
		}
		c.Next()
	}
}

func directUploadHTTPError(c *gin.Context, status int, code, message string, retryable bool) {
	c.AbortWithStatusJSON(status, gin.H{"code": status, "message": message, "error_code": code, "retryable": retryable})
}

func respondDirectUpload(c *gin.Context, data any, err error) {
	if err != nil {
		var taskError *services.DirectUploadError
		if errors.As(err, &taskError) {
			directUploadHTTPError(c, taskError.Status, taskError.Code, taskError.Message, taskError.Retryable)
			return
		}
		// SDK errors can contain presigned URLs or credentials. Never serialize them.
		directUploadHTTPError(c, http.StatusInternalServerError, "upload_service_error", "上传服务暂时不可用，请稍后重试", true)
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": http.StatusOK, "message": "ok", "data": data})
}

func CreateDirectUpload(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 32<<10)
	var req services.CreateDirectUploadRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		directUploadHTTPError(c, http.StatusBadRequest, "invalid_upload_request", "上传参数无效", false)
		return
	}
	data, err := services.CreateDirectUpload(c.Request.Context(), c.GetInt("user_id"), c.GetInt("user_role"), req)
	respondDirectUpload(c, data, err)
}

func ListDirectUploads(c *gin.Context) {
	data, err := services.ListDirectUploads(c.Request.Context(), c.GetInt("user_id"))
	respondDirectUpload(c, gin.H{"tasks": data}, err)
}
func GetDirectUpload(c *gin.Context) {
	data, err := services.GetDirectUpload(c.Request.Context(), c.GetInt("user_id"), c.Param("id"))
	respondDirectUpload(c, data, err)
}
func SignDirectUpload(c *gin.Context) {
	data, err := services.SignDirectUpload(c.Request.Context(), c.GetInt("user_id"), c.Param("id"))
	respondDirectUpload(c, data, err)
}
func CompleteDirectUpload(c *gin.Context) {
	data, err := services.CompleteDirectUpload(c.Request.Context(), c.GetInt("user_id"), c.Param("id"))
	respondDirectUpload(c, data, err)
}
func RetryDirectUpload(c *gin.Context) {
	data, err := services.RetryDirectUpload(c.Request.Context(), c.GetInt("user_id"), c.Param("id"))
	respondDirectUpload(c, data, err)
}
func CancelDirectUpload(c *gin.Context) {
	data, err := services.CancelDirectUpload(c.Request.Context(), c.GetInt("user_id"), c.Param("id"))
	respondDirectUpload(c, data, err)
}
func FallbackDirectUpload(c *gin.Context) {
	// Reject foreign/missing tasks before accepting or buffering their file body.
	if _, err := services.GetDirectUpload(c.Request.Context(), c.GetInt("user_id"), c.Param("id")); err != nil {
		respondDirectUpload(c, nil, err)
		return
	}
	// Bound multipart parsing before FormFile can spool arbitrary data to disk.
	// The service additionally enforces the task's exact original size/hash.
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, services.DirectUploadMaxBytes+(1<<20))
	if err := c.Request.ParseMultipartForm(1 << 20); err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			directUploadHTTPError(c, http.StatusRequestEntityTooLarge, "file_too_large", "中转上传文件超过允许大小", false)
		} else {
			directUploadHTTPError(c, http.StatusBadRequest, "invalid_upload_body", "中转文件传输不完整，请重新上传", true)
		}
		return
	}
	if c.Request.MultipartForm != nil {
		defer c.Request.MultipartForm.RemoveAll()
	}
	if c.Request.MultipartForm == nil { directUploadHTTPError(c,http.StatusBadRequest,"invalid_upload_body","必须提供 multipart 文件",false);return }
	files := c.Request.MultipartForm.File["file"]
	if len(files) != 1 {
		directUploadHTTPError(c, http.StatusBadRequest, "invalid_upload_body", "每个上传任务必须包含一个文件", false)
		return
	}
	file, err := files[0].Open()
	if err != nil {
		directUploadHTTPError(c, http.StatusBadRequest, "invalid_upload_body", "无法读取中转文件，请重试", true)
		return
	}
	defer file.Close()
	data, err := services.FallbackDirectUpload(c.Request.Context(), c.GetInt("user_id"), c.Param("id"), file)
	respondDirectUpload(c, data, err)
}
