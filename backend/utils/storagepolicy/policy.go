// Package storagepolicy is the shared role-pool authority for controller
// uploads, summaries and direct-upload revalidation. It never changes auth.
package storagepolicy

import (
	"oneimg/backend/models"
	"slices"

	"gorm.io/gorm"
)

type Policies map[int]models.RoleStoragePolicy

// Read on every request/revalidation: no stale policy cache after a role edit.
func Load(db *gorm.DB) (Policies, error) {
	var rows []models.RoleStoragePolicy
	if err := db.Find(&rows).Error; err != nil {
		return nil, err
	}
	p := Policies{}
	for _, row := range rows {
		p[row.Role] = row
	}
	return p, nil
}

func ValidRole(role int) bool {
	return role == models.RoleAdmin || role == models.RoleUser || role == models.RoleGuest
}
func (p Policies) Custom(role int) bool { return p[role].Mode == "custom" }
func DefaultID(role int, s models.Settings) int {
	if role == models.RoleGuest {
		return s.EffectiveGuestStorageID()
	}
	return s.DefaultStorage
}

// Allows checks authorization, not remaining quota. Pending direct tasks hold
// quota reservations already; fullness must not invalidate their authority.
func (p Policies) Allows(role int, permission models.Permission, s models.Settings, b models.Buckets) bool {
	if b.Disabled || (s.MultiStorageSync && b.Type == "default") {
		return false
	}
	if p.Custom(role) {
		if slices.Contains(p[role].BucketIDs, b.Id) {
			return true
		}
		if role == models.RoleGuest || (!s.MultiStorageSync && role == models.RoleAdmin) {
			return false
		}
		return permission.HasBucket(b.Id)
	}
	if role == models.RoleGuest {
		return b.Id == s.EffectiveGuestStorageID()
	}
	if s.MultiStorageSync {
		return permission.HasBucket(b.Id)
	}
	return role == models.RoleAdmin || b.Id == s.DefaultStorage || permission.HasBucket(b.Id)
}

// AllowsDirect preserves historical direct-upload authority in legacy mode.
// Custom pools use the same role/extra-grant rule as controller uploads. Guest
// direct-upload authentication is still rejected by directOwner.
func (p Policies) AllowsDirect(role int, permission models.Permission, s models.Settings, b models.Buckets) bool {
	if b.Disabled {
		return false
	}
	if !p.Custom(role) {
		return role == models.RoleAdmin || b.Id == s.DefaultStorage || permission.HasBucket(b.Id)
	}
	return p.Allows(role, permission, s, b)
}

// Select keeps legacy availability rules unchanged. Custom role pools apply
// the same enabled/quota filter regardless of role and deduplicate by bucket.
func (p Policies) Select(role int, permission models.Permission, s models.Settings, all []models.Buckets) []models.Buckets {
	out := make([]models.Buckets, 0, len(all))
	seen := map[int]bool{}
	for _, b := range all {
		if seen[b.Id] || !p.Allows(role, permission, s, b) {
			continue
		}
		full := b.Capacity > 0 && b.Usage >= b.Capacity
		if full && (p.Custom(role) || (!s.MultiStorageSync && b.Id != DefaultID(role, s))) {
			continue
		}
		seen[b.Id] = true
		out = append(out, b)
	}
	return out
}

func (p Policies) InheritedIDs(role int, s models.Settings, all []models.Buckets) []int {
	out := []int{}
	for _, b := range p.Select(role, models.Permission{}, s, all) {
		out = append(out, b.Id)
	}
	slices.Sort(out)
	return out
}

type RoleSummary struct {
	Role               int    `json:"role"`
	Mode               string `json:"mode"`
	BucketIDs          []int  `json:"bucket_ids"`
	EffectiveBucketIDs []int  `json:"effective_bucket_ids"`
}

func (p Policies) Summaries(s models.Settings, all []models.Buckets) []RoleSummary {
	out := []RoleSummary{}
	for _, role := range []int{models.RoleAdmin, models.RoleUser, models.RoleGuest} {
		mode, ids := "legacy", []int{}
		if p.Custom(role) {
			mode = "custom"
			ids = append(ids, p[role].BucketIDs...)
		}
		out = append(out, RoleSummary{Role: role, Mode: mode, BucketIDs: ids, EffectiveBucketIDs: p.InheritedIDs(role, s, all)})
	}
	return out
}
