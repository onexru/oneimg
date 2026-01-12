package app

import (
	"log"
	"oneimg/backend/database"
	"oneimg/backend/models"
	"oneimg/backend/utils/buckets"
)

func Migrate(db *database.Database) {
	var images []models.Image
	defaultBucket := make(map[int]models.Image)
	s3Bucket := make(map[int]models.Image)
	r2Bucket := make(map[int]models.Image)
	ftpBucket := make(map[int]models.Image)
	webdavBucket := make(map[int]models.Image)
	telegramBucket := make(map[int]models.Image)

	if err := db.DB.Find(&images).Error; err != nil {
		log.Printf("[数据迁移] 查询图片列表失败: %s", err.Error())
		return
	}
	log.Printf("[数据迁移] 共查询到 %d 张图片，开始分类处理", len(images))

	for _, image := range images {
		switch image.Storage {
		case "default":
			defaultBucket[image.Id] = image
		case "s3":
			s3Bucket[image.Id] = image
		case "r2":
			r2Bucket[image.Id] = image
		case "ftp":
			ftpBucket[image.Id] = image
		case "webdav":
			webdavBucket[image.Id] = image
		case "telegram":
			telegramBucket[image.Id] = image
		}
	}

	if len(defaultBucket) > 0 {
		log.Printf("[数据迁移-default] 共处理 %d 张图片", len(defaultBucket))
		err := db.DB.Model(&models.Image{}).Where("storage = ?", "default").Updates(map[string]any{
			"bucket_id": 1,
			"storage":   "default",
		}).Error
		if err != nil {
			log.Printf("[数据迁移-default] 批量更新图片失败: %s", err.Error())
		} else {
			log.Printf("[数据迁移-default] 图片绑定桶ID=1 完成")
		}
	}

	handleBucketMigrate(db, "s3", 2, "S3对象存储", s3Bucket)
	handleBucketMigrate(db, "r2", 3, "Cloudflare R2存储", r2Bucket)
	handleBucketMigrate(db, "ftp", 4, "FTP文件存储", ftpBucket)
	handleBucketMigrate(db, "webdav", 5, "WebDAV存储", webdavBucket)
	handleBucketMigrate(db, "telegram", 6, "Telegram存储", telegramBucket)

	log.Printf("[数据迁移] 所有存储类型迁移流程执行完毕")
}

func handleBucketMigrate(db *database.Database, storageType string, bucketId int, bucketName string, imageMap map[int]models.Image) {
	imageCount := len(imageMap)
	if imageCount == 0 {
		return
	}
	log.Printf("[数据迁移-%s] 共处理 %d 张图片，开始创建存储桶+统计容量", storageType, imageCount)

	tx := db.DB.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
			log.Printf("[数据迁移-%s] 程序异常，事务回滚: %v", storageType, r)
		}
	}()

	var totalUsage uint64
	for _, img := range imageMap {
		totalUsage += uint64(img.FileSize)
	}

	bucket := models.Buckets{
		Id:       bucketId,
		Name:     bucketName,
		Type:     storageType,
		Capacity: 1099511627776,
		Config:   getBucketConfig(storageType),
		Usage:    totalUsage,
	}

	if err := tx.Create(&bucket).Error; err != nil {
		tx.Rollback()
		log.Printf("[数据迁移-%s] 创建存储桶失败，事务回滚: %s", storageType, err.Error())
		return
	}

	if err := tx.Model(&models.Image{}).Where("storage = ?", storageType).Update("bucket_id", bucket.Id).Error; err != nil {
		tx.Rollback()
		log.Printf("[数据迁移-%s] 批量更新图片bucket_id失败，事务回滚: %s", storageType, err.Error())
		return
	}

	tx.Commit()
	log.Printf("[数据迁移-%s] 迁移完成 存储桶ID:%d 总图片数:%d 总占用容量:%d Byte", storageType, bucket.Id, imageCount, totalUsage)
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
