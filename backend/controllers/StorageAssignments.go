package controllers

import (
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"net/http"
	"oneimg/backend/database"
	"oneimg/backend/middlewares"
	"oneimg/backend/models"
	"oneimg/backend/utils/result"
	"oneimg/backend/utils/storagepolicy"
	"slices"
	"strconv"
)

func GetStorageAssignments(c *gin.Context) {
	c.Header("Cache-Control", "private, no-store")
	user, ok := middlewares.GetCurrentUser(c)
	if !ok {
		c.JSON(401, result.Error(401, "未登录"))
		return
	}
	s, buckets, roles, err := readStoragePolicyOverview()
	if err != nil {
		internalFailure(c, "获取角色存储策略失败", err)
		return
	}
	c.JSON(200, result.Success("ok", gin.H{"multi_storage_sync": s.MultiStorageSync, "guest_enabled": s.Tourist, "default_storage_id": s.DefaultStorage, "buckets": buckets, "roles": roles, "can_edit": canEditStoragePolicy(c, user)}))
}

func UpdateStorageAssignment(c *gin.Context) {
	user, ok := middlewares.GetCurrentUser(c)
	if !ok {
		c.JSON(401, result.Error(401, "未登录"))
		return
	}
	if !canEditStoragePolicy(c, user) {
		c.JSON(403, result.Error(403, "需要上传与存储设置权限"))
		return
	}
	role, err := strconv.Atoi(c.Param("role"))
	if err != nil || !storagepolicy.ValidRole(role) {
		c.JSON(400, result.Error(400, "角色无效"))
		return
	}
	var req struct {
		Mode      string `json:"mode"`
		BucketIDs []int  `json:"bucket_ids"`
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 16<<10)
	if err := c.ShouldBindJSON(&req); err != nil || req.BucketIDs == nil || (req.Mode != "legacy" && req.Mode != "custom") || len(req.BucketIDs) > 200 {
		c.JSON(400, result.Error(400, "请提交 legacy/custom 模式及最多 200 个存储源 ID 数组"))
		return
	}
	if req.Mode == "legacy" && len(req.BucketIDs) > 0 {
		c.JSON(400, result.Error(400, "恢复默认策略时 bucket_ids 必须为空数组"))
		return
	}
	seen := map[int]bool{}
	for _, id := range req.BucketIDs {
		if id <= 0 || seen[id] {
			c.JSON(400, result.Error(400, "存储源 ID 必须为唯一正整数"))
			return
		}
		seen[id] = true
	}
	slices.Sort(req.BucketIDs)
	db := database.GetDB().DB
	// Serialize policy writes through the existing singleton settings row. Do
	// not create default policies or mutate per-user grants/auth settings.
	var validation string
	err = db.Transaction(func(tx *gorm.DB) error {
		var s models.Settings
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&s, 1).Error; err != nil {
			return err
		}
		if len(req.BucketIDs) > 0 {
			var buckets []models.Buckets
			if err := tx.Where("id IN ?", req.BucketIDs).Find(&buckets).Error; err != nil {
				return err
			}
			if len(buckets) != len(req.BucketIDs) {
				validation = "包含不存在的存储源"
				return nil
			}
			var old models.RoleStoragePolicy
			if err := tx.Where("role = ?", role).Find(&old).Error; err != nil {
				return err
			}
			for _, b := range buckets {
				if s.MultiStorageSync && b.Type == "default" {
					validation = "多存储模式的本机源固定保留，请仅选择远端同步源"
					return nil
				}
				if b.Disabled && !(old.Mode == "custom" && slices.Contains(old.BucketIDs, b.Id)) {
					validation = "不能新选择已停用的存储源"
					return nil
				}
			}
		}
		if req.Mode == "legacy" {
			return tx.Delete(&models.RoleStoragePolicy{}, "role = ?", role).Error
		}
		row := models.RoleStoragePolicy{Role: role, Mode: req.Mode, BucketIDs: req.BucketIDs}
		return tx.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "role"}}, DoUpdates: clause.AssignmentColumns([]string{"mode", "bucket_ids"})}).Create(&row).Error
	})
	if err != nil {
		internalFailure(c, "更新角色存储策略失败", err)
		return
	}
	if validation != "" {
		c.JSON(400, result.Error(400, validation))
		return
	}
	// Policy reads are uncached; a subsequent GET reads the committed row.
	c.JSON(200, result.Success("角色存储分配已更新；用户额外授权不变", nil))
}
