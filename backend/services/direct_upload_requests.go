package services

import (
	"context"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"oneimg/backend/models"
	"strings"
	"time"
)

func CreateDirectUpload(ctx context.Context, ownerID, role int, req CreateDirectUploadRequest) (DirectUploadResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	directMu.Lock()
	defer directMu.Unlock()
	db, err := directDB(ctx)
	if err != nil {
		return DirectUploadResponse{}, err
	}
	s, _, err := directAccess(db, ownerID, req.BucketID)
	if err != nil {
		return DirectUploadResponse{}, err
	}
	if role == models.RoleGuest || s.MultiStorageSync {
		return DirectUploadResponse{}, directError(409, "direct_not_available", "当前模式请使用普通上传", false)
	}
	if err = directValidRequest(req, s); err != nil {
		return DirectUploadResponse{}, err
	}
	var t models.DirectUploadTask
	err = db.Transaction(func(tx *gorm.DB) error {
		// Serialize owner caps across processes as well as the local API mutex.
		var u models.User
		if e := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&u, ownerID).Error; e != nil {
			return e
		}
		q := tx.Where("owner_id = ? AND client_id = ?", ownerID, req.ClientID).Find(&t)
		if q.Error != nil {
			return q.Error
		}
		if q.RowsAffected != 0 {
			if t.RequestedFolderId != req.FolderID || t.BucketID != req.BucketID || t.Filename != req.Filename || t.Size != req.Size || t.SHA256 != req.SHA256 || t.ContentType != req.ContentType || strings.Join(t.Tags, "\x00") != strings.Join(req.Tags, "\x00") {
				return directError(409, "idempotency_conflict", "同一任务标识不能用于不同文件", false)
			}
			return nil
		}
		if e := ValidateFolderSelection(tx, ownerID, u.Role, req.FolderID); e != nil {
			return e
		}
		var active, recent int64
		if e := tx.Model(&models.DirectUploadTask{}).Where("owner_id = ? AND expires_at > ? AND (status IN ? OR (status = ? AND thumbnail_status IN ?))", ownerID, time.Now(), []string{"awaiting_upload", "receiving", "queued", "processing", "failed"}, "ready", []string{"pending", "processing"}).Count(&active).Error; e != nil {
			return e
		}
		if e := tx.Model(&models.DirectUploadTask{}).Where("owner_id = ? AND created_at > ?", ownerID, time.Now().Add(-time.Hour)).Count(&recent).Error; e != nil {
			return e
		}
		if active >= 12 || recent >= 60 {
			return directError(429, "task_limit", "上传任务过多，请完成或取消已有任务后再试", true)
		}
		if e := directQuota(tx, req.BucketID, req.Size); e != nil {
			return e
		}
		id := uuid.NewString()
		now := time.Now().UTC()
		thumb := "disabled"
		if s.Thumbnail {
			thumb = "pending"
		}
		t = models.DirectUploadTask{ID: id, OwnerID: ownerID, Role: u.Role, ClientID: req.ClientID, BucketID: req.BucketID, FolderId: req.FolderID, RequestedFolderId: req.FolderID, Filename: req.Filename, Size: req.Size, ContentType: req.ContentType, SHA256: req.SHA256, Tags: req.Tags, Settings: directSnapshot(s), Status: "awaiting_upload", Transport: "direct", ThumbnailStatus: thumb, DirectKey: "_oneimg_direct/" + id + "/browser", ProxyKey: "_oneimg_direct/" + id + "/proxy", ReservedBytes: req.Size, ExpiresAt: now.Add(directTaskTTL), CreatedAt: now, UpdatedAt: now}
		return tx.Create(&t).Error
	})
	if err != nil {
		return DirectUploadResponse{}, directDBError(err)
	}
	if t.Status == "awaiting_upload" {
		return directSign(ctx, db, t)
	}
	return directResponse(db, t)
}

func ListDirectUploads(ctx context.Context, ownerID int) ([]DirectUploadResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	db, err := directDB(ctx)
	if err != nil {
		return nil, err
	}
	if _, err = directOwner(db, ownerID); err != nil {
		return nil, err
	}
	var tasks []models.DirectUploadTask
	if err = db.Where("owner_id = ? AND error_code <> ?", ownerID, "image_deleted").Order("created_at DESC").Limit(100).Find(&tasks).Error; err != nil {
		return nil, directDBError(err)
	}
	out := make([]DirectUploadResponse, 0, len(tasks))
	for _, t := range tasks {
		r, e := directResponse(db, t)
		if e != nil {
			return nil, e
		}
		out = append(out, r)
	}
	return out, nil
}

func GetDirectUpload(ctx context.Context, ownerID int, id string) (DirectUploadResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	db, err := directDB(ctx)
	if err != nil {
		return DirectUploadResponse{}, err
	}
	t, err := directTask(db, ownerID, id, false)
	if err != nil {
		return DirectUploadResponse{}, err
	}
	return directResponse(db, t)
}
