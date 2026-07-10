package controllers

import (
	"math/rand"
	"net/http"
	"oneimg/backend/database"
	"oneimg/backend/models"
	"oneimg/backend/utils/result"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
)

// 包初始化：初始化随机种子
func init() {
	rand.Seed(time.Now().UnixNano())
}

// GetUsers 用户列表分页查询
func GetUsers(c *gin.Context) {
	db := database.GetDB().DB

	// 分页参数
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

	// ID筛选
	idStr := c.DefaultQuery("id", "0")
	id, err := strconv.Atoi(idStr)
	if err == nil && id > 0 {
		query = query.Where("id = ?", id)
	}

	// 用户名模糊搜索
	username := c.DefaultQuery("username", "")
	if username != "" {
		query = query.Where("username LIKE ?", "%"+username+"%")
	}

	// 角色筛选
	roleStr := c.DefaultQuery("role", "0")
	role, err := strconv.Atoi(roleStr)
	if err == nil && role > 0 {
		query = query.Where("role = ?", role)
	}

	// 统计总数
	if err := query.Count(&total).Error; err != nil {
		c.JSON(http.StatusInternalServerError, result.Fail(500, "查询总数失败："+err.Error()))
		return
	}

	// 分页查询
	offset := (page - 1) * limit
	if err := query.Order("id DESC").Offset(offset).Limit(limit).Find(&users).Error; err != nil {
		c.JSON(http.StatusInternalServerError, result.Fail(500, "查询用户列表失败："+err.Error()))
		return
	}

	resultData := map[string]any{
		"total": total,
		"list":  users,
	}
	c.JSON(http.StatusOK, result.Success("查询成功", resultData))
}

// CreateUser 新增用户
func CreateUser(c *gin.Context) {
	type CreateUserReq struct {
		Username string `json:"username" binding:"required,min=3,max=50"`
		Password string `json:"password" binding:"required,min=6,max=100"`
		Role     int    `json:"role" binding:"required,oneof=1 3"`
	}
	var req CreateUserReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, result.Fail(400, "参数校验失败："+err.Error()))
		return
	}

	db := database.GetDB().DB

	if db.Where("username = ?", req.Username).First(&models.User{}).Error == nil {
		c.JSON(http.StatusBadRequest, result.Error(400, "用户名已存在"))
		return
	}

	// 角色兜底
	if req.Role != models.RoleAdmin && req.Role != models.RoleUser {
		req.Role = models.RoleUser
	}

	// 密码加密
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

	// 创建用户
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

// DeleteUser 删除用户（修复未执行Delete的bug）
func DeleteUser(c *gin.Context) {
	userIDStr := c.Param("id")
	id, err := strconv.Atoi(userIDStr)
	if err != nil || id < 1 {
		c.JSON(http.StatusBadRequest, result.Fail(400, "用户ID参数错误"))
		return
	}

	// 禁止删除超级管理员
	if id == models.SuperAdminID {
		c.JSON(http.StatusBadRequest, result.Fail(400, "不能删除超级管理员账号"))
		return
	}

	// 禁止删除自身
	loginUID, _ := c.Get("user_id")
	if loginUID == id {
		c.JSON(http.StatusBadRequest, result.Fail(400, "不能删除当前登录用户"))
		return
	}

	db := database.GetDB().DB
	var user models.User
	// 校验用户存在
	if err := db.Where("id = ?", id).First(&user).Error; err != nil {
		c.JSON(http.StatusNotFound, result.Fail(404, "用户不存在"))
		return
	}

	// 执行删除
	if err := db.Delete(&user).Error; err != nil {
		c.JSON(http.StatusInternalServerError, result.Fail(500, "删除用户失败："+err.Error()))
		return
	}

	c.JSON(http.StatusOK, result.Success("删除成功", nil))
}

// UpdateUserRole 修改用户角色
func UpdateUserRole(c *gin.Context) {
	type UpdateRoleReq struct {
		ID   int `json:"id" binding:"required,min=1"`
		Role int `json:"role" binding:"required,oneof=1 3"`
	}
	var req UpdateRoleReq
	// 缺失核心绑定逻辑，已补上
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, result.Fail(400, "参数校验失败："+err.Error()))
		return
	}

	id := req.ID
	// 禁止修改超级管理员角色
	if id == models.SuperAdminID {
		c.JSON(http.StatusBadRequest, result.Fail(400, "不能修改超级管理员角色"))
		return
	}

	// 禁止修改自身角色
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

	// 更新角色
	if err := db.Model(&user).Update("role", req.Role).Error; err != nil {
		c.JSON(http.StatusInternalServerError, result.Fail(500, "更新角色失败："+err.Error()))
		return
	}

	c.JSON(http.StatusOK, result.Success("更新成功", nil))
}

// ResetPassword 重置用户密码
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
	newPassword := generateRandomSecret(12)
	hashedPwd, err := hashPassword(newPassword)
	if err != nil {
		c.JSON(http.StatusInternalServerError, result.Fail(500, "密码加密失败"))
		return
	}

	if err := db.Model(&user).Update("password", hashedPwd).Error; err != nil {
		c.JSON(http.StatusInternalServerError, result.Fail(500, "重置密码失败："+err.Error()))
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
		Permission []int `json:"permission" binding:"required"`
	}
	var req UpdatePermissionReq

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, result.Fail(400, "参数校验失败："+err.Error()))
		return
	}

	if id == models.SuperAdminID {
		c.JSON(http.StatusBadRequest, result.Fail(400, "不能修改超级管理员权限"))
		return
	}

	// 禁止修改自身权限
	loginUID, _ := c.Get("user_id")
	if loginUID == id {
		c.JSON(http.StatusBadRequest, result.Fail(400, "不能修改当前登录用户权限"))
		return
	}

	db := database.GetDB().DB
	var user models.User
	if err := db.Where("id = ?", id).First(&user).Error; err != nil {
		c.JSON(http.StatusNotFound, result.Fail(404, "用户不存在"))
		return
	}

	// 更新权限
	if err := db.Model(&user).Update("permission", models.Permission{
		Buckets: req.Permission,
	}).Error; err != nil {
		c.JSON(http.StatusInternalServerError, result.Fail(500, "更新权限失败："+err.Error()))
		return
	}

	c.JSON(http.StatusOK, result.Success("更新成功", nil))
}

// hashPassword bcrypt加密密码
func hashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(hash), err
}

// generateRandomSecret 生成指定长度纯字母数字随机密码（修复base64超长问题）
func generateRandomSecret(length int) string {
	const charset = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[rand.Intn(len(charset))]
	}
	return string(b)
}
