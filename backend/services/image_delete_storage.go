package services

import (
	"context"
	"errors"
	"oneimg/backend/database"
	"oneimg/backend/models"
	"oneimg/backend/storage"
)

// Deletes exactly one main/thumbnail artifact. Telegram used to delete both on
// either invocation, which could release main quota while a thumbnail failed.
func deleteReplicaArtifact(ctx context.Context, image models.Image, bucket models.Buckets, r models.ImageStorage, thumb bool) error {
	key := r.URL
	if key == "" {
		key = image.Url
	}
	if thumb {
		key = r.Thumbnail
		if key == "" {
			key = image.Thumbnail
		}
	}
	if key == "" {
		return nil
	}
	metadata := r.Metadata
	if bucket.Type == "telegram" {
		db := database.GetDB()
		if db == nil || db.DB == nil {
			return errors.New("database is not initialized")
		}
		var err error
		metadata, err = storage.ResolveTelegramMetadata(db.DB, image.FileName, metadata)
		if err != nil {
			return err
		}
	}
	// Construction doesn't need settings except the legacy S3 factory parameter.
	ref := storage.ArtifactRef(key, metadata, thumb)
	if bucket.Type == "telegram" && storage.MetadataInt(ref.Metadata, "tg_message_id") == 0 && storage.MetadataString(ref.Metadata, "tg_file_id") == "" && (r.Status == models.ImageStorageStatusSuccess || r.Metadata["delete_charged"] == true || r.SyncedAt != nil) {
		return errors.New("Telegram artifact metadata is missing")
	}
	driver, err := storage.New(models.Settings{}, bucket)
	if err != nil {
		return err
	}
	if err = driver.Delete(ctx, ref); err != nil {
		return err
	}
	return nil
}

// Compensation for uncommitted replica uploads is best-effort. Partial Telegram
// IDs are returned to the durable task so a later delete still finds messages.
func deleteRemoteReplica(ctx context.Context, image models.Image, bucket models.Buckets, r models.ImageStorage) error {
	if bucket.Type == "default" {
		return nil
	}
	var failures []error
	for _, thumb := range []bool{false, true} {
		if err := deleteReplicaArtifact(ctx, image, bucket, r, thumb); err != nil {
			failures = append(failures, err)
		}
	}
	return errors.Join(failures...)
}

// Retained for old callers/tests; confirmed absence is typed, not a broad 550 or
// substring check that can mistake denied deletions for successful cleanup.
func isMissingRemoteFileError(err error) bool { return errors.Is(err, storage.ErrNotFound) }
