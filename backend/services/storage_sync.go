package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"oneimg/backend/database"
	"oneimg/backend/models"
	"oneimg/backend/utils/buckets"
	"oneimg/backend/utils/ftp"
	storageS3 "oneimg/backend/utils/s3"
	"oneimg/backend/utils/securestorage"
	storageSettings "oneimg/backend/utils/settings"
	"oneimg/backend/utils/telegram"
	"oneimg/backend/utils/webdav"

	"github.com/aws/aws-sdk-go-v2/aws"
	awss3 "github.com/aws/aws-sdk-go-v2/service/s3"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const storageSyncPollInterval = 3 * time.Second
const storageSyncMaxAttempts = 3

var (
	storageSyncStartOnce sync.Once
	storageSyncWake      = make(chan struct{}, 1)
	// A single process only runs one upload/delete operation at a time. Database
	// compare-and-swap still protects task claiming and persisted state.
	storageReplicaOperationMu sync.Mutex
)

type localStorageArtifact struct {
	MainPath      string
	ThumbnailPath string
	URL           string
	Thumbnail     string
	FileName      string
	MimeType      string
	FileSize      int64
	ThumbnailSize int64
}

// StartStorageSyncWorker starts the durable, single-worker storage queue.
// Calling it more than once is safe. Tasks which were interrupted while in
// uploading state are returned to pending before the worker starts.
func StartStorageSyncWorker() {
	storageSyncStartOnce.Do(func() {
		db := database.GetDB()
		if db == nil || db.DB == nil {
			log.Printf("[storage-sync] database is not initialized; worker not started")
			return
		}

		result := db.DB.Model(&models.ImageStorage{}).
			Where("status = ?", models.ImageStorageStatusUploading).
			Updates(map[string]any{
				"status":        models.ImageStorageStatusPending,
				"error":         "",
				"started_at":    nil,
				"next_retry_at": nil,
			})
		if result.Error != nil {
			log.Printf("[storage-sync] failed to recover interrupted tasks: %v", result.Error)
		} else if result.RowsAffected > 0 {
			log.Printf("[storage-sync] recovered %d interrupted task(s)", result.RowsAffected)
		}

		go runStorageSyncWorker()
	})
	WakeStorageSyncWorker()
}

// WakeStorageSyncWorker asks the worker to poll immediately. The signal is
// deliberately lossy because pending work is durable in the database.
func WakeStorageSyncWorker() {
	select {
	case storageSyncWake <- struct{}{}:
	default:
	}
}

func runStorageSyncWorker() {
	ticker := time.NewTicker(storageSyncPollInterval)
	defer ticker.Stop()

	for {
		for processNextStorageSyncTask() {
		}

		select {
		case <-storageSyncWake:
		case <-ticker.C:
		}
	}
}

func processNextStorageSyncTask() bool {
	storageReplicaOperationMu.Lock()
	defer storageReplicaOperationMu.Unlock()

	db := database.GetDB()
	if db == nil || db.DB == nil {
		return false
	}
	setting, err := storageSettings.GetSettings()
	if err != nil {
		log.Printf("[storage-sync] failed to load feature switch: %v", err)
		return false
	}
	if !setting.MultiStorageSync {
		return false
	}

	var replica models.ImageStorage
	lookup := db.DB.Where(
		"status = ? AND (next_retry_at IS NULL OR next_retry_at <= ?) AND "+
			"EXISTS (SELECT 1 FROM buckets WHERE buckets.id = image_storages.bucket_id AND buckets.disabled = ?)",
		models.ImageStorageStatusPending, time.Now(), false,
	).
		Order("id ASC").
		Limit(1).
		Find(&replica)
	if lookup.Error != nil {
		log.Printf("[storage-sync] failed to find pending task: %v", lookup.Error)
		return false
	}
	if lookup.RowsAffected == 0 {
		return false
	}

	now := time.Now()
	claim := db.DB.Model(&models.ImageStorage{}).
		Where("id = ? AND status = ?", replica.ID, models.ImageStorageStatusPending).
		Updates(map[string]any{
			"status":        models.ImageStorageStatusUploading,
			"error":         "",
			"started_at":    &now,
			"next_retry_at": nil,
		})
	if claim.Error != nil {
		log.Printf("[storage-sync] failed to claim task %d: %v", replica.ID, claim.Error)
		return false
	}
	if claim.RowsAffected == 0 {
		return true
	}
	replica.Status = models.ImageStorageStatusUploading
	replica.StartedAt = &now

	taskContext, cancelTask := context.WithTimeout(context.Background(), 5*time.Minute)
	metadata, syncErr := synchronizeReplica(taskContext, &replica)
	cancelTask()
	if syncErr != nil {
		if err := markStorageSyncFailed(replica.ID, syncErr, metadata); err != nil {
			log.Printf("[storage-sync] task %d failed and status update failed: %v (upload error: %v)", replica.ID, err, syncErr)
		} else {
			log.Printf("[storage-sync] task %d failed: %v", replica.ID, syncErr)
		}
	}

	return true
}

func synchronizeReplica(ctx context.Context, replica *models.ImageStorage) (map[string]any, error) {
	db := database.GetDB().DB

	var image models.Image
	if err := db.First(&image, replica.ImageID).Error; err != nil {
		return nil, fmt.Errorf("load image %d: %w", replica.ImageID, err)
	}

	var bucket models.Buckets
	if err := db.First(&bucket, replica.BucketID).Error; err != nil {
		return nil, fmt.Errorf("load bucket %d: %w", replica.BucketID, err)
	}
	if bucket.Disabled {
		return nil, fmt.Errorf("storage source %d is temporarily disabled", bucket.Id)
	}

	artifact, err := buildLocalStorageArtifact(image)
	if err != nil {
		return nil, err
	}

	if err := checkStorageCapacity(bucket, artifact.FileSize+artifact.ThumbnailSize); err != nil {
		return nil, err
	}

	var metadata map[string]any
	switch bucket.Type {
	case "default":
		// The local canonical file already is the successfully synchronized copy.
	case "s3", "r2":
		err = uploadArtifactToS3(ctx, bucket, artifact)
	case "webdav":
		err = uploadArtifactToWebDAV(ctx, bucket, artifact)
	case "ftp":
		err = uploadArtifactToFTP(bucket, artifact)
	case "telegram":
		metadata, err = uploadArtifactToTelegram(bucket, artifact)
	default:
		err = fmt.Errorf("unsupported storage type %q", bucket.Type)
	}
	if err != nil {
		cleanupReplicaAfterFailedUpload(image, bucket, *replica, metadata, err)
		return metadata, err
	}

	if err := completeStorageSync(replica.ID, bucket, artifact, metadata); err != nil {
		var latest models.ImageStorage
		if loadErr := db.First(&latest, replica.ID).Error; loadErr == nil && latest.Status == models.ImageStorageStatusSuccess {
			return metadata, nil
		}
		cleanupReplicaAfterFailedUpload(image, bucket, *replica, metadata, err)
		return metadata, err
	}

	log.Printf("[storage-sync] image %d synchronized to bucket %d (%s)", image.Id, bucket.Id, bucket.Type)
	return metadata, nil
}

func cleanupReplicaAfterFailedUpload(image models.Image, bucket models.Buckets, replica models.ImageStorage, metadata map[string]any, uploadErr error) {
	if bucket.Type == "default" {
		return
	}
	replica.URL = image.Url
	replica.Thumbnail = image.Thumbnail
	replica.Metadata = metadata
	cleanupContext, cancelCleanup := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancelCleanup()
	if cleanupErr := deleteRemoteReplica(cleanupContext, image, bucket, replica); cleanupErr != nil {
		log.Printf("[storage-sync] cleanup after failed upload also failed (task=%d, upload=%v): %v", replica.ID, uploadErr, cleanupErr)
	}
}

func buildLocalStorageArtifact(image models.Image) (localStorageArtifact, error) {
	mainPath, err := canonicalLocalPath(image.Url)
	if err != nil {
		return localStorageArtifact{}, fmt.Errorf("resolve local image path: %w", err)
	}
	mainInfo, err := os.Stat(mainPath)
	if err != nil {
		return localStorageArtifact{}, fmt.Errorf("stat local image %q: %w", mainPath, err)
	}
	if !mainInfo.Mode().IsRegular() {
		return localStorageArtifact{}, fmt.Errorf("local image is not a regular file: %s", mainPath)
	}

	artifact := localStorageArtifact{
		MainPath: mainPath,
		URL:      image.Url,
		FileName: image.FileName,
		MimeType: image.MimeType,
		FileSize: mainInfo.Size(),
	}
	if artifact.FileName == "" {
		artifact.FileName = filepath.Base(mainPath)
	}
	if artifact.MimeType == "" {
		artifact.MimeType = "application/octet-stream"
	}

	if image.Thumbnail != "" {
		thumbnailPath, pathErr := canonicalLocalPath(image.Thumbnail)
		if pathErr != nil {
			return localStorageArtifact{}, fmt.Errorf("resolve local thumbnail path: %w", pathErr)
		}
		thumbnailInfo, statErr := os.Stat(thumbnailPath)
		if statErr != nil {
			return localStorageArtifact{}, fmt.Errorf("stat local thumbnail %q: %w", thumbnailPath, statErr)
		}
		if !thumbnailInfo.Mode().IsRegular() {
			return localStorageArtifact{}, fmt.Errorf("local thumbnail is not a regular file: %s", thumbnailPath)
		}
		artifact.ThumbnailPath = thumbnailPath
		artifact.Thumbnail = image.Thumbnail
		artifact.ThumbnailSize = thumbnailInfo.Size()
	}

	return artifact, nil
}

func canonicalLocalPath(publicPath string) (string, error) {
	path := strings.TrimSpace(publicPath)
	if path == "" {
		return "", errors.New("path is empty")
	}
	if strings.HasPrefix(path, "http://") || strings.HasPrefix(path, "https://") {
		return "", errors.New("absolute URL is not a local file path")
	}
	if queryIndex := strings.IndexByte(path, '?'); queryIndex >= 0 {
		path = path[:queryIndex]
	}
	path = strings.TrimPrefix(path, "/")
	cleanPath := filepath.Clean(filepath.FromSlash(path))
	if cleanPath == "." || cleanPath == "" || filepath.IsAbs(cleanPath) || cleanPath == ".." || strings.HasPrefix(cleanPath, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("unsafe local path %q", publicPath)
	}

	root, err := filepath.Abs(".")
	if err != nil {
		return "", err
	}
	fullPath, err := filepath.Abs(filepath.Join(root, cleanPath))
	if err != nil {
		return "", err
	}
	relative, err := filepath.Rel(root, fullPath)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("local path escapes application directory: %q", publicPath)
	}
	return fullPath, nil
}

func checkStorageCapacity(bucket models.Buckets, size int64) error {
	if size < 0 {
		return errors.New("replica size cannot be negative")
	}
	if bucket.Type == "default" || bucket.Type == "telegram" || bucket.Capacity == 0 {
		return nil
	}
	required := uint64(size)
	if required > bucket.Capacity || bucket.Usage > bucket.Capacity-required {
		return fmt.Errorf("bucket %d has insufficient capacity", bucket.Id)
	}
	return nil
}

func uploadArtifactToS3(ctx context.Context, bucket models.Buckets, artifact localStorageArtifact) error {
	setting, err := storageSettings.GetSettings()
	if err != nil {
		return fmt.Errorf("load settings: %w", err)
	}
	client, err := storageS3.NewS3Client(setting, bucket)
	if err != nil {
		return err
	}

	remoteBucket := ""
	switch bucket.Type {
	case "s3":
		remoteBucket = buckets.ConvertToS3Bucket(bucket.Config).S3Bucket
	case "r2":
		remoteBucket = buckets.ConvertToR2Bucket(bucket.Config).R2Bucket
	}
	if remoteBucket == "" {
		return errors.New("remote bucket name is empty")
	}

	mainFile, err := os.Open(artifact.MainPath)
	if err != nil {
		return err
	}
	mainEncrypted, err := securestorage.IsEncryptedFile(artifact.MainPath)
	if err != nil {
		mainFile.Close()
		return err
	}
	_, uploadErr := client.PutObject(ctx, &awss3.PutObjectInput{
		Bucket:      aws.String(remoteBucket),
		Key:         aws.String(remoteObjectKey(artifact.URL)),
		Body:        mainFile,
		ContentType: aws.String(synchronizedContentType(artifact.MimeType, mainEncrypted)),
	})
	closeErr := mainFile.Close()
	if uploadErr != nil {
		return fmt.Errorf("upload main image: %w", uploadErr)
	}
	if closeErr != nil {
		return closeErr
	}

	if artifact.ThumbnailPath == "" {
		return nil
	}
	thumbnailFile, err := os.Open(artifact.ThumbnailPath)
	if err != nil {
		return err
	}
	thumbnailEncrypted, err := securestorage.IsEncryptedFile(artifact.ThumbnailPath)
	if err != nil {
		thumbnailFile.Close()
		return err
	}
	_, uploadErr = client.PutObject(ctx, &awss3.PutObjectInput{
		Bucket:      aws.String(remoteBucket),
		Key:         aws.String(remoteObjectKey(artifact.Thumbnail)),
		Body:        thumbnailFile,
		ContentType: aws.String(synchronizedContentType("image/webp", thumbnailEncrypted)),
	})
	closeErr = thumbnailFile.Close()
	if uploadErr != nil {
		return fmt.Errorf("upload thumbnail: %w", uploadErr)
	}
	return closeErr
}

func uploadArtifactToWebDAV(ctx context.Context, bucket models.Buckets, artifact localStorageArtifact) error {
	config := buckets.ConvertToWebDavBucket(bucket.Config)
	client := webdav.Client(webdav.Config{
		BaseURL:  config.WebdavURL,
		Username: config.WebdavUser,
		Password: config.WebdavPass,
		Timeout:  30 * time.Second,
	})

	mainFile, err := os.Open(artifact.MainPath)
	if err != nil {
		return err
	}
	uploadErr := client.WebDAVUpload(ctx, artifact.URL, mainFile)
	closeErr := mainFile.Close()
	if uploadErr != nil {
		return fmt.Errorf("upload main image: %w", uploadErr)
	}
	if closeErr != nil {
		return closeErr
	}

	if artifact.ThumbnailPath == "" {
		return nil
	}
	thumbnailFile, err := os.Open(artifact.ThumbnailPath)
	if err != nil {
		return err
	}
	uploadErr = client.WebDAVUpload(ctx, artifact.Thumbnail, thumbnailFile)
	closeErr = thumbnailFile.Close()
	if uploadErr != nil {
		return fmt.Errorf("upload thumbnail: %w", uploadErr)
	}
	return closeErr
}

func uploadArtifactToFTP(bucket models.Buckets, artifact localStorageArtifact) error {
	config := buckets.ConvertToFTPBucket(bucket.Config)
	client := ftp.NewFTPUtil(ftp.FTPConfig{
		Host:     config.FTPHost,
		Port:     config.FTPPort,
		User:     config.FTPUser,
		Password: config.FTPPass,
		Timeout:  30,
	})
	defer client.Close()

	mainBytes, err := os.ReadFile(artifact.MainPath)
	if err != nil {
		return err
	}
	if err := client.UploadImage(artifact.URL, mainBytes, synchronizedContentType(artifact.MimeType, securestorage.IsEncrypted(mainBytes))); err != nil {
		return fmt.Errorf("upload main image: %w", err)
	}

	if artifact.ThumbnailPath == "" {
		return nil
	}
	thumbnailBytes, err := os.ReadFile(artifact.ThumbnailPath)
	if err != nil {
		return err
	}
	if err := client.UploadImage(artifact.Thumbnail, thumbnailBytes, synchronizedContentType("image/webp", securestorage.IsEncrypted(thumbnailBytes))); err != nil {
		return fmt.Errorf("upload thumbnail: %w", err)
	}
	return nil
}

func uploadArtifactToTelegram(bucket models.Buckets, artifact localStorageArtifact) (map[string]any, error) {
	config := buckets.ConvertToTelegramBucket(bucket.Config)
	client := telegram.NewClient(config.TGBotToken)
	client.Timeout = 20 * time.Second
	client.Retry = 3

	mainBytes, err := os.ReadFile(artifact.MainPath)
	if err != nil {
		return nil, err
	}
	fileID, messageID, err := uploadTelegramArtifactBytes(
		client,
		config.TGReceivers,
		mainBytes,
		artifact.FileName,
		fmt.Sprintf("上传图片: %s", artifact.FileName),
	)
	if err != nil {
		return nil, fmt.Errorf("upload main image: %w", err)
	}

	metadata := map[string]any{
		"tg_file_id":    fileID,
		"tg_message_id": messageID,
	}
	if artifact.ThumbnailPath == "" {
		return metadata, nil
	}

	thumbnailBytes, err := os.ReadFile(artifact.ThumbnailPath)
	if err != nil {
		return metadata, err
	}
	thumbnailFileID, thumbnailMessageID, err := uploadTelegramArtifactBytes(
		client,
		config.TGReceivers,
		thumbnailBytes,
		"thumbnail_"+artifact.FileName,
		fmt.Sprintf("缩略图: %s", artifact.FileName),
	)
	if err != nil {
		return metadata, fmt.Errorf("upload thumbnail: %w", err)
	}
	metadata["tg_thumbnail_file_id"] = thumbnailFileID
	metadata["tg_thumbnail_message_id"] = thumbnailMessageID
	return metadata, nil
}

func synchronizedContentType(contentType string, encrypted bool) string {
	if encrypted {
		return "application/octet-stream"
	}
	return contentType
}

func uploadTelegramArtifactBytes(client *telegram.Config, chatID string, data []byte, filename, caption string) (string, int, error) {
	if securestorage.IsEncrypted(data) {
		return client.UploadDocumentByBytes(chatID, data, filename+".oneimg", caption)
	}
	return client.UploadPhotoByBytes(chatID, data, filename, caption)
}

func remoteObjectKey(path string) string {
	return strings.TrimPrefix(strings.ReplaceAll(path, "\\", "/"), "/")
}

func completeStorageSync(replicaID int, bucket models.Buckets, artifact localStorageArtifact, metadata map[string]any) error {
	db := database.GetDB().DB
	now := time.Now()
	totalSize := artifact.FileSize + artifact.ThumbnailSize
	metadataValue, err := storageMetadataValue(metadata)
	if err != nil {
		return err
	}

	return db.Transaction(func(tx *gorm.DB) error {
		var current models.ImageStorage
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&current, replicaID).Error; err != nil {
			return err
		}
		if current.Status == models.ImageStorageStatusSuccess {
			return nil
		}
		if current.Status != models.ImageStorageStatusUploading {
			return fmt.Errorf("task %d is no longer uploading (status=%s)", replicaID, current.Status)
		}

		if bucket.Type != "default" && totalSize > 0 {
			totalSizeUint := uint64(totalSize)
			usageUpdate := tx.Model(&models.Buckets{}).
				Where("id = ? AND (capacity = 0 OR type IN ('telegram','default') OR usage + ? <= capacity)", bucket.Id, totalSizeUint).
				UpdateColumn("usage", gorm.Expr("usage + ?", totalSizeUint))
			if usageUpdate.Error != nil {
				return usageUpdate.Error
			}
			if usageUpdate.RowsAffected == 0 {
				return fmt.Errorf("bucket %d has insufficient capacity", bucket.Id)
			}
		}

		updates := map[string]any{
			"storage":        bucket.Type,
			"status":         models.ImageStorageStatusSuccess,
			"url":            artifact.URL,
			"thumbnail":      artifact.Thumbnail,
			"file_size":      artifact.FileSize,
			"thumbnail_size": artifact.ThumbnailSize,
			"error":          "",
			"metadata":       metadataValue,
			"started_at":     nil,
			"next_retry_at":  nil,
			"synced_at":      &now,
		}
		result := tx.Model(&models.ImageStorage{}).
			Where("id = ? AND status = ?", replicaID, models.ImageStorageStatusUploading).
			Updates(updates)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return fmt.Errorf("task %d was modified before completion", replicaID)
		}
		return nil
	})
}

func markStorageSyncFailed(replicaID int, syncErr error, metadata map[string]any) error {
	db := database.GetDB().DB
	return db.Transaction(func(tx *gorm.DB) error {
		var current models.ImageStorage
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&current, replicaID).Error; err != nil {
			return err
		}
		if current.Status != models.ImageStorageStatusUploading {
			return nil
		}

		attempts := current.RetryCount + 1
		status := models.ImageStorageStatusFailed
		var nextRetryAt *time.Time
		if attempts < storageSyncMaxAttempts {
			status = models.ImageStorageStatusPending
			retryTime := time.Now().Add(time.Duration(1<<(attempts-1)) * 5 * time.Second)
			nextRetryAt = &retryTime
		}
		updates := map[string]any{
			"status":        status,
			"error":         syncErr.Error(),
			"retry_count":   attempts,
			"started_at":    nil,
			"next_retry_at": nextRetryAt,
		}
		if metadata != nil {
			metadataValue, err := storageMetadataValue(metadata)
			if err != nil {
				return err
			}
			updates["metadata"] = metadataValue
		}
		return tx.Model(&models.ImageStorage{}).
			Where("id = ? AND status = ?", replicaID, models.ImageStorageStatusUploading).
			Updates(updates).Error
	})
}

func storageMetadataValue(metadata map[string]any) (any, error) {
	if metadata == nil {
		return nil, nil
	}
	encoded, err := json.Marshal(metadata)
	if err != nil {
		return nil, fmt.Errorf("encode storage metadata: %w", err)
	}
	return string(encoded), nil
}

// BackfillImageStorages creates one successful replica record for the current
// singular storage fields on every legacy Image. It is safe to run repeatedly.
func BackfillImageStorages() error {
	db := database.GetDB()
	if db == nil || db.DB == nil {
		return errors.New("database is not initialized")
	}

	var bucketList []models.Buckets
	if err := db.DB.Find(&bucketList).Error; err != nil {
		return err
	}
	bucketByID := make(map[int]models.Buckets, len(bucketList))
	for _, bucket := range bucketList {
		bucketByID[bucket.Id] = bucket
	}

	var telegramRows []models.ImageTeleGram
	if err := db.DB.Find(&telegramRows).Error; err != nil {
		return err
	}
	telegramByFilename := make(map[string]models.ImageTeleGram, len(telegramRows))
	for _, row := range telegramRows {
		if _, exists := telegramByFilename[row.FileName]; !exists {
			telegramByFilename[row.FileName] = row
		}
	}

	var images []models.Image
	result := db.DB.Order("id ASC").FindInBatches(&images, 200, func(tx *gorm.DB, _ int) error {
		for _, image := range images {
			bucketID := image.BucketId
			if bucketID == 0 {
				bucketID = 1
			}
			storageType := image.Storage
			if storageType == "" {
				storageType = bucketByID[bucketID].Type
			}
			if storageType == "" {
				storageType = "default"
			}

			thumbnailSize := int64(0)
			if storageType == "default" && image.Thumbnail != "" {
				if thumbnailPath, err := canonicalLocalPath(image.Thumbnail); err == nil {
					if info, err := os.Stat(thumbnailPath); err == nil && info.Mode().IsRegular() {
						thumbnailSize = info.Size()
					}
				}
			}

			var metadata map[string]any
			if storageType == "telegram" {
				if legacy, ok := telegramByFilename[image.FileName]; ok {
					metadata = telegramMetadata(legacy)
				}
			}

			replica := models.ImageStorage{
				ImageID:       image.Id,
				BucketID:      bucketID,
				Storage:       storageType,
				Status:        models.ImageStorageStatusSuccess,
				URL:           image.Url,
				Thumbnail:     image.Thumbnail,
				FileSize:      image.FileSize,
				ThumbnailSize: thumbnailSize,
				Metadata:      metadata,
				SyncedAt:      timePointer(image.CreatedAt),
			}
			if err := tx.Clauses(clause.OnConflict{
				Columns:   []clause.Column{{Name: "image_id"}, {Name: "bucket_id"}},
				DoNothing: true,
			}).Create(&replica).Error; err != nil {
				return err
			}
		}
		return nil
	})
	return result.Error
}

func telegramMetadata(row models.ImageTeleGram) map[string]any {
	return map[string]any{
		"tg_file_id":              row.TGFileId,
		"tg_thumbnail_file_id":    row.TGThumbnailFileId,
		"tg_message_id":           row.TGMessageId,
		"tg_thumbnail_message_id": row.TGThumbnailMessageId,
	}
}

func timePointer(value time.Time) *time.Time {
	if value.IsZero() {
		return nil
	}
	copy := value
	return &copy
}

// DeleteImageReplicas deletes every remote replica first and the local
// canonical files last. Successfully deleted replicas have their bucket usage
// released. On a remote error, the local source is retained for a safe retry.
func DeleteImageReplicas(ctx context.Context, image models.Image) error {
	storageReplicaOperationMu.Lock()
	defer storageReplicaOperationMu.Unlock()

	db := database.GetDB()
	if db == nil || db.DB == nil {
		return errors.New("database is not initialized")
	}

	var replicas []models.ImageStorage
	if err := db.DB.Where("image_id = ?", image.Id).Order("id ASC").Find(&replicas).Error; err != nil {
		return err
	}
	if len(replicas) == 0 {
		bucketID := image.BucketId
		if bucketID == 0 {
			bucketID = 1
		}
		storageType := image.Storage
		if storageType == "" {
			storageType = "default"
		}
		replicas = []models.ImageStorage{{
			ImageID:   image.Id,
			BucketID:  bucketID,
			Storage:   storageType,
			Status:    models.ImageStorageStatusSuccess,
			URL:       image.Url,
			Thumbnail: image.Thumbnail,
			FileSize:  image.FileSize,
		}}
	}

	var localReplicas []models.ImageStorage
	var deleteErrors []error
	for _, replica := range replicas {
		var bucket models.Buckets
		if err := db.DB.First(&bucket, replica.BucketID).Error; err != nil {
			deleteErrors = append(deleteErrors, fmt.Errorf("load bucket %d: %w", replica.BucketID, err))
			continue
		}
		if bucket.Type == "default" || replica.Storage == "default" {
			localReplicas = append(localReplicas, replica)
			continue
		}
		if replica.Status == models.ImageStorageStatusPending {
			if err := removeReplicaRecord(db.DB, bucket, replica); err != nil {
				deleteErrors = append(deleteErrors, err)
			}
			continue
		}

		if err := deleteRemoteReplica(ctx, image, bucket, replica); err != nil {
			_ = db.DB.Model(&models.ImageStorage{}).Where("id = ?", replica.ID).Updates(map[string]any{
				"status": models.ImageStorageStatusFailed,
				"error":  err.Error(),
			}).Error
			deleteErrors = append(deleteErrors, fmt.Errorf("delete image %d from bucket %d: %w", image.Id, bucket.Id, err))
			continue
		}
		if err := removeReplicaRecord(db.DB, bucket, replica); err != nil {
			deleteErrors = append(deleteErrors, err)
		}
	}

	if len(deleteErrors) > 0 {
		return errors.Join(deleteErrors...)
	}

	if image.Storage == "default" || len(localReplicas) > 0 {
		if err := deleteLocalImageFiles(image); err != nil {
			return err
		}
	}
	for _, replica := range localReplicas {
		if replica.ID != 0 {
			if err := db.DB.Delete(&models.ImageStorage{}, replica.ID).Error; err != nil {
				deleteErrors = append(deleteErrors, err)
			}
		}
	}
	return errors.Join(deleteErrors...)
}

// DeleteBucketReplicas removes all replicas from one remote bucket without
// deleting their canonical Image rows. The default local bucket is protected.
func DeleteBucketReplicas(ctx context.Context, bucket models.Buckets) error {
	storageReplicaOperationMu.Lock()
	defer storageReplicaOperationMu.Unlock()

	if bucket.Type == "default" {
		return errors.New("the default local bucket cannot be deleted")
	}
	db := database.GetDB()
	if db == nil || db.DB == nil {
		return errors.New("database is not initialized")
	}

	var replicas []models.ImageStorage
	if err := db.DB.Where("bucket_id = ?", bucket.Id).Order("id ASC").Find(&replicas).Error; err != nil {
		return err
	}

	var deleteErrors []error
	for _, replica := range replicas {
		var image models.Image
		if err := db.DB.First(&image, replica.ImageID).Error; err != nil {
			if !errors.Is(err, gorm.ErrRecordNotFound) {
				deleteErrors = append(deleteErrors, err)
				continue
			}
			image = models.Image{
				Id:        replica.ImageID,
				Url:       replica.URL,
				Thumbnail: replica.Thumbnail,
				FileName:  filepath.Base(replica.URL),
			}
		}
		if replica.Status == models.ImageStorageStatusPending {
			if err := removeReplicaRecord(db.DB, bucket, replica); err != nil {
				deleteErrors = append(deleteErrors, err)
			}
			continue
		}

		if err := deleteRemoteReplica(ctx, image, bucket, replica); err != nil {
			_ = db.DB.Model(&models.ImageStorage{}).Where("id = ?", replica.ID).Updates(map[string]any{
				"status": models.ImageStorageStatusFailed,
				"error":  err.Error(),
			}).Error
			deleteErrors = append(deleteErrors, fmt.Errorf("delete replica %d: %w", replica.ID, err))
			continue
		}
		if err := removeReplicaRecord(db.DB, bucket, replica); err != nil {
			deleteErrors = append(deleteErrors, err)
		}
	}
	return errors.Join(deleteErrors...)
}

func deleteRemoteReplica(ctx context.Context, image models.Image, bucket models.Buckets, replica models.ImageStorage) error {
	mainPath := replica.URL
	if mainPath == "" {
		mainPath = image.Url
	}
	thumbnailPath := replica.Thumbnail
	if thumbnailPath == "" {
		thumbnailPath = image.Thumbnail
	}

	switch bucket.Type {
	case "s3", "r2":
		return deleteS3Replica(ctx, bucket, mainPath, thumbnailPath)
	case "webdav":
		return deleteWebDAVReplica(ctx, bucket, mainPath, thumbnailPath)
	case "ftp":
		return deleteFTPReplica(bucket, mainPath, thumbnailPath)
	case "telegram":
		return deleteTelegramReplica(image, bucket, replica)
	case "default":
		return nil
	default:
		return fmt.Errorf("unsupported storage type %q", bucket.Type)
	}
}

func deleteS3Replica(ctx context.Context, bucket models.Buckets, mainPath, thumbnailPath string) error {
	setting, err := storageSettings.GetSettings()
	if err != nil {
		return err
	}
	client, err := storageS3.NewS3Client(setting, bucket)
	if err != nil {
		return err
	}
	remoteBucket := ""
	if bucket.Type == "s3" {
		remoteBucket = buckets.ConvertToS3Bucket(bucket.Config).S3Bucket
	} else {
		remoteBucket = buckets.ConvertToR2Bucket(bucket.Config).R2Bucket
	}

	var deleteErrors []error
	for _, path := range []string{mainPath, thumbnailPath} {
		if path == "" {
			continue
		}
		_, err := client.DeleteObject(ctx, &awss3.DeleteObjectInput{
			Bucket: aws.String(remoteBucket),
			Key:    aws.String(remoteObjectKey(path)),
		})
		if err != nil {
			deleteErrors = append(deleteErrors, err)
		}
	}
	return errors.Join(deleteErrors...)
}

func deleteWebDAVReplica(ctx context.Context, bucket models.Buckets, mainPath, thumbnailPath string) error {
	config := buckets.ConvertToWebDavBucket(bucket.Config)
	client := webdav.Client(webdav.Config{
		BaseURL:  config.WebdavURL,
		Username: config.WebdavUser,
		Password: config.WebdavPass,
		Timeout:  30 * time.Second,
	})
	var deleteErrors []error
	for _, path := range []string{mainPath, thumbnailPath} {
		if path == "" {
			continue
		}
		if err := client.WebDAVDelete(ctx, path); err != nil && !isMissingRemoteFileError(err) {
			deleteErrors = append(deleteErrors, err)
		}
	}
	return errors.Join(deleteErrors...)
}

func deleteFTPReplica(bucket models.Buckets, mainPath, thumbnailPath string) error {
	config := buckets.ConvertToFTPBucket(bucket.Config)
	client := ftp.NewFTPUtil(ftp.FTPConfig{
		Host:     config.FTPHost,
		Port:     config.FTPPort,
		User:     config.FTPUser,
		Password: config.FTPPass,
		Timeout:  30,
	})
	defer client.Close()

	var deleteErrors []error
	for _, path := range []string{mainPath, thumbnailPath} {
		if path == "" {
			continue
		}
		if err := client.DeleteImage(path); err != nil && !isMissingRemoteFileError(err) {
			deleteErrors = append(deleteErrors, err)
		}
	}
	return errors.Join(deleteErrors...)
}

func deleteTelegramReplica(image models.Image, bucket models.Buckets, replica models.ImageStorage) error {
	metadata := replica.Metadata
	legacy := models.ImageTeleGram{}
	db := database.GetDB().DB
	legacyErr := db.Where("file_name = ?", image.FileName).First(&legacy).Error
	if legacyErr != nil && !errors.Is(legacyErr, gorm.ErrRecordNotFound) {
		return legacyErr
	}
	if metadataInt(metadata, "tg_message_id") == 0 && metadataInt(metadata, "tg_thumbnail_message_id") == 0 {
		if legacyErr == nil {
			metadata = telegramMetadata(legacy)
		}
	}

	config := buckets.ConvertToTelegramBucket(bucket.Config)
	client := telegram.NewClient(config.TGBotToken)
	client.Timeout = 20 * time.Second
	client.Retry = 3
	uploader := telegram.NewTelegramUploader(client)

	var deleteErrors []error
	for _, messageID := range []int{
		metadataInt(metadata, "tg_message_id"),
		metadataInt(metadata, "tg_thumbnail_message_id"),
	} {
		if messageID <= 0 {
			continue
		}
		if err := uploader.DeletePhoto(config.TGReceivers, messageID); err != nil && !isMissingRemoteFileError(err) {
			deleteErrors = append(deleteErrors, err)
		}
	}
	if len(deleteErrors) == 0 && legacy.Id != 0 {
		if err := db.Delete(&legacy).Error; err != nil {
			deleteErrors = append(deleteErrors, err)
		}
	}
	return errors.Join(deleteErrors...)
}

func metadataInt(metadata map[string]any, key string) int {
	if metadata == nil {
		return 0
	}
	switch value := metadata[key].(type) {
	case int:
		return value
	case int32:
		return int(value)
	case int64:
		return int(value)
	case uint:
		return int(value)
	case uint32:
		return int(value)
	case uint64:
		return int(value)
	case float64:
		return int(value)
	case json.Number:
		parsed, _ := value.Int64()
		return int(parsed)
	case string:
		parsed, _ := strconv.Atoi(value)
		return parsed
	default:
		return 0
	}
}

func isMissingRemoteFileError(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "not found") ||
		strings.Contains(message, "no such file") ||
		strings.Contains(message, "文件不存在") ||
		strings.Contains(message, "状态码：404") ||
		strings.Contains(message, "status code: 404") ||
		strings.Contains(message, " 404") ||
		strings.Contains(message, " 550")
}

func removeReplicaRecord(db *gorm.DB, bucket models.Buckets, replica models.ImageStorage) error {
	return db.Transaction(func(tx *gorm.DB) error {
		if replica.Status == models.ImageStorageStatusSuccess && bucket.Type != "default" {
			totalSize := replica.FileSize + replica.ThumbnailSize
			if totalSize > 0 {
				totalSizeUint := uint64(totalSize)
				if err := tx.Model(&models.Buckets{}).Where("id = ?", bucket.Id).
					UpdateColumn("usage", gorm.Expr("CASE WHEN usage >= ? THEN usage - ? ELSE 0 END", totalSizeUint, totalSizeUint)).Error; err != nil {
					return err
				}
			}
		}
		if replica.ID != 0 {
			if err := tx.Model(&models.Image{}).
				Where("id = ? AND access_bucket_id = ?", replica.ImageID, bucket.Id).
				Update("access_bucket_id", 0).Error; err != nil {
				return err
			}
			return tx.Delete(&models.ImageStorage{}, replica.ID).Error
		}
		return nil
	})
}

func deleteLocalImageFiles(image models.Image) error {
	var deleteErrors []error
	for _, path := range []string{image.Url, image.Thumbnail} {
		if path == "" {
			continue
		}
		localPath, err := canonicalLocalPath(path)
		if err != nil {
			deleteErrors = append(deleteErrors, err)
			continue
		}
		if err := os.Remove(localPath); err != nil && !errors.Is(err, os.ErrNotExist) {
			deleteErrors = append(deleteErrors, err)
		}
	}
	return errors.Join(deleteErrors...)
}
