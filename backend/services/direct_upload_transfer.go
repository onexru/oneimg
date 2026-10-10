package services

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"gorm.io/gorm"
	"io"
	"oneimg/backend/models"
	"oneimg/backend/utils/images"
	"time"
)

func SignDirectUpload(ctx context.Context, ownerID int, id string) (DirectUploadResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	directMu.Lock()
	defer directMu.Unlock()
	db, err := directDB(ctx)
	if err != nil {
		return DirectUploadResponse{}, err
	}
	t, err := directTask(db, ownerID, id, true)
	if err != nil {
		return DirectUploadResponse{}, err
	}
	return directSign(ctx, db, t)
}

func directSign(ctx context.Context, db *gorm.DB, t models.DirectUploadTask) (DirectUploadResponse, error) {
	if t.Status == "awaiting_upload" {
		if err := ValidateFolderSelection(db, t.OwnerID, t.Role, t.FolderId); err != nil {
			return DirectUploadResponse{}, directDBError(err)
		}
	}
	if t.Status != "awaiting_upload" {
		return directResponse(db, t)
	}
	if !time.Now().Before(t.ExpiresAt) {
		return DirectUploadResponse{}, directError(410, "task_expired", "上传任务已过期，请重新选择文件", false)
	}
	if t.SignCount >= 8 {
		return DirectUploadResponse{}, directError(429, "sign_limit", "直传重试次数已达上限，请使用服务器兜底上传", true)
	}
	_, b, err := directAccess(db, t.OwnerID, t.BucketID)
	if err != nil {
		return DirectUploadResponse{}, err
	}
	store, err := directOpenStore(b)
	if err != nil {
		return DirectUploadResponse{}, directStoreError(err)
	}
	ttl := directPutTTL
	if remaining := time.Until(t.ExpiresAt); remaining < ttl {
		ttl = remaining
	}
	if ttl < time.Second {
		return DirectUploadResponse{}, directError(410, "task_expired", "上传任务已过期", false)
	}
	url, err := store.Sign(ctx, t.DirectKey, t.ContentType, t.Size, ttl)
	if err != nil {
		return DirectUploadResponse{}, directStoreError(err)
	}
	expires := time.Now().UTC().Add(ttl)
	q := db.Model(&models.DirectUploadTask{}).Where("id = ? AND status = ? AND sign_count = ?", t.ID, "awaiting_upload", t.SignCount).Updates(map[string]any{"last_put_expires_at": expires, "sign_count": t.SignCount + 1})
	if q.Error != nil {
		return DirectUploadResponse{}, directDBError(q.Error)
	}
	if q.RowsAffected != 1 {
		return DirectUploadResponse{}, directError(409, "task_changed", "任务状态已改变，请刷新", true)
	}
	r, err := directResponse(db, t)
	r.Upload = &DirectUploadInstruction{URL: url, Method: "PUT", Headers: map[string]string{"Content-Type": t.ContentType}, ExpiresAt: expires}
	return r, err
}

func CompleteDirectUpload(ctx context.Context, ownerID int, id string) (DirectUploadResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	directMu.Lock()
	defer directMu.Unlock()
	db, err := directDB(ctx)
	if err != nil {
		return DirectUploadResponse{}, err
	}
	t, err := directTask(db, ownerID, id, true)
	if err != nil {
		return DirectUploadResponse{}, err
	}
	if t.Status != "awaiting_upload" {
		return directResponse(db, t)
	}
	if time.Now().After(t.ExpiresAt) {
		return DirectUploadResponse{}, directError(410, "task_expired", "上传任务已过期，请重新选择文件", false)
	}
	_, b, err := directAccess(db, ownerID, t.BucketID)
	if err != nil {
		return DirectUploadResponse{}, err
	}
	store, err := directOpenStore(b)
	if err != nil {
		return DirectUploadResponse{}, directStoreError(err)
	}
	n, err := store.Head(ctx, t.DirectKey)
	if err != nil {
		return DirectUploadResponse{}, directStoreError(err)
	}
	if n != t.Size {
		return DirectUploadResponse{}, directError(422, "size_mismatch", "上传文件大小不匹配，请重新传输原文件", false)
	}
	q := db.Model(&models.DirectUploadTask{}).Where("id = ? AND status = ?", id, "awaiting_upload").Updates(map[string]any{"status": "queued", "source_key": t.DirectKey, "error_code": "", "message": "", "retryable": false})
	if q.Error != nil {
		return DirectUploadResponse{}, directDBError(q.Error)
	}
	WakeDirectUploadWorker()
	return GetDirectUpload(ctx, ownerID, id)
}

func FallbackDirectUpload(ctx context.Context, ownerID int, id string, reader io.Reader) (DirectUploadResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	db, err := directDB(ctx)
	if err != nil {
		return DirectUploadResponse{}, err
	}
	directMu.Lock()
	t, err := directTask(db, ownerID, id, true)
	if err != nil {
		directMu.Unlock()
		return DirectUploadResponse{}, err
	}
	if t.Status != "awaiting_upload" {
		directMu.Unlock()
		return directResponse(db, t)
	}
	if time.Now().After(t.ExpiresAt) {
		directMu.Unlock()
		return DirectUploadResponse{}, directError(410, "task_expired", "上传任务已过期", false)
	}
	_, b, err := directAccess(db, ownerID, t.BucketID)
	if err != nil {
		directMu.Unlock()
		return DirectUploadResponse{}, err
	}
	q := db.Model(&models.DirectUploadTask{}).Where("id = ? AND status = ?", id, "awaiting_upload").Updates(map[string]any{"status": "receiving", "transport": "proxy"})
	directMu.Unlock()
	if q.Error != nil {
		return DirectUploadResponse{}, directDBError(q.Error)
	}
	if q.RowsAffected != 1 {
		return GetDirectUpload(ctx, ownerID, id)
	}
	var transferErr error
	// Read chunk-by-chunk with the request context; io.Reader itself cannot
	// interrupt a blocked custom reader, so HTTP wrappers must set a body deadline.
	data, err := images.ReadDirectLimited(directContextReader{ctx, reader}, t.Size)
	if err != nil {
		if errors.Is(err, images.ErrFileTooLarge) {
			transferErr = directError(413, "file_too_large", "文件超过声明大小", false)
		} else {
			transferErr = directError(408, "transfer_interrupted", "服务器接收中断，请重试", true)
		}
	} else {
		transferErr = directVerifyBytes(data, t)
	}
	if transferErr == nil {
		var store directObjectStore
		store, err = directOpenStore(b)
		if err == nil {
			err = store.Put(ctx, t.ProxyKey, data, t.ContentType)
		}
		if err != nil {
			transferErr = directStoreError(err)
		}
	}
	// Persist request cancellation too; never strand receiving until a restart.
	finishCtx, finishCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer finishCancel()
	directMu.Lock()
	defer directMu.Unlock()
	finishDB := db.WithContext(finishCtx)
	fields := map[string]any{"status": "queued", "source_key": t.ProxyKey, "error_code": "", "message": "", "retryable": false}
	if transferErr != nil {
		e := directAsError(transferErr)
		fields = map[string]any{"status": "awaiting_upload", "error_code": e.Code, "message": e.Message, "retryable": e.Retryable}
	}
	q = finishDB.Model(&models.DirectUploadTask{}).Where("id = ? AND status = ?", id, "receiving").Updates(fields)
	if q.Error != nil {
		return DirectUploadResponse{}, directDBError(q.Error)
	}
	WakeDirectUploadWorker()
	if transferErr != nil {
		return DirectUploadResponse{}, transferErr
	}
	return GetDirectUpload(finishCtx, ownerID, id)
}

func directVerifyBytes(data []byte, t models.DirectUploadTask) error {
	if int64(len(data)) != t.Size {
		return directError(422, "size_mismatch", "上传文件大小不匹配，请重新选择原文件", false)
	}
	h := sha256.Sum256(data)
	if hex.EncodeToString(h[:]) != t.SHA256 {
		return directError(422, "hash_mismatch", "文件 SHA256 校验失败，请重新选择原文件", false)
	}
	return nil
}

type directContextReader struct {
	ctx context.Context
	r   io.Reader
}

func (r directContextReader) Read(p []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	return r.r.Read(p)
}
