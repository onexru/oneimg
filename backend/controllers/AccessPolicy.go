package controllers

import (
	"errors"
	"fmt"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"net/http"
	"oneimg/backend/database"
	"oneimg/backend/middlewares"
	"oneimg/backend/models"
	"oneimg/backend/utils/result"
	"oneimg/backend/utils/settings"
	"oneimg/backend/utils/storagepolicy"
)

// AccessPolicyBucket intentionally excludes configuration, endpoints and secrets.
type AccessPolicyBucket struct {
	ID       int    `json:"id"`
	Name     string `json:"name"`
	Type     string `json:"type"`
	Disabled bool   `json:"disabled"`
	Capacity uint64 `json:"capacity"`
	Usage    uint64 `json:"usage"`
}
type AccessPolicyResponse struct {
	MultiStorageSync        bool                        `json:"multi_storage_sync"`
	GuestEnabled            bool                        `json:"guest_enabled"`
	GuestStorageID          int                         `json:"guest_storage_id"`
	EffectiveGuestStorageID int                         `json:"effective_guest_storage_id"`
	DefaultStorageID        int                         `json:"default_storage_id"`
	Buckets                 []AccessPolicyBucket        `json:"buckets"`
	Roles                   []storagepolicy.RoleSummary `json:"roles"`
	CanEditGuestStorage     bool                        `json:"can_edit_guest_storage"`
	CanAssignUsers          bool                        `json:"can_assign_users"`
}

func canEditStoragePolicy(c *gin.Context, user *models.User) bool {
	// Exactly UpdateSettings rules; wildcard alone is NOT setting:upload.
	return c.GetString("auth_method") != "api_token" && (user.ID == models.SuperAdminID || user.Permission.HasPermission("setting:upload"))
}
func readStoragePolicyOverview() (models.Settings, []AccessPolicyBucket, []storagepolicy.RoleSummary, error) {
	s, err := settings.GetSettings()
	if err != nil {
		return s, nil, nil, err
	}
	var all []models.Buckets
	db := database.GetDB().DB
	if err := db.Select("id", "name", "type", "disabled", "capacity", "usage").Order("id ASC").Find(&all).Error; err != nil {
		return s, nil, nil, err
	}
	policy, err := storagepolicy.Load(db)
	if err != nil {
		return s, nil, nil, err
	}
	buckets := make([]AccessPolicyBucket, 0, len(all))
	for _, b := range all {
		buckets = append(buckets, AccessPolicyBucket{ID: b.Id, Name: b.Name, Type: b.Type, Disabled: b.Disabled, Capacity: b.Capacity, Usage: b.Usage})
	}
	return s, buckets, policy.Summaries(s, all), nil
}

// Management-only route; deliberately does not require setting:list.
func GetAccessPolicy(c *gin.Context) {
	c.Header("Cache-Control", "private, no-store")
	user, ok := middlewares.GetCurrentUser(c)
	if !ok {
		c.JSON(401, result.Error(401, "未登录"))
		return
	}
	s, buckets, roles, err := readStoragePolicyOverview()
	if err != nil {
		internalFailure(c, "获取访问策略失败", err)
		return
	}
	nonToken := c.GetString("auth_method") != "api_token"
	effectiveGuestID := s.EffectiveGuestStorageID()
	for _, role := range roles {
		if role.Role == models.RoleGuest && role.Mode == "custom" {
			effectiveGuestID = 0 // Use roles for empty/multi-source pools.
			if len(role.EffectiveBucketIDs) == 1 {
				effectiveGuestID = role.EffectiveBucketIDs[0]
			}
		}
	}
	c.JSON(http.StatusOK, result.Success("ok", AccessPolicyResponse{
		MultiStorageSync: s.MultiStorageSync, GuestEnabled: s.Tourist, GuestStorageID: s.GuestStorage,
		EffectiveGuestStorageID: effectiveGuestID, DefaultStorageID: s.DefaultStorage,
		Buckets: buckets, Roles: roles, CanEditGuestStorage: canEditStoragePolicy(c, user),
		CanAssignUsers: user.Permission.HasPermission("user:permission:update") || (nonToken && (user.ID == models.SuperAdminID || user.Permission.HasPermission("*"))),
	}))
}
func defaultUploadStorageID(role int, s models.Settings) int { return storagepolicy.DefaultID(role, s) }
func validateGuestStorage(value any) error {
	id, err := strictSettingInt(value)
	if err != nil || id < 0 {
		return fmt.Errorf("游客存储必须是大于或等于 0 的整数")
	}
	if id == 0 {
		return nil
	}
	var bucket models.Buckets
	if err := database.GetDB().DB.Select("id", "disabled").First(&bucket, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return fmt.Errorf("游客存储源不存在")
		}
		return fmt.Errorf("查询游客存储源失败")
	}
	if bucket.Disabled {
		return fmt.Errorf("游客存储源已停用，请先启用后再选择")
	}
	return nil
}
