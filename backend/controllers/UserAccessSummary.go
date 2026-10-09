package controllers

import (
	"oneimg/backend/models"
	"oneimg/backend/utils/storagepolicy"
)

// Read-only metadata describes effective access, not the raw extra-grant arrays.
// It is never used to grant authorization or persisted back to Permission.
type UserAccessSummary struct {
	StorageMode               string `json:"storage_mode"`
	StorageCount              int    `json:"storage_count"`
	DefaultStorageID          int    `json:"default_storage_id"`
	DefaultStorageIncluded    bool   `json:"default_storage_included"`
	LocalStorageIncluded      bool   `json:"local_storage_included"`
	AllManagementPermissions  bool   `json:"all_management_permissions"`
	AdditionalPermissionCount int    `json:"additional_permission_count"`
	InheritedBucketIDs        []int  `json:"inherited_bucket_ids"`
}
type UserWithAccessSummary struct {
	models.User
	AccessSummary UserAccessSummary `json:"access_summary"`
}

func summarizeUserAccess(user models.User, setting models.Settings, buckets []models.Buckets, policies ...storagepolicy.Policies) UserAccessSummary {
	policy := storagepolicy.Policies{}
	if len(policies) > 0 {
		policy = policies[0]
	}
	s := UserAccessSummary{StorageMode: "single", DefaultStorageID: storagepolicy.DefaultID(user.Role, setting), AllManagementPermissions: user.ID == models.SuperAdminID || user.Permission.HasPermission("*"), InheritedBucketIDs: policy.InheritedIDs(user.Role, setting, buckets)}
	codes := map[string]bool{}
	for _, code := range user.Permission.Codes {
		if _, ok := models.AllPermissionMap[code]; ok {
			codes[code] = true
		}
	}
	s.AdditionalPermissionCount = len(codes)
	available := policy.Select(user.Role, user.Permission, setting, buckets)
	s.StorageCount = len(available)
	if setting.MultiStorageSync {
		s.StorageMode = "multi"
		for _, bucket := range buckets {
			if bucket.Type == "default" {
				s.LocalStorageIncluded = true
				break
			}
		}
	} else {
		for _, bucket := range available {
			if bucket.Id == s.DefaultStorageID {
				s.DefaultStorageIncluded = true
			}
		}
	}
	return s
}
