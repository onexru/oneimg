package users

import (
	"net/http"
	"oneimg/backend/utils/authsecurity"
	"strconv"
	"strings"
	"time"

	"oneimg/backend/database"
	"oneimg/backend/middlewares"
	"oneimg/backend/models"
	"oneimg/backend/utils/result"
	"oneimg/backend/utils/settings"
	"oneimg/backend/utils/storagepolicy"

	shared "oneimg/backend/controllers/shared"

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
		shared.InternalFailure(c, "查询总数失败", err)
		return
	}

	offset := (page - 1) * limit
	if err := query.Order("id DESC").Offset(offset).Limit(limit).Find(&users).Error; err != nil {
		shared.InternalFailure(c, "查询用户列表失败", err)
		return
	}

	setting, err := settings.GetSettings()
	if err != nil {
		shared.InternalFailure(c, "读取用户权限摘要失败", err)
		return
	}
	var buckets []models.Buckets
	if err := db.Select("id", "type", "disabled", "capacity", "usage").Find(&buckets).Error; err != nil {
		shared.InternalFailure(c, "读取存储可用状态失败", err)
		return
	}
	policy, err := storagepolicy.Load(db)
	if err != nil {
		shared.InternalFailure(c, "读取角色存储策略失败", err)
		return
	}
	list := make([]UserWithAccessSummary, 0, len(users))
	for _, user := range users {
		summary := UserWithAccessSummary{User: user, AccessSummary: summarizeUserAccess(user, setting, buckets, policy)}
		// 所有用户都带配额摘要（含管理员），前端才能用同一套卡片布局。
		// 管理员那份额度为 unlimited=true，界面显示「不限」而不是进度条。
		summary.Quota = quotaSummaryForUser(db, user, setting)
		list = append(list, summary)
	}
	c.JSON(http.StatusOK, result.Success("查询成功", map[string]any{
		"total":              total,
		"list":               list,
		"multi_storage_sync": setting.MultiStorageSync,
		// 默认配额随列表一起下发，前端可在没有自定义配额的账号上
		// 直接展示「跟随系统默认 xxx」，不必再发一次设置请求。
		"default_user_quota": setting.DefaultUserQuota,
		"can_manage_quota":   CanManageUserQuota(c),
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
	if !shared.BindJSON(c, &req) {
		return
	}

	if req.Username != strings.TrimSpace(req.Username) || shared.IsTouristUsername(req.Username) || len(req.Password) > 72 {
		c.JSON(400, result.Error(400, "用户名为保留名称或密码长度无效"))
		return
	}

	// 创建管理员必须另有 user:role:update 或是超管。
	// 原先只要求 user:create，等于「能建普通用户」就顺带能建管理员，
	// 把专门的 role 授权环节架空。角色是权限体系的上层结构，不该由
	// 建号权限隐式获得。
	if req.Role == models.RoleAdmin {
		operator, ok := middlewares.GetCurrentUser(c)
		if !ok {
			c.JSON(http.StatusUnauthorized, result.Error(401, "未登录"))
			return
		}
		if operator.ID != models.SuperAdminID && !operator.Permission.HasPermission("user:role:update") {
			c.JSON(http.StatusForbidden, result.Error(403, "创建管理员需要 user:role:update 权限或超级管理员身份"))
			return
		}
	}
	db := database.GetDB().DB

	if db.Where("username = ?", req.Username).First(&models.User{}).Error == nil {
		c.JSON(http.StatusBadRequest, result.Error(400, "用户名已存在"))
		return
	}

	if req.Role != models.RoleAdmin && req.Role != models.RoleUser {
		req.Role = models.RoleUser
	}

	hashedPwd, err := HashPassword(req.Password)
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
		// 原始 DB 错误只进服务端日志，不回给客户端。
		shared.InternalFailure(c, "创建用户失败", err)
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
		shared.InternalFailure(c, "删除用户失败", err)
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
	if !shared.BindJSON(c, &req) {
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

	// 提升为管理员必须由超管亲自操作。
	//
	// 只要求 user:role:update 是不够的：持该权限的普通账号可以把**第二个账号 B**
	// 提成 role=1，再由 B 以管理员身份做任何事 —— 与「授予自己未持有的权限码」
	// 完全同构的横向提权，只是换了角色通道。
	// 只拦「超管 + 自己」挡不住这条：动的既不是超管也不是自己。
	{
		operator, ok := middlewares.GetCurrentUser(c)
		if !ok {
			c.JSON(http.StatusUnauthorized, result.Fail(401, "未登录"))
			return
		}
		if req.Role == models.RoleAdmin && operator.ID != models.SuperAdminID {
			c.JSON(http.StatusForbidden, result.Fail(403, "只有超级管理员可以授予管理员角色"))
			return
		}
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
		shared.InternalFailure(c, "更新角色失败", err)
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

	// 非超管不得重置管理员密码。
	// 重置密码会把 new_password 直接返回给调用者，等于拿到该账号的完整身份；
	// 若目标是被委派了权限的管理员，一次重置就能继承其全部权限。
	// 这比「改自己的权限」更隐蔽 —— 动的是别人的账号，也绕过了
	// UpdateUserPermission 里对 ID=1 与自身的保护。
	{
		operator, ok := middlewares.GetCurrentUser(c)
		if !ok {
			c.JSON(http.StatusUnauthorized, result.Fail(401, "未登录"))
			return
		}
		if operator.ID != models.SuperAdminID && user.Role == models.RoleAdmin {
			c.JSON(http.StatusForbidden, result.Fail(403, "不能重置管理员密码"))
			return
		}
	}

	// 生成12位友好随机密码
	newPassword, err := authsecurity.RandomSecret(18)
	if err != nil {
		c.JSON(500, result.Error(500, "随机密码生成失败"))
		return
	}
	hashedPwd, err := HashPassword(newPassword)
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
		shared.InternalFailure(c, "重置密码失败", err)
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

	if !shared.BindJSON(c, &req) {
		return
	}

	setting, err := settings.GetSettings()
	if err != nil {
		c.JSON(http.StatusInternalServerError, result.Fail(500, "读取多存储设置失败"))
		return
	}
	// 权限码（Codes）修改必须无条件执行超管/自我保护，与前端
	// accessReadOnlyReason 的只读规则保持一致；多存储同步模式仅放宽
	// 存储源（Buckets）的编辑范围，绝不放宽权限码门控。
	if req.Codes != nil || !setting.MultiStorageSync {
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

		// 非超管只能授予自己已持有的权限码（逐码 subset 校验）。
		//
		// 只校验「码合法」是不够的：持有 user:permission:update 的普通账号
		// 可以开第二个账号 B，把 setting:security 之类自己都没有的码授给 B，
		// 再由 B 去改 SSO 配置完成提权 —— 这条链完全绕开了
		// 「普通用户不能给自己加全部权限」的意图。授予权必须收敛到持有权。
		operator, ok := middlewares.GetCurrentUser(c)
		if !ok {
			c.JSON(http.StatusUnauthorized, result.Fail(401, "未登录"))
			return
		}
		if operator.ID != models.SuperAdminID {
			escalated := make([]string, 0, len(validCodes))
			for _, code := range validCodes {
				if !operator.Permission.HasPermission(code) {
					escalated = append(escalated, code)
				}
			}
			if len(escalated) > 0 {
				names := make([]string, 0, len(escalated))
				for _, code := range escalated {
					names = append(names, models.GetPermissionName(code))
				}
				c.JSON(http.StatusForbidden, result.Fail(403,
					"不能授予自己未持有的权限："+strings.Join(names, "、")))
				return
			}
		}
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
		shared.InternalFailure(c, "更新权限失败", err)
		return
	}

	message := "更新成功"
	if setting.MultiStorageSync {
		message = "同步存储源更新成功"
	}
	c.JSON(http.StatusOK, result.Success(message, nil))
}

// HashPassword 使用 bcrypt 加密密码。
func HashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(hash), err
}
