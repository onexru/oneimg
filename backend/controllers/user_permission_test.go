package controllers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"oneimg/backend/database"
	"oneimg/backend/models"

	"github.com/gin-gonic/gin"
)

func TestUpdateUserPermissionAllowsSuperAdminSyncTargets(t *testing.T) {
	initExternalAuthTestDB(t)
	db := database.GetDB().DB
	if err := db.Create(&models.Settings{ID: 1, MultiStorageSync: true}).Error; err != nil {
		t.Fatalf("create settings: %v", err)
	}
	buckets := []models.Buckets{
		{Id: 1, Name: "local", Type: "default", Config: map[string]any{}},
		{Id: 2, Name: "remote", Type: "s3", Capacity: 1024, Config: map[string]any{}},
	}
	if err := db.Create(&buckets).Error; err != nil {
		t.Fatalf("create buckets: %v", err)
	}
	superAdmin := models.User{
		ID:         models.SuperAdminID,
		Role:       models.RoleAdmin,
		Username:   "root-admin",
		Password:   "test",
		Permission: models.Permission{Buckets: []int{}},
	}
	if err := db.Create(&superAdmin).Error; err != nil {
		t.Fatalf("create super admin: %v", err)
	}

	status, message := performPermissionUpdate(t, models.SuperAdminID, models.SuperAdminID, []int{2, 2})
	if status != http.StatusOK {
		t.Fatalf("update super admin targets status=%d message=%q", status, message)
	}
	var stored models.User
	if err := db.First(&stored, models.SuperAdminID).Error; err != nil {
		t.Fatalf("load super admin: %v", err)
	}
	if len(stored.Permission.Buckets) != 1 || stored.Permission.Buckets[0] != 2 {
		t.Fatalf("stored synchronization targets = %v", stored.Permission.Buckets)
	}

	status, message = performPermissionUpdate(t, models.SuperAdminID, models.SuperAdminID, []int{})
	if status != http.StatusOK {
		t.Fatalf("clear super admin targets status=%d message=%q", status, message)
	}
	if err := db.First(&stored, models.SuperAdminID).Error; err != nil {
		t.Fatalf("reload super admin: %v", err)
	}
	if len(stored.Permission.Buckets) != 0 {
		t.Fatalf("cleared synchronization targets = %v", stored.Permission.Buckets)
	}

	status, _ = performPermissionUpdate(t, models.SuperAdminID, models.SuperAdminID, []int{1})
	if status != http.StatusBadRequest {
		t.Fatalf("local default bucket status=%d, want 400", status)
	}

	if err := db.Model(&models.Settings{}).Where("id = ?", 1).Update("multi_storage_sync", false).Error; err != nil {
		t.Fatalf("disable multi-storage sync: %v", err)
	}
	status, _ = performPermissionUpdate(t, models.SuperAdminID, models.SuperAdminID, []int{2})
	if status != http.StatusBadRequest {
		t.Fatalf("single-storage super admin permission status=%d, want 400", status)
	}
}

func performPermissionUpdate(t *testing.T, loginUserID, targetUserID int, permission []int) (int, string) {
	t.Helper()
	body, err := json.Marshal(map[string]any{"permission": permission})
	if err != nil {
		t.Fatalf("marshal permission request: %v", err)
	}
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Params = gin.Params{{Key: "id", Value: fmt.Sprintf("%d", targetUserID)}}
	context.Set("user_id", loginUserID)
	context.Request = httptest.NewRequest(http.MethodPost, "/api/users/updatePermission/1", bytes.NewReader(body))
	context.Request.Header.Set("Content-Type", "application/json")

	UpdateUserPermission(context)

	var response struct {
		Message string `json:"message"`
	}
	_ = json.Unmarshal(recorder.Body.Bytes(), &response)
	return recorder.Code, response.Message
}
