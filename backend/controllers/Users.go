package controllers

import (
	"net/http"
	"oneimg/backend/utils/authsecurity"
	"strconv"
	"strings"
	"time"

	"oneimg/backend/database"
	"oneimg/backend/models"
	"oneimg/backend/utils/result"
	"oneimg/backend/utils/settings"
	"oneimg/backend/utils/storagepolicy"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// GetUsers 分页查询用户列表（管理员）。
func GetUsers(c *gin.Context) {
	c.Header("Cache-Control", "private, no-store")
	db := database.GetDB().DB

	page, err := strconv.Atoi(c.DefaultQuery("page", "1"))
	if err != nil || page < 1 {
		page = 1
	}
	limit, err := strconv.Atoi(c.DefaultQuery("limit", "20"))
	if err != nil || limit < 1 || limit > 100 {
		limit = 20
	}

	var users []models.User
	var total int64
	query := db.Model(&users)

	if id, err := strconv.Atoi(c.DefaultQuery("id", "0")); err == nil && id > 0 {
		query = query.Where("id = ?", id)
	}
	if username := c.DefaultQuery("username", ""); username != "" {
		query = query.Where("username LIKE ?", "%"+username+"%")
	}
	if role, err := strconv.Atoi(c.DefaultQuery("role", "0")); err == nil && role > 0 {
		query = query.Where("role = ?", role)
	}

	if err := query.Count(&total).Error; err != nil {
		internalFailure(c, "查询总数失败", err)
		return
	}

	offset := (page - 1) * limit
	if err := query.Order("id DESC").Offset(offset).Limit(limit).Find(&users).Error; err != nil {
		internalFailure(c, "查询用户列表失败", err)
		return
	}

	setting, err := settings.GetSettings()
	if err != nil {
		internalFailure(c, "读取用户权限摘要失败", err)
		return
	}
	var buckets []models.Buckets
	if err := db.Select("id", "type", "disabled", "capacity", "usage").Find(&buckets).Error; err != nil {
		internalFailure(c, "读取存储可用状态失败", err)
		return
	}
	policy, err := storagepolicy.Load(db)
	if err != nil {
		internalFailure(c, "读取角色存储策略失败", err)
		return
	}
	list := make([]UserWithAccessSummary, 0, len(users))
	for _, user := range users {
		list = append(list, UserWithAccessSummary{User: user, AccessSummary: summarizeUserAccess(user, setting, buckets, policy)})
	}
	c.JSON(http.StatusOK, result.Success("查询成功", map[string]any{
		"total":              total,
		"list":               list,
		"multi_storage_sync": setting.MultiStorageSync,
	}))
}

// CreateUser 创建用户。
func CreateUser(c *gin.Context) {
	type CreateUserReq struct {
		Username string `json:"username" binding:"required,min=3,max=50"`
		Password string `json:"password" binding:"required,min=8,max=72"`
		Role     int    `json:"role" binding:"required,oneof=1 3"`
	}
	var req CreateUserReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, result.Fail(400, "参数校验失败："+err.Error()))
		return
	}

	if req.Username != strings.TrimSpace(req.Username) || isTouristUsername(req.Username) || len(req.Password) > 72 {
		c.JSON(400, result.Error(400, "用户名为保留名称或密码长度无效"))
		return
	}
	db := database.GetDB().DB

	if db.Where("username = ?", req.Username).First(&models.User{}).Error == nil {
		c.JSON(http.StatusBadRequest, result.Error(400, "用户名已存在"))
		return
	}

	if req.Role != models.RoleAdmin && req.Role != models.RoleUser {
		req.Role = models.RoleUser
	}

	hashedPwd, err := hashPassword(req.Password)
	if err != nil {
		c.JSON(http.StatusInternalServerError, result.Fail(500, "密码加密失败"))
		return
	}

	newUser := models.User{
		Username: req.Username,
		Password: hashedPwd,
		Role:     req.Role,
		Permission: models.Permission{
			Buckets: []int{},
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	if err := db.Create(&newUser).Error; err != nil {
		errMsg := err.Error()
		if strings.Contains(errMsg, "unique constraint") || strings.Contains(errMsg, "duplicate key") {
			c.JSON(http.StatusBadRequest, result.Fail(400, "该用户名已存在"))
			return
		}
		c.JSON(http.StatusInternalServerError, result.Fail(500, "创建用户失败："+errMsg))
		return
	}

	resp := map[string]any{
		"id":       newUser.ID,
		"username": newUser.Username,
		"role":     newUser.Role,
	}
	c.JSON(http.StatusOK, result.Success("创建成功", resp))
}

// DeleteUser 删除用户；外部身份保留为禁用墓碑，防止 SSO 自动重建。
func DeleteUser(c *gin.Context) {
	userIDStr := c.Param("id")
	id, err := strconv.Atoi(userIDStr)
	if err != nil || id < 1 {
		c.JSON(http.StatusBadRequest, result.Fail(400, "用户ID参数错误"))
		return
	}

	if id == models.SuperAdminID {
		c.JSON(http.StatusBadRequest, result.Fail(400, "不能删除超级管理员账号"))
		return
	}

	loginUID, _ := c.Get("user_id")
	if loginUID == id {
		c.JSON(http.StatusBadRequest, result.Fail(400, "不能删除当前登录用户"))
		return
	}

	db := database.GetDB().DB
	var user models.User
	if err := db.Where("id = ?", id).First(&user).Error; err != nil {
		c.JSON(http.StatusNotFound, result.Fail(404, "用户不存在"))
		return
	}

	if err := db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&models.ExternalIdentity{}).Where("user_id = ?", id).Updates(map[string]any{
			"user_id":  0,
			"disabled": true,
		}).Error; err != nil {
			return err
		}
		if err := authsecurity.RevokeUserSessions(tx, user.ID); err != nil {
			return err
		}
		if err := tx.Model(&models.APICredential{}).Where("owner_id = ?", user.ID).Update("revoked", true).Error; err != nil {
			return err
		}
		if err := tx.Where("user_id = ?", user.ID).Delete(&models.AuthLoginEvent{}).Error; err != nil {
			return err
		}
		return tx.Delete(&user).Error
	}); err != nil {
		internalFailure(c, "删除用户失败", err)
		return
	}

	c.JSON(http.StatusOK, result.Success("删除成功", nil))
}

// UpdateUserRole 修改用户角色（不可改超管/自身）。
func UpdateUserRole(c *gin.Context) {
	type UpdateRoleReq struct {
		ID   int `json:"id" binding:"required,min=1"`
		Role int `json:"role" binding:"required,oneof=1 3"`
	}
	var req UpdateRoleReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, result.Fail(400, "参数校验失败："+err.Error()))
		return
	}

	id := req.ID
	if id == models.SuperAdminID {
		c.JSON(http.StatusBadRequest, result.Fail(400, "不能修改超级管理员角色"))
		return
	}

	loginUID, _ := c.Get("user_id")
	if loginUID == id {
		c.JSON(http.StatusBadRequest, result.Fail(400, "不能修改当前登录用户角色"))
		return
	}

	db := database.GetDB().DB
	var user models.User
	if err := db.Where("id = ?", id).First(&user).Error; err != nil {
		c.JSON(http.StatusNotFound, result.Fail(404, "用户不存在"))
		return
	}

	if err := db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&user).Update("role", req.Role).Error; err != nil {
			return err
		}
		return authsecurity.RevokeUserSessions(tx, user.ID)
	}); err != nil {
		internalFailure(c, "更新角色失败", err)
		return
	}

	c.JSON(http.StatusOK, result.Success("更新成功", nil))
}

// ResetPassword 重置用户密码并返回明文新密码（仅管理员操作）。
func ResetPassword(c *gin.Context) {
	userIDStr := c.Param("id")
	id, err := strconv.Atoi(userIDStr)
	if err != nil || id < 1 {
		c.JSON(http.StatusBadRequest, result.Fail(400, "用户ID参数错误"))
		return
	}

	if id == models.SuperAdminID {
		c.JSON(http.StatusBadRequest, result.Fail(400, "不能重置超级管理员密码"))
		return
	}

	// 禁止重置自身密码
	loginUID, _ := c.Get("user_id")
	if loginUID == id {
		c.JSON(http.StatusBadRequest, result.Fail(400, "不能重置当前登录用户密码"))
		return
	}

	db := database.GetDB().DB
	var user models.User
	if err := db.Where("id = ?", id).First(&user).Error; err != nil {
		c.JSON(http.StatusNotFound, result.Fail(404, "用户不存在"))
		return
	}

	// 生成12位友好随机密码
	newPassword, err := authsecurity.RandomSecret(18)
	if err != nil {
		c.JSON(500, result.Error(500, "随机密码生成失败"))
		return
	}
	hashedPwd, err := hashPassword(newPassword)
	if err != nil {
		c.JSON(http.StatusInternalServerError, result.Fail(500, "密码加密失败"))
		return
	}

	if err := db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&user).Update("password", hashedPwd).Error; err != nil {
			return err
		}
		if err := authsecurity.RevokeUserSessions(tx, user.ID); err != nil {
			return err
		}
		return tx.Model(&models.APICredential{}).Where("owner_id = ?", user.ID).Update("revoked", true).Error
	}); err != nil {
		internalFailure(c, "重置密码失败", err)
		return
	}

	c.JSON(http.StatusOK, result.Success("密码重置成功", map[string]any{
		"new_password": newPassword,
	}))
}

// UpdateUserPermission 更新用户权限
func UpdateUserPermission(c *gin.Context) {
	userIDStr := c.Param("id")
	id, err := strconv.Atoi(userIDStr)
	if err != nil || id < 1 {
		c.JSON(http.StatusBadRequest, result.Fail(400, "用户ID参数错误"))
		return
	}

	type UpdatePermissionReq struct {
		Permission []int    `json:"permission"`
		Codes      []string `json:"codes"`
	}
	var req UpdatePermissionReq

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, result.Fail(400, "参数校验失败："+err.Error()))
		return
	}

	setting, err := settings.GetSettings()
	if err != nil {
		c.JSON(http.StatusInternalServerError, result.Fail(500, "读取多存储设置失败"))
		return
	}
	if !setting.MultiStorageSync {
		if id == models.SuperAdminID {
			c.JSON(http.StatusBadRequest, result.Fail(400, "不能修改超级管理员权限"))
			return
		}

		if c.GetInt("user_id") == id {
			c.JSON(http.StatusBadRequest, result.Fail(400, "不能修改当前登录用户权限"))
			return
		}
	}

	db := database.GetDB().DB
	var user models.User
	if err := db.Where("id = ?", id).First(&user).Error; err != nil {
		c.JSON(http.StatusNotFound, result.Fail(404, "用户不存在"))
		return
	}

	var uniquePermissions []int
	if req.Permission != nil {
		uniquePermissions = make([]int, 0, len(req.Permission))
		seenBuckets := make(map[int]struct{}, len(req.Permission))
		for _, bucketID := range req.Permission {
			if bucketID <= 0 {
				c.JSON(http.StatusBadRequest, result.Fail(400, "存储源ID无效"))
				return
			}
			if _, exists := seenBuckets[bucketID]; exists {
				continue
			}
			seenBuckets[bucketID] = struct{}{}
			uniquePermissions = append(uniquePermissions, bucketID)
		}
		if len(uniquePermissions) > 0 {
			var bucketCount int64
			bucketQuery := db.Model(&models.Buckets{}).Where("id IN ?", uniquePermissions)
			invalidMessage := "包含不存在的存储源"
			if setting.MultiStorageSync {
				bucketQuery = bucketQuery.Where("type <> ?", "default")
				invalidMessage = "同步存储源必须存在且不能是本地默认存储"
			}
			if err := bucketQuery.Count(&bucketCount).Error; err != nil {
				c.JSON(http.StatusInternalServerError, result.Fail(500, "校验存储源失败"))
				return
			}
			if bucketCount != int64(len(uniquePermissions)) {
				c.JSON(http.StatusBadRequest, result.Fail(400, invalidMessage))
				return
			}
		}
	}

	var validCodes []string
	if req.Codes != nil {
		validCodes = models.FilterValidPermissionCodes(req.Codes)
	}

	if err := db.Transaction(func(tx *gorm.DB) error {
		// Reload under the row lock, then merge only supplied fields. Independent
		// storage/ordinary-permission editors must preserve concurrent edits.
		var current models.User
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&current, id).Error; err != nil {
			return err
		}
		permission := current.Permission
		if req.Permission != nil {
			permission.Buckets = uniquePermissions
		}
		if req.Codes != nil {
			permission.Codes = validCodes
		}
		if permission.Codes == nil {
			permission.Codes = []string{}
		}
		if permission.Buckets == nil {
			permission.Buckets = []int{}
		}
		if err := tx.Model(&current).Update("permission", permission).Error; err != nil {
			return err
		}
		return authsecurity.RevokeUserSessions(tx, current.ID)
	}); err != nil {
		internalFailure(c, "更新权限失败", err)
		return
	}

	message := "更新成功"
	if setting.MultiStorageSync {
		message = "同步存储源更新成功"
	}
	c.JSON(http.StatusOK, result.Success(message, nil))
}

// hashPassword 使用 bcrypt 加密密码。
func hashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(hash), err
}
