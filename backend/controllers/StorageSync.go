package controllers

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"oneimg/backend/database"
	"oneimg/backend/models"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type ImageStorageStatusResponse struct {
	BucketID    int        `json:"bucket_id"`
	BucketName  string     `json:"bucket_name"`
	BucketType  string     `json:"bucket_type"`
	Status      string     `json:"status"`
	URL         string     `json:"url,omitempty"`
	Thumbnail   string     `json:"thumbnail,omitempty"`
	Error       string     `json:"error,omitempty"`
	RetryCount  int        `json:"retry_count"`
	UpdatedAt   time.Time  `json:"updated_at"`
	NextRetryAt *time.Time `json:"next_retry_at,omitempty"`
	SyncedAt    *time.Time `json:"synced_at,omitempty"`
}

// resolveUploadBuckets returns the durable local source and the remote targets
// assigned to the current user. In multi-storage mode every persisted user,
// including administrators, has an explicit list; guests retain the system
// default because they have no user record.
func resolveUploadBuckets(c *gin.Context, setting models.Settings) (models.Buckets, []models.Buckets, error) {
	db := database.GetDB()
	if db == nil || db.DB == nil {
		return models.Buckets{}, nil, errors.New("数据库未初始化")
	}

	var allBuckets []models.Buckets
	if err := db.DB.Order("id ASC").Find(&allBuckets).Error; err != nil {
		return models.Buckets{}, nil, err
	}

	var localBucket models.Buckets
	for _, bucket := range allBuckets {
		if bucket.Type == "default" {
			localBucket = bucket
			if bucket.Id == 1 {
				break
			}
		}
	}
	if localBucket.Id == 0 {
		return models.Buckets{}, nil, fmt.Errorf("本机存储源不存在")
	}

	role := c.GetInt("user_role")
	permission := models.Permission{Buckets: []int{}}
	if role != models.RoleGuest {
		var user models.User
		err := db.DB.Select("id", "permission").First(&user, c.GetInt("user_id")).Error
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return models.Buckets{}, nil, err
		}
		if err == nil {
			permission = user.Permission
		}
	}

	targets := make([]models.Buckets, 0, len(allBuckets))
	for _, bucket := range allBuckets {
		if bucket.Id == localBucket.Id || bucket.Type == "default" {
			continue
		}

		allowed := models.IntSliceContains(permission.Buckets, bucket.Id)
		if role == models.RoleGuest {
			allowed = bucket.Id == setting.DefaultStorage
		}
		if allowed {
			targets = append(targets, bucket)
		}
	}

	return localBucket, targets, nil
}

// resolveLegacyUploadBuckets preserves the single-storage selector semantics.
func resolveLegacyUploadBuckets(c *gin.Context, setting models.Settings) ([]models.Buckets, error) {
	db := database.GetDB()
	if db == nil || db.DB == nil {
		return nil, errors.New("数据库未初始化")
	}

	var allBuckets []models.Buckets
	if err := db.DB.Order("id ASC").Find(&allBuckets).Error; err != nil {
		return nil, err
	}

	role := c.GetInt("user_role")
	permission := models.Permission{Buckets: []int{}}
	if role != models.RoleAdmin && role != models.RoleGuest {
		var user models.User
		err := db.DB.Select("id", "permission").First(&user, c.GetInt("user_id")).Error
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, err
		}
		if err == nil {
			permission = user.Permission
		}
	}

	result := make([]models.Buckets, 0, len(allBuckets))
	for _, bucket := range allBuckets {
		if bucket.Id != setting.DefaultStorage {
			if bucket.Capacity > 0 && bucket.Usage >= bucket.Capacity {
				continue
			}
			if role != models.RoleAdmin && !models.IntSliceContains(permission.Buckets, bucket.Id) {
				continue
			}
		}
		if role == models.RoleGuest && bucket.Id != setting.DefaultStorage {
			continue
		}
		result = append(result, bucket)
	}
	return result, nil
}

func canUseLegacyUploadBucket(c *gin.Context, setting models.Settings, bucketID int) (bool, error) {
	buckets, err := resolveLegacyUploadBuckets(c, setting)
	if err != nil {
		return false, err
	}
	for _, bucket := range buckets {
		if bucket.Id == bucketID {
			return true, nil
		}
	}
	return false, nil
}

func loadImageStorageStatuses(imageIDs []int, setting models.Settings) (map[int][]ImageStorageStatusResponse, error) {
	result := make(map[int][]ImageStorageStatusResponse, len(imageIDs))
	if len(imageIDs) == 0 {
		return result, nil
	}

	db := database.GetDB().DB
	var storages []models.ImageStorage
	if err := db.Where("image_id IN ?", imageIDs).Order("image_id ASC, bucket_id ASC").Find(&storages).Error; err != nil {
		return nil, err
	}

	bucketIDs := make([]int, 0, len(storages))
	seen := make(map[int]struct{}, len(storages))
	for _, storage := range storages {
		if _, ok := seen[storage.BucketID]; !ok {
			seen[storage.BucketID] = struct{}{}
			bucketIDs = append(bucketIDs, storage.BucketID)
		}
	}
	var bucketList []models.Buckets
	if len(bucketIDs) > 0 {
		if err := db.Select("id", "name", "type").Where("id IN ?", bucketIDs).Find(&bucketList).Error; err != nil {
			return nil, err
		}
	}
	bucketMap := make(map[int]models.Buckets, len(bucketList))
	for _, bucket := range bucketList {
		bucketMap[bucket.Id] = bucket
	}

	for _, storage := range storages {
		bucket := bucketMap[storage.BucketID]
		bucketType := bucket.Type
		if bucketType == "" {
			bucketType = storage.Storage
		}
		bucketName := bucket.Name
		if bucketName == "" {
			bucketName = fmt.Sprintf("存储源 #%d", storage.BucketID)
		}
		result[storage.ImageID] = append(result[storage.ImageID], ImageStorageStatusResponse{
			BucketID:    storage.BucketID,
			BucketName:  bucketName,
			BucketType:  bucketType,
			Status:      storage.Status,
			URL:         applyPublicImageURL(setting, bucketType, storage.BucketID, storage.URL),
			Thumbnail:   applyPublicImageURL(setting, bucketType, storage.BucketID, storage.Thumbnail),
			Error:       storage.Error,
			RetryCount:  storage.RetryCount,
			UpdatedAt:   storage.UpdatedAt,
			NextRetryAt: storage.NextRetryAt,
			SyncedAt:    storage.SyncedAt,
		})
	}

	return result, nil
}

func cleanupLocalUpload(image models.Image) {
	for _, publicPath := range []string{image.Url, image.Thumbnail} {
		path := strings.TrimSpace(publicPath)
		if path == "" || strings.HasPrefix(path, "http://") || strings.HasPrefix(path, "https://") {
			continue
		}
		cleanPath := filepath.Clean(filepath.FromSlash(strings.TrimPrefix(path, "/")))
		if cleanPath == "." || filepath.IsAbs(cleanPath) || cleanPath == ".." || strings.HasPrefix(cleanPath, ".."+string(filepath.Separator)) {
			continue
		}
		_ = os.Remove(cleanPath)
	}
}
