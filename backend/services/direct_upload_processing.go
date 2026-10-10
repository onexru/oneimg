package services

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"oneimg/backend/models"
	"oneimg/backend/utils/images"
	"oneimg/backend/utils/securestorage"
	"path"
	"time"
)

func directRead(ctx context.Context, store directObjectStore, key string, limit int64) ([]byte, error) {
	r, err := store.Read(ctx, key)
	if err != nil {
		return nil, directStoreError(err)
	}
	defer r.Close()
	b, err := images.ReadDirectLimited(r, limit)
	if err != nil {
		if errors.Is(err, images.ErrFileTooLarge) {
			return nil, directError(413, "file_too_large", "存储对象超过允许大小", false)
		}
		return nil, directStoreError(err)
	}
	return b, nil
}

func directProcessMain(ctx context.Context, db *gorm.DB, t models.DirectUploadTask) error {
	_, b, err := directAccess(db, t.OwnerID, t.BucketID)
	if err != nil {
		return err
	}
	store, err := directOpenStore(b)
	if err != nil {
		return directStoreError(err)
	}
	if t.SourceKey != t.DirectKey && t.SourceKey != t.ProxyKey {
		return directError(409, "file_required", "尚未收到文件，请重新选择原文件", false)
	}
	data, err := directRead(ctx, store, t.SourceKey, t.Size)
	if err != nil {
		return err
	}
	if err = directVerifyBytes(data, t); err != nil {
		return err
	}
	if _, _, err = images.ValidateDirectImage(data, t.ContentType); err != nil {
		return directImageError(err)
	}
	processed, err := images.ProcessDirectImage(data, t.ContentType, directProcessingSettings(t.Settings))
	if err != nil {
		return directImageError(err)
	}
	stored, err := securestorage.Encode(processed.CompressedBytes, t.Settings.EncryptedStorage)
	if err != nil {
		return directError(503, "encryption_unavailable", "加密配置不可用，请联系管理员后重试", true)
	}
	if int64(len(stored)) > DirectUploadMaxBytes+1024 {
		return directError(413, "processed_too_large", "处理后的图片超过直传大小限制", false)
	}
	key := directFinalKey(t, processed.OutputExt)
	h := sha256.Sum256(stored)
	mainHash := hex.EncodeToString(h[:])
	size := int64(len(stored))
	directMu.Lock()
	err = db.Transaction(func(tx *gorm.DB) error {
		var current models.DirectUploadTask
		if e := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&current, "id = ?", t.ID).Error; e != nil {
			return e
		}
		if current.Status != "processing" {
			return directError(409, "task_changed", "任务已取消或状态已改变", false)
		}
		if _, _, e := directAccess(tx, t.OwnerID, t.BucketID); e != nil {
			return e
		}
		if e := directQuota(tx, t.BucketID, size-current.ReservedBytes); e != nil {
			return e
		}
		return tx.Model(&current).Updates(map[string]any{"reserved_bytes": size, "final_size": size, "final_key": key, "main_sha256": mainHash}).Error
	})
	directMu.Unlock()
	if err != nil {
		return err
	}
	mime := processed.MimeType
	if t.Settings.EncryptedStorage {
		mime = "application/octet-stream"
	}
	if err = store.Put(ctx, key, stored, mime); err != nil {
		return directStoreError(err)
	}
	directMu.Lock()
	defer directMu.Unlock()
	return db.Transaction(func(tx *gorm.DB) error {
		// Same owner-first lock as folder deletion; reload the persisted task
		// after acquiring it, never publish the stale worker snapshot folder.
		if e := lockFolderOwner(tx, t.OwnerID); e != nil {
			return e
		}
		var current models.DirectUploadTask
		if e := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&current, "id = ?", t.ID).Error; e != nil {
			return e
		}
		if current.Status != "processing" {
			return directError(409, "task_changed", "任务已取消或状态已改变", false)
		}
		if _, _, e := directAccess(tx, t.OwnerID, t.BucketID); e != nil {
			return e
		}
		folderID, e := ResolvePublicationFolder(tx, current.OwnerID, current.FolderId)
		if e != nil {
			return e
		}
		now := time.Now().UTC()
		im := models.Image{FolderId: folderID, Url: "/" + key, FileName: path.Base(key), FileSize: size, MimeType: processed.MimeType, Width: processed.Width, Height: processed.Height, Storage: "r2", BucketId: t.BucketID, UserId: t.OwnerID, UUID: t.ID, CreatedAt: now}
		if e := tx.Create(&im).Error; e != nil {
			return e
		}
		replica := models.ImageStorage{ImageID: im.Id, BucketID: t.BucketID, Storage: "r2", Status: models.ImageStorageStatusSuccess, URL: im.Url, FileSize: size, SyncedAt: &now}
		if e := tx.Create(&replica).Error; e != nil {
			return e
		}
		if len(t.Tags) > 0 {
			var tags []models.Tags
			if e := tx.Where("name IN ?", t.Tags).Find(&tags).Error; e != nil {
				return e
			}
			for _, tag := range tags {
				if e := tx.Create(&models.ImageToTags{ImageId: im.Id, TagId: tag.Id}).Error; e != nil {
					return e
				}
			}
		}
		q := tx.Model(&models.DirectUploadTask{}).Where("id = ? AND status = ? AND image_id = 0", t.ID, "processing").Updates(map[string]any{"status": "ready", "folder_id": folderID, "image_id": im.Id, "reserved_bytes": 0, "error_code": "", "message": "", "retryable": false, "next_retry_at": nil})
		if q.Error != nil {
			return q.Error
		}
		if q.RowsAffected != 1 {
			return directError(409, "task_changed", "任务状态已改变", false)
		}
		return nil
	})
}

func directProcessThumbnail(ctx context.Context, db *gorm.DB, t models.DirectUploadTask) error {
	_, b, err := directAccess(db, t.OwnerID, t.BucketID)
	if err != nil {
		return err
	}
	var im models.Image
	var replica models.ImageStorage
	if err = db.First(&im, t.ImageID).Error; err != nil {
		return directError(410, "image_deleted", "图片已删除，停止生成缩略图", false)
	}
	if err = db.Where("image_id = ? AND bucket_id = ? AND status = ?", t.ImageID, t.BucketID, models.ImageStorageStatusSuccess).First(&replica).Error; err != nil {
		return directError(410, "image_deleted", "图片副本已删除，停止生成缩略图", false)
	}
	store, err := directOpenStore(b)
	if err != nil {
		return directStoreError(err)
	}
	data, err := directRead(ctx, store, t.FinalKey, t.FinalSize)
	if err != nil {
		return err
	}
	h := sha256.Sum256(data)
	if int64(len(data)) != t.FinalSize || hex.EncodeToString(h[:]) != t.MainSHA256 {
		return directError(422, "main_hash_mismatch", "主图校验失败，停止生成缩略图", false)
	}
	plain, _, err := securestorage.Decode(data)
	if err != nil {
		return directError(503, "encryption_unavailable", "主图解密失败，请检查加密配置", true)
	}
	thumb, err := images.GenerateDirectThumbnail(plain)
	if err != nil {
		return directImageError(err)
	}
	stored, err := securestorage.Encode(thumb, t.Settings.EncryptedStorage)
	if err != nil {
		return directError(503, "encryption_unavailable", "缩略图加密失败，请检查加密配置", true)
	}
	key := path.Join(path.Dir(t.FinalKey), "thumbnails", t.ID+".webp")
	size := int64(len(stored))
	directMu.Lock()
	err = db.Transaction(func(tx *gorm.DB) error {
		var current models.DirectUploadTask
		if e := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&current, "id = ?", t.ID).Error; e != nil {
			return e
		}
		if current.Status != "ready" || current.ThumbnailStatus != "processing" {
			return directError(409, "task_changed", "缩略图任务状态已改变", false)
		}
		if e := directQuota(tx, t.BucketID, size-current.ReservedBytes); e != nil {
			return e
		}
		return tx.Model(&current).Updates(map[string]any{"reserved_bytes": size, "thumbnail_key": key}).Error
	})
	directMu.Unlock()
	if err != nil {
		return err
	}
	mime := "image/webp"
	if t.Settings.EncryptedStorage {
		mime = "application/octet-stream"
	}
	if err = store.Put(ctx, key, stored, mime); err != nil {
		return directStoreError(err)
	}
	directMu.Lock()
	defer directMu.Unlock()
	return db.Transaction(func(tx *gorm.DB) error {
		if e := lockFolderOwner(tx, t.OwnerID); e != nil {
			return e
		}
		var image models.Image
		if e := tx.First(&image, t.ImageID).Error; e != nil {
			return e
		}
		if _, e := ResolvePublicationFolder(tx, image.UserId, image.FolderId); e != nil {
			return e
		}
		if _, _, e := directAccess(tx, t.OwnerID, t.BucketID); e != nil {
			return e
		}
		q := tx.Model(&models.DirectUploadTask{}).Where("id = ? AND status = ? AND thumbnail_status = ?", t.ID, "ready", "processing").Updates(map[string]any{"thumbnail_status": "ready", "reserved_bytes": 0, "error_code": "", "message": "", "retryable": false, "next_retry_at": nil})
		if q.Error != nil {
			return q.Error
		}
		if q.RowsAffected != 1 {
			return directError(409, "task_changed", "缩略图任务状态已改变", false)
		}
		q = tx.Model(&models.ImageStorage{}).Where("id = ? AND status = ?", replica.ID, models.ImageStorageStatusSuccess).Updates(map[string]any{"thumbnail": "/" + key, "thumbnail_size": size})
		if q.Error != nil {
			return q.Error
		}
		if q.RowsAffected != 1 {
			return directError(410, "image_deleted", "图片副本已删除", false)
		}
		q = tx.Model(&models.Image{}).Where("id = ? AND user_id = ?", t.ImageID, t.OwnerID).Update("thumbnail", "/"+key)
		if q.Error != nil {
			return q.Error
		}
		if q.RowsAffected != 1 {
			return directError(410, "image_deleted", "图片已删除", false)
		}
		return nil
	})
}
