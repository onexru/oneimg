package images

import (
	"context"
	"errors"
	"fmt"
	"mime/multipart"
	"oneimg/backend/database"
	"oneimg/backend/interfaces"
	"oneimg/backend/models"
	"oneimg/backend/services"
	"oneimg/backend/utils/images"
	"oneimg/backend/utils/uploads"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	storagectrl "oneimg/backend/controllers/storage"
)

// 上传之后的数据库/容量问题，与图片本身无关，单独归类。
const (
	failureRecord = "record_failed"
	failureQuota  = "quota_exceeded"
)

// failedUploadResult 组装逐项失败结果。code 是稳定分类，message 给人看。
func failedUploadResult(file *multipart.FileHeader, message, code string) interfaces.ImageUploadResult {
	return interfaces.ImageUploadResult{Success: false, FileName: file.Filename, Message: message, ErrorCode: code}
}

// describeUploadFailure 把上传链路上的错误翻成「能照着做」的文案。
//
// 以前这里一律写「文件上传失败 / 文件保存失败」，体积超限、像素超限、格式
// 不支持、存储写入失败在响应里完全没有区别，接入方只能靠二分试尺寸反推
// 限制值（issue #58）。分类与文案统一放在 images.DescribeFailure，存储侧
// 的失败由这里的第二段补齐。
func describeUploadFailure(err error, setting *models.Settings) (code, message string) {
	code, message = images.DescribeFailure(err, images.UploadByteLimit(int64(setting.MaxFileSize)))
	if code != "" {
		return code, message
	}
	if errors.Is(err, uploads.ErrStorageWrite) {
		return images.FailureStorage, "写入存储源失败，请稍后重试或更换存储源"
	}
	return images.FailureGeneric, "上传处理失败，请稍后重试"
}

func respondUploadBatch(uc *uploads.UploadContext, results []interfaces.ImageUploadResult, message string, targets int) {
	count := 0
	for _, r := range results {
		if r.Success {
			count++
		}
	}
	status := 200
	if count == 0 {
		status = 400
		message = "所有文件上传失败"
	} else if count < len(results) {
		message = "部分文件上传成功，请查看逐项结果"
	}
	uc.Gin().JSON(status, gin.H{"code": status, "message": message, "data": map[string]any{"files": results, "count": count, "failed_count": len(results) - count, "sync_targets": targets}})
}
func chargeUploadQuota(tx *gorm.DB, bucket models.Buckets, result *interfaces.ImageUploadResult) error {
	size := result.FileSize + result.ThumbnailSize
	if size < 0 {
		return fmt.Errorf("invalid upload size")
	}
	col := database.UsageColumn(tx)
	query := tx.Model(&models.Buckets{}).Where("id = ?", bucket.Id)
	if bucket.Type != "default" && bucket.Type != "telegram" {
		// subtraction form cannot overflow and includes equality at the boundary.
		query = query.Where("capacity = 0 OR ("+col+" <= capacity AND capacity - "+col+" >= ?)", size)
	}
	update := query.UpdateColumn("usage", gorm.Expr(col+" + ?", size))
	if update.Error != nil {
		return update.Error
	}
	if update.RowsAffected != 1 {
		return fmt.Errorf("bucket quota exhausted")
	}
	return nil
}
func cleanupUnpublishedUpload(c *gin.Context, image models.Image, result *interfaces.ImageUploadResult) {
	if image.Storage == "default" {
		storagectrl.CleanupLocalUpload(image)
		return
	}
	// No published Image exists, but materialize a retryable deletion manifest so
	// an SDK failure cannot orphan already stored objects invisibly.
	db := database.GetDB().DB
	image.Id = 0
	image.Deleting = true
	image.FolderId = 0
	err := db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&image).Error; err != nil {
			return err
		}
		replica := models.ImageStorage{ImageID: image.Id, BucketID: image.BucketId, Storage: image.Storage, Status: models.ImageStorageStatusDeleting, URL: image.Url, Thumbnail: image.Thumbnail, FileSize: result.FileSize, ThumbnailSize: result.ThumbnailSize, Metadata: map[string]any{"delete_charged": false}}
		return tx.Create(&replica).Error
	})
	if err == nil {
		if services.DeleteImageReplicas(context.Background(), image) == nil {
			_ = db.Delete(&image).Error
		}
	}
}
