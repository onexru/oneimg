package controllers

import (
	"path/filepath"
	"testing"

	"oneimg/backend/config"
	"oneimg/backend/database"
	"oneimg/backend/models"

	"github.com/gin-gonic/gin"
)

func TestResolveUploadBucketsSwitchModes(t *testing.T) {
	database.InitDB(&config.Config{
		DbType:     "sqlite",
		SqlitePath: filepath.Join(t.TempDir(), "upload-buckets.db"),
	})
	db := database.GetDB().DB
	buckets := []models.Buckets{
		{Id: 1, Name: "local", Type: "default", Config: map[string]any{}},
		{Id: 2, Name: "default remote", Type: "s3", Capacity: 1024, Config: map[string]any{}},
		{Id: 3, Name: "assigned remote", Type: "webdav", Capacity: 1024, Config: map[string]any{}},
		{Id: 4, Name: "paused remote", Type: "ftp", Disabled: true, Capacity: 1024, Config: map[string]any{}},
	}
	if err := db.Create(&buckets).Error; err != nil {
		t.Fatalf("create buckets: %v", err)
	}
	user := models.User{
		ID:       10,
		Role:     models.RoleUser,
		Username: "sync-user",
		Password: "test",
		Permission: models.Permission{
			Buckets: []int{3, 4},
		},
	}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}
	superAdmin := models.User{
		ID:         models.SuperAdminID,
		Role:       models.RoleAdmin,
		Username:   "root-admin",
		Password:   "test",
		Permission: models.Permission{Buckets: []int{2}},
	}
	if err := db.Create(&superAdmin).Error; err != nil {
		t.Fatalf("create super admin: %v", err)
	}
	setting := models.Settings{DefaultStorage: 2, MultiStorageSync: true}

	userContext := &gin.Context{}
	userContext.Set("user_id", user.ID)
	userContext.Set("user_role", models.RoleUser)
	local, targets, err := resolveUploadBuckets(userContext, setting)
	if err != nil {
		t.Fatalf("resolve user targets: %v", err)
	}
	if local.Id != 1 || !bucketIDsEqual(targets, []int{3}) {
		t.Fatalf("multi mode must use explicit user targets; local=%d targets=%v", local.Id, bucketIDs(targets))
	}

	legacyTargets, err := resolveLegacyUploadBuckets(userContext, setting)
	if err != nil {
		t.Fatalf("resolve legacy targets: %v", err)
	}
	if !bucketIDsEqual(legacyTargets, []int{2, 3}) {
		t.Fatalf("single mode must preserve default+authorized buckets, got %v", bucketIDs(legacyTargets))
	}

	guestContext := &gin.Context{}
	guestContext.Set("user_role", models.RoleGuest)
	_, guestTargets, err := resolveUploadBuckets(guestContext, setting)
	if err != nil {
		t.Fatalf("resolve guest targets: %v", err)
	}
	if !bucketIDsEqual(guestTargets, []int{2}) {
		t.Fatalf("guest should only synchronize to the configured default, got %v", bucketIDs(guestTargets))
	}

	adminContext := &gin.Context{}
	adminContext.Set("user_id", models.SuperAdminID)
	adminContext.Set("user_role", models.RoleAdmin)
	_, adminTargets, err := resolveUploadBuckets(adminContext, setting)
	if err != nil {
		t.Fatalf("resolve admin targets: %v", err)
	}
	if !bucketIDsEqual(adminTargets, []int{2}) {
		t.Fatalf("super admin should use its explicit synchronization targets, got %v", bucketIDs(adminTargets))
	}
	if err := db.Model(&superAdmin).Update("permission", models.Permission{Buckets: []int{3}}).Error; err != nil {
		t.Fatalf("update super admin targets: %v", err)
	}
	_, adminTargets, err = resolveUploadBuckets(adminContext, setting)
	if err != nil {
		t.Fatalf("resolve updated super admin targets: %v", err)
	}
	if !bucketIDsEqual(adminTargets, []int{3}) {
		t.Fatalf("updated super admin synchronization targets were not applied, got %v", bucketIDs(adminTargets))
	}
}

func bucketIDs(buckets []models.Buckets) []int {
	ids := make([]int, 0, len(buckets))
	for _, bucket := range buckets {
		ids = append(ids, bucket.Id)
	}
	return ids
}

func bucketIDsEqual(buckets []models.Buckets, want []int) bool {
	got := bucketIDs(buckets)
	if len(got) != len(want) {
		return false
	}
	for index := range got {
		if got[index] != want[index] {
			return false
		}
	}
	return true
}
