package uploads

import (
	"context"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"log"
	"oneimg/backend/database"
	"oneimg/backend/models"
	"oneimg/backend/storage"
	"oneimg/backend/utils/images"
	"time"
)

// Compensation survives a disconnected client, but remains bounded. If an
// upstream refuses cleanup, retain an uncharged deletion fence, never publish
// or account a partially uploaded object as a successful image.
func compensateUpload(driver storage.Driver, ref storage.Ref) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	return driver.Delete(ctx, ref)
}
func retainCleanupManifest(c *gin.Context, bucket *models.Buckets, main, thumb string, p *images.ProcessedImage, metadata map[string]any) {
	db := database.GetDB()
	if db == nil || db.DB == nil {
		log.Printf("[storage]未完成上传的清理记录无法保存：数据库未初始化")
		return
	}
	values := map[string]any{"delete_charged": false}
	for k, v := range metadata {
		values[k] = v
	}
	image := models.Image{Url: main, Thumbnail: thumb, FileName: p.UniqueFileName, MimeType: p.MimeType, FileSize: int64(len(p.CompressedBytes)), Storage: bucket.Type, BucketId: bucket.Id, UserId: c.GetInt("user_id"), Deleting: true}
	err := db.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&image).Error; err != nil {
			return err
		}
		row := models.ImageStorage{ImageID: image.Id, BucketID: bucket.Id, Storage: bucket.Type, Status: models.ImageStorageStatusDeleting, URL: main, Thumbnail: thumb, FileSize: image.FileSize, ThumbnailSize: publishedThumbnailSize(thumb, p.ThumbnailBytes), Metadata: values, Error: "未完成上传的存储文件清理失败，请重试删除"}
		return tx.Create(&row).Error
	})
	if err != nil {
		log.Printf("[storage]未完成上传的清理记录保存失败")
	}
}
