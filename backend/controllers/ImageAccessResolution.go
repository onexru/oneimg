package controllers
import (
 "errors"
 "oneimg/backend/models"
 "gorm.io/gorm"
)
type resolvedImageAccess struct {
	bucket      models.Buckets
	replica     *models.ImageStorage
	storageType string
	path        string
}

func resolveImageAccess(db *gorm.DB, image models.Image, thumbnail bool) (resolvedImageAccess, error) {
	canonicalPath := image.Url
	if thumbnail {
		canonicalPath = image.Thumbnail
	}

	// An explicitly selected source must have both an enabled bucket and a
	// successful replica. If it cannot currently serve the requested object,
	// transparently fall back to the durable local copy.
	if image.AccessBucketId > 0 {
		if resolved, ok := resolveImageReplicaAccess(db, image.Id, image.AccessBucketId, thumbnail); ok {
			return resolved, nil
		}
		if resolved, ok := resolveLocalImageAccess(db, image.Id, thumbnail); ok {
			return resolved, nil
		}
	} else if image.Storage != "default" {
		// Zero means the default access policy. Prefer a successful local
		// replica whenever one exists, including migrated legacy images whose
		// canonical record still points at a remote bucket.
		if resolved, ok := resolveLocalImageAccess(db, image.Id, thumbnail); ok {
			return resolved, nil
		}
	}

	var canonicalBucket models.Buckets
	canonicalErr := db.First(&canonicalBucket, image.BucketId).Error
	if canonicalErr == nil && !canonicalBucket.Disabled && canonicalPath != "" {
		storageType := image.Storage
		if storageType == "" {
			storageType = canonicalBucket.Type
		}
		resolved := resolvedImageAccess{
			bucket:      canonicalBucket,
			storageType: storageType,
			path:        canonicalPath,
		}
		var replica models.ImageStorage
		if err := db.Where(
			"image_id = ? AND bucket_id = ? AND status = ?",
			image.Id, canonicalBucket.Id, models.ImageStorageStatusSuccess,
		).First(&replica).Error; err == nil {
			resolved.replica = &replica
		}
		return resolved, nil
	}

	if resolved, ok := resolveLocalImageAccess(db, image.Id, thumbnail); ok {
		return resolved, nil
	}
	if canonicalErr != nil && !errors.Is(canonicalErr, gorm.ErrRecordNotFound) {
		return resolvedImageAccess{}, canonicalErr
	}
	return resolvedImageAccess{}, errors.New("没有可用的图片存储源")
}

func resolveImageReplicaAccess(db *gorm.DB, imageID, bucketID int, thumbnail bool) (resolvedImageAccess, bool) {
	var replica models.ImageStorage
	if err := db.Where(
		"image_id = ? AND bucket_id = ? AND status = ?",
		imageID, bucketID, models.ImageStorageStatusSuccess,
	).First(&replica).Error; err != nil {
		return resolvedImageAccess{}, false
	}

	var bucket models.Buckets
	if err := db.Where("id = ? AND disabled = ?", bucketID, false).First(&bucket).Error; err != nil {
		return resolvedImageAccess{}, false
	}
	path := replica.URL
	if thumbnail {
		path = replica.Thumbnail
	}
	if path == "" {
		return resolvedImageAccess{}, false
	}
	storageType := replica.Storage
	if storageType == "" {
		storageType = bucket.Type
	}
	return resolvedImageAccess{bucket: bucket, replica: &replica, storageType: storageType, path: path}, true
}

func resolveLocalImageAccess(db *gorm.DB, imageID int, thumbnail bool) (resolvedImageAccess, bool) {
	var replica models.ImageStorage
	if err := db.Model(&models.ImageStorage{}).
		Select("image_storages.*").
		Joins("JOIN buckets ON buckets.id = image_storages.bucket_id").
		Where(
			"image_storages.image_id = ? AND image_storages.status = ? AND buckets.type = ? AND buckets.disabled = ?",
			imageID, models.ImageStorageStatusSuccess, "default", false,
		).
		Order("buckets.id ASC").
		First(&replica).Error; err != nil {
		return resolvedImageAccess{}, false
	}

	var bucket models.Buckets
	if err := db.First(&bucket, replica.BucketID).Error; err != nil {
		return resolvedImageAccess{}, false
	}
	path := replica.URL
	if thumbnail {
		path = replica.Thumbnail
	}
	if path == "" {
		return resolvedImageAccess{}, false
	}
	return resolvedImageAccess{bucket: bucket, replica: &replica, storageType: "default", path: path}, true
}
