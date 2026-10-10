package controllers

import (
	"errors"
	"io"
	"net/http"
	"strconv"

	"oneimg/backend/database"
	"oneimg/backend/models"
	"oneimg/backend/services"
	"oneimg/backend/utils/result"

	"github.com/gin-gonic/gin"
)

func folderHTTPError(c *gin.Context, err error) {
	var e *services.GalleryFolderError
	if errors.As(err, &e) {
		c.AbortWithStatusJSON(e.Status, gin.H{"code": e.Status, "message": e.Message, "error_code": e.Code})
		return
	}
	internalFailure(c, "文件夹操作失败，请稍后重试", err)
}
func requireFolderAccount(c *gin.Context) bool {
	c.Header("Cache-Control", "private, no-store")
	if c.GetInt("user_id") <= 0 || c.GetInt("user_role") == models.RoleGuest {
		folderHTTPError(c, &services.GalleryFolderError{Status: 403, Code: "folders_unavailable", Message: "请登录后使用文件夹"})
		return false
	}
	return true
}
func parseFolderID(raw string, allowZero bool) (int, error) {
	// Reject signs, spaces, fractions and overflow rather than guessing intent.
	valid := raw != ""
	for _, ch := range raw {
		if ch < '0' || ch > '9' {
			valid = false
		}
	}
	id, err := strconv.Atoi(raw)
	if !valid || err != nil || id < 0 || (!allowZero && id == 0) {
		return 0, &services.GalleryFolderError{Status: 400, Code: "invalid_folder_id", Message: "文件夹ID无效"}
	}
	return id, nil
}
func validateUploadFolder(c *gin.Context, folderID int) bool {
	err := services.ValidateFolderSelection(database.GetDB().DB.WithContext(c.Request.Context()), c.GetInt("user_id"), c.GetInt("user_role"), folderID)
	if err != nil {
		folderHTTPError(c, err)
		return false
	}
	return true
}
func multipartUploadFolder(c *gin.Context) (int, bool) {
	values := c.Request.MultipartForm.Value["folder_id"]
	if len(values) == 0 {
		return 0, true
	}
	if len(values) != 1 {
		folderHTTPError(c, &services.GalleryFolderError{Status: 400, Code: "invalid_folder_id", Message: "只能指定一个文件夹"})
		return 0, false
	}
	id, err := parseFolderID(values[0], true)
	if err != nil {
		folderHTTPError(c, err)
		return 0, false
	}
	return id, validateUploadFolder(c, id)
}
func GetFolders(c *gin.Context) {
	if !requireFolderAccount(c) {
		return
	}
	r, err := services.ListGalleryFolders(database.GetDB().DB.WithContext(c.Request.Context()), c.GetInt("user_id"))
	if err != nil {
		folderHTTPError(c, err)
		return
	}
	c.JSON(http.StatusOK, result.Success("ok", r))
}
func CreateFolder(c *gin.Context) { writeFolder(c, false) }
func UpdateFolder(c *gin.Context) { writeFolder(c, true) }
func writeFolder(c *gin.Context, rename bool) {
	if !requireFolderAccount(c) {
		return
	}
	var id int
	var err error
	if rename {
		id, err = parseFolderID(c.Param("id"), false)
		if err != nil {
			folderHTTPError(c, err)
			return
		}
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 8<<10)
	var req struct {
		Name        string  `json:"name"`
		Description *string `json:"description"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, result.Error(400, "文件夹参数格式错误"))
		return
	}
	db := database.GetDB().DB.WithContext(c.Request.Context())
	descriptions := []string{}
	if req.Description != nil {
		descriptions = append(descriptions, *req.Description)
	}
	var r services.GalleryFolderResponse
	if rename {
		r, err = services.RenameGalleryFolder(db, c.GetInt("user_id"), id, req.Name, descriptions...)
	} else {
		r, err = services.CreateGalleryFolder(db, c.GetInt("user_id"), req.Name, descriptions...)
	}
	if err != nil {
		folderHTTPError(c, err)
		return
	}
	c.JSON(http.StatusOK, result.Success("ok", r))
}
func DeleteFolder(c *gin.Context) {
	if !requireFolderAccount(c) {
		return
	}
	id, err := parseFolderID(c.Param("id"), false)
	if err != nil {
		folderHTTPError(c, err)
		return
	}
	var req struct {
		DeleteImages bool   `json:"delete_images"`
		ConfirmName  string `json:"confirm_name"`
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 8<<10)
	if err := c.ShouldBindJSON(&req); err != nil && !errors.Is(err, io.EOF) {
		c.JSON(400, result.Error(400, "删除参数格式错误"))
		return
	}
	if req.DeleteImages {
		r, err := services.DeleteGalleryFolderWithImages(c.Request.Context(), database.GetDB().DB, c.GetInt("user_id"), id, req.ConfirmName)
		if err != nil {
			folderHTTPError(c, err)
			return
		}
		if !r.Deleted {
			status, code, message := 409, "folder_delete_incomplete", "删除尚未完成，请继续删除剩余图片"
			if r.FailedCount > 0 {
				status, code, message = 502, "folder_delete_failed", "部分图片删除失败，已保留记录，请重试"
			}
			c.JSON(status, gin.H{"code": status, "message": message, "error_code": code, "data": r})
			return
		}
		c.JSON(200, result.Success("ok", r))
		return
	}
	if err := services.DeleteGalleryFolder(database.GetDB().DB.WithContext(c.Request.Context()), c.GetInt("user_id"), id); err != nil {
		folderHTTPError(c, err)
		return
	}
	c.JSON(http.StatusOK, result.Success("ok", gin.H{}))
}
func MoveImagesFolder(c *gin.Context) {
	if !requireFolderAccount(c) {
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 16<<10)
	var req struct {
		ImageIDs []int `json:"image_ids"`
		FolderID *int  `json:"folder_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.FolderID == nil {
		c.JSON(400, result.Error(400, "图片或文件夹参数格式错误"))
		return
	}
	moved, err := services.MoveImagesToGalleryFolder(database.GetDB().DB.WithContext(c.Request.Context()), c.GetInt("user_id"), req.ImageIDs, *req.FolderID)
	if err != nil {
		folderHTTPError(c, err)
		return
	}
	c.JSON(http.StatusOK, result.Success("ok", gin.H{"moved_count": moved}))
}
