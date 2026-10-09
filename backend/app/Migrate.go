package app

import (
	"fmt"
	"gorm.io/gorm"
	"log"
	"oneimg/backend/database"
	"oneimg/backend/models"
	"oneimg/backend/utils/buckets"
)

func Migrate(db *database.Database) {
	if err := db.DB.Transaction(migrateLegacyData); err != nil {
		log.Printf("[数据迁移] 失败（已回滚）: %v", err)
	}
}

// Only migrate dangling/mismatched legacy bucket references. Existing buckets,
// credentials and valid image bindings are untouched, and no image blobs/lists
// are loaded into memory. Legacy empty provider configs still require an admin
// to configure credentials, just as before; never infer production credentials.
func migrateLegacyData(tx *gorm.DB) error {
	type legacyGroup struct {
		Storage    string
		TotalUsage int64
	}
	var groups []legacyGroup
	condition := "NOT EXISTS (SELECT 1 FROM buckets WHERE buckets.id = images.bucket_id AND buckets.type = images.storage)"
	if err := tx.Model(&models.Image{}).Select("storage, COALESCE(SUM(CASE WHEN file_size > 0 THEN file_size ELSE 0 END),0) AS total_usage").Where(condition).Group("storage").Scan(&groups).Error; err != nil {
		return err
	}
	names := map[string]string{"default": "本地默认存储", "s3": "S3对象存储", "r2": "Cloudflare R2存储", "ftp": "FTP文件存储", "webdav": "WebDAV存储", "telegram": "Telegram存储"}
	for _, group := range groups {
		name, ok := names[group.Storage]
		if !ok {
			return fmt.Errorf("unknown legacy storage type %q", group.Storage)
		}
		var bucket models.Buckets
		err := tx.Where("type = ?", group.Storage).Order("id ASC").First(&bucket).Error
		if err == gorm.ErrRecordNotFound {
			bucket = models.Buckets{Name: name, Type: group.Storage, Capacity: 1099511627776, Config: getBucketConfig(group.Storage), Usage: 0}
			if err = tx.Create(&bucket).Error; err != nil {
				return err
			}
		} else if err != nil {
			return err
		}
		if err := tx.Model(&models.Image{}).Where("storage = ?", group.Storage).Where(condition).Update("bucket_id", bucket.Id).Error; err != nil {
			return err
		}
		if group.TotalUsage > 0 && group.Storage != "default" {
			if err := tx.Model(&models.Buckets{}).Where("id = ?", bucket.Id).UpdateColumn("usage", gorm.Expr(database.UsageColumn(tx)+" + ?", group.TotalUsage)).Error; err != nil {
				return err
			}
		}
	}
	// Legacy zero role only; do not change any explicitly chosen user role.
	return tx.Model(&models.User{}).Where("id = ? AND role = ?", 1, 0).Update("role", models.RoleAdmin).Error
}

func getBucketConfig(storageType string) map[string]any {
	switch storageType {
	case "s3":
		return buckets.S3BucketToMap(models.S3Bucket{})
	case "r2":
		return buckets.R2BucketToMap(models.R2Bucket{})
	case "ftp":
		return buckets.FTPBucketToMap(models.FTPBucket{})
	case "webdav":
		return buckets.WebDavBucketToMap(models.WebDavBucket{})
	case "telegram":
		return buckets.TelegramBucketToMap(models.TelegramBucket{})
	default:
		return make(map[string]any)
	}
}
