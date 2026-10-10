package services

import (
	"context"
	"errors"
	"gorm.io/gorm"
	"oneimg/backend/database"
	"oneimg/backend/models"
	"oneimg/backend/utils/publicurl"
	"oneimg/backend/utils/storagepolicy"
	"path"
	"strings"
)

func directDB(ctx context.Context) (*gorm.DB, error) {
	d := database.GetDB()
	if d == nil || d.DB == nil {
		return nil, directDBError(errors.New("no db"))
	}
	return d.DB.WithContext(ctx), nil
}

func directOwner(db *gorm.DB, id int) (models.User, error) {
	var u models.User
	if id <= 0 {
		return u, directError(403, "authentication_required", "请登录后上传", false)
	}
	if err := db.First(&u, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return u, directError(403, "owner_missing", "用户不存在或已被删除", false)
		}
		return u, directDBError(err)
	}
	if u.Role == models.RoleGuest {
		return u, directError(403, "guest_unsupported", "游客请使用普通上传", false)
	}
	return u, nil
}

func directAccess(db *gorm.DB, owner, bucket int) (models.Settings, models.Buckets, error) {
	var s models.Settings
	var b models.Buckets
	u, err := directOwner(db, owner)
	if err != nil {
		return s, b, err
	}
	if err = db.First(&s, 1).Error; err != nil {
		return s, b, directDBError(err)
	}
	if err = db.First(&b, bucket).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return s, b, directError(403, "bucket_missing", "存储源已被删除", false)
		}
		return s, b, directDBError(err)
	}
	if b.Type != "r2" || b.Disabled {
		return s, b, directError(403, "bucket_unavailable", "该存储源不支持直传或已停用", false)
	}
	policy, err := storagepolicy.Load(db)
	if err != nil {
		return s, b, directDBError(err)
	}
	if !policy.AllowsDirect(u.Role, u.Permission, s, b) {
		return s, b, directError(403, "bucket_forbidden", "无权使用该存储源", false)
	}
	return s, b, nil
}

func directTask(db *gorm.DB, owner int, id string, mutate bool) (models.DirectUploadTask, error) {
	var t models.DirectUploadTask
	if _, err := directOwner(db, owner); err != nil {
		return t, err
	}
	if err := db.Where("id = ? AND owner_id = ?", id, owner).First(&t).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return t, directError(404, "task_not_found", "上传任务不存在", false)
		}
		return t, directDBError(err)
	}
	if mutate {
		if err := ValidateFolderSelection(db, owner, t.Role, t.FolderId); err != nil {
			return t, directDBError(err)
		}
		if _, _, err := directAccess(db, owner, t.BucketID); err != nil {
			return t, err
		}
	}
	return t, nil
}

func directResponse(db *gorm.DB, t models.DirectUploadTask) (DirectUploadResponse, error) {
	r := DirectUploadResponse{ID: t.ID, ClientID: t.ClientID, BucketID: t.BucketID, FolderID: t.FolderId, Filename: t.Filename, Status: t.Status, Transport: t.Transport, ThumbnailStatus: t.ThumbnailStatus, ErrorCode: t.ErrorCode, Message: t.Message, Retryable: t.Retryable, Attempts: t.Attempts, ThumbnailAttempts: t.ThumbnailAttempts, ImageID: t.ImageID, CreatedAt: t.CreatedAt, UpdatedAt: t.UpdatedAt, NextRetryAt: t.NextRetryAt}
	if t.ImageID != 0 {
		var im models.Image
		if err := db.First(&im, t.ImageID).Error; err != nil {
			if !errors.Is(err, gorm.ErrRecordNotFound) {
				return r, directDBError(err)
			}
		} else {
			var s models.Settings
			if err := db.First(&s, 1).Error; err != nil {
				return r, directDBError(err)
			}
			// Encryption is snapshotted per object, not controlled by today's setting.
			if t.Settings.EncryptedStorage {
				s.EncryptedStorage = true
			}
			r.Image = &DirectUploadImage{ID: im.Id, URL: publicurl.BuildForStorage(s, im.Storage, im.BucketId, im.Url), ThumbnailURL: publicurl.BuildForStorage(s, im.Storage, im.BucketId, im.Thumbnail), Filename: im.FileName, FileSize: im.FileSize, MimeType: im.MimeType, Width: im.Width, Height: im.Height, Storage: im.Storage, BucketID: im.BucketId, FolderID: im.FolderId}
		}
	}
	return r, nil
}

func directSnapshot(s models.Settings) models.DirectUploadSettings {
	return models.DirectUploadSettings{CompressImage: s.CompressImage, SaveWebp: s.SaveWebp, Thumbnail: s.Thumbnail, EncryptedStorage: s.EncryptedStorage, WatermarkEnable: s.WatermarkEnable, WatermarkText: s.WatermarkText, WatermarkPos: s.WatermarkPos, WatermarkSize: s.WatermarkSize, WatermarkColor: s.WatermarkColor, WatermarkOpac: s.WatermarkOpac}
}

func directProcessingSettings(s models.DirectUploadSettings) models.Settings {
	return models.Settings{CompressImage: s.CompressImage, SaveWebp: s.SaveWebp, Thumbnail: false, EncryptedStorage: s.EncryptedStorage, WatermarkEnable: s.WatermarkEnable, WatermarkText: s.WatermarkText, WatermarkPos: s.WatermarkPos, WatermarkSize: s.WatermarkSize, WatermarkColor: s.WatermarkColor, WatermarkOpac: s.WatermarkOpac}
}

func directValidRequest(r CreateDirectUploadRequest, s models.Settings) error {
	if r.FolderID < 0 || !directUUID.MatchString(r.ClientID) || r.BucketID <= 0 || !directHash.MatchString(r.SHA256) {
		return directError(400, "invalid_request", "任务标识、存储源或 SHA256 无效", false)
	}
	if r.Filename == "" || len(r.Filename) > 255 || strings.ContainsAny(r.Filename, "/\\\x00\r\n") || r.Filename == "." || r.Filename == ".." {
		return directError(400, "invalid_filename", "文件名无效", false)
	}
	max := int64(s.MaxFileSize)
	if max <= 0 || max > DirectUploadMaxBytes {
		max = DirectUploadMaxBytes
	}
	if r.Size <= 0 || r.Size > max {
		return directError(413, "file_too_large", "文件超过当前上传大小限制（直传最多 32 MiB）", false)
	}
	allowed := false
	for _, m := range strings.Split(s.AllowedTypes, ",") {
		if strings.TrimSpace(m) == r.ContentType {
			allowed = true
		}
	}
	if !allowed || !strings.Contains("|image/png|image/jpeg|image/gif|image/webp|", "|"+r.ContentType+"|") {
		return directError(415, "direct_format_unsupported", "直传仅支持 PNG、JPEG、WebP 和 GIF；SVG 请使用普通上传", false)
	}
	if len(r.Tags) > 20 {
		return directError(400, "invalid_tags", "标签数量超过限制", false)
	}
	for _, tag := range r.Tags {
		if len(tag) > 150 {
			return directError(400, "invalid_tags", "标签长度超过限制", false)
		}
	}
	return nil
}

// directQuota includes reservations in buckets.usage, so legacy uploads also see
// reserved capacity. Conversion to a published charge happens in the same commit.
func directQuota(tx *gorm.DB, bucket int, delta int64) error {
	if delta == 0 {
		return nil
	}
	col := database.UsageColumn(tx)
	q := tx.Model(&models.Buckets{}).Where("id = ?", bucket)
	if delta > 0 {
		q = q.Where("disabled = ? AND (capacity = 0 OR "+col+" + ? <= capacity)", false, delta)
	} else {
		q = q.Where(col+" >= ?", -delta)
	}
	q = q.UpdateColumn("usage", gorm.Expr(col+" + ?", delta))
	if q.Error != nil {
		return q.Error
	}
	if q.RowsAffected != 1 {
		return directError(409, "quota_exceeded", "存储空间不足，请释放容量后重试", false)
	}
	return nil
}

func directFinalKey(t models.DirectUploadTask, ext string) string {
	return path.Join("uploads", "direct", t.CreatedAt.UTC().Format("2006/01"), t.ID+ext)
}
