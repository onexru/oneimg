package services

import (
	"context"
	"log"
	"oneimg/backend/database"
	"oneimg/backend/models"
	storageSettings "oneimg/backend/utils/settings"
	"sync"
	"time"
)

const storageSyncMaxAttempts = 3

var (
	storageSyncStartOnce sync.Once
	storageSyncWake      = make(chan struct{}, 1)
	// A single process only runs one upload/delete operation at a time. Database
	// compare-and-swap still protects task claiming and persisted state.
	storageReplicaOperationMu sync.Mutex
)

// StartStorageSyncWorker starts the durable, single-worker storage queue.
// Calling it more than once is safe. Tasks which were interrupted while in
// uploading state are returned to pending before the worker starts.
func StartStorageSyncWorker() {
	db := database.GetDB()
	if db == nil || db.DB == nil {
		log.Printf("[storage-sync] database is not initialized; worker not started")
		return
	}
	storageSyncStartOnce.Do(func() {

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
	runDurableWorker(context.Background(), storageSyncWake, processNextStorageSyncTask, nil, 0, nextStorageWorkerRetry)
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
			"EXISTS (SELECT 1 FROM buckets WHERE buckets.id = image_storages.bucket_id AND buckets.disabled = ?) AND "+
			"EXISTS (SELECT 1 FROM images WHERE images.id = image_storages.image_id AND images.deleting = ? AND NOT EXISTS (SELECT 1 FROM folders WHERE folders.id = images.folder_id AND folders.deleting = ?))",
		models.ImageStorageStatusPending, time.Now(), false, false, true,
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
