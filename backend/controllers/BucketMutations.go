package controllers

import (
	"context"
	"errors"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"log"
	"net/http"
	"oneimg/backend/database"
	"oneimg/backend/models"
	"oneimg/backend/services"
	"oneimg/backend/utils/result"
	"oneimg/backend/utils/settings"
	"strconv"
	"time"
)

// UpdateBucketEnabled 启用/禁用存储桶；禁用可逆，保留对象与同步记录。
func UpdateBucketEnabled(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, result.Error(400, "存储源ID无效"))
		return
	}

	var req updateBucketEnabledRequest
	if err := c.ShouldBindJSON(&req); err != nil || req.Enabled == nil {
		c.JSON(http.StatusBadRequest, result.Error(400, "启用状态参数无效"))
		return
	}

	db := database.GetDB()
	var bucket models.Buckets
	if err := db.DB.First(&bucket, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, result.Error(404, "存储源不存在"))
			return
		}
		c.JSON(http.StatusInternalServerError, result.Error(500, "查询存储源失败"))
		return
	}
	if bucket.Type == "default" && !*req.Enabled {
		c.JSON(http.StatusBadRequest, result.Error(400, "本机存储源用于访问回退，不能停用"))
		return
	}

	disabled := !*req.Enabled
	if err := db.DB.Model(&bucket).Update("disabled", disabled).Error; err != nil {
		c.JSON(http.StatusInternalServerError, result.Error(500, "更新存储源状态失败"))
		return
	}
	bucket.Disabled = disabled
	if *req.Enabled {
		services.WakeStorageSyncWorker()
	}

	message := "存储源已启用"
	if disabled {
		message = "存储源已临时停用"
	}
	c.JSON(http.StatusOK, result.Success(message, gin.H{
		"id":       bucket.Id,
		"enabled":  !bucket.Disabled,
		"disabled": bucket.Disabled,
	}))
}

// DeleteBuckets 删除存储桶；仅移除该源上的副本，保留其它源与主记录。
func DeleteBuckets(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, result.Error(400, "存储桶ID无效"))
		return
	}

	db := database.GetDB()
	var bucket models.Buckets
	if err := db.DB.First(&bucket, id).Error; err != nil {
		c.JSON(http.StatusNotFound, result.Error(404, "存储桶不存在"))
		return
	}
	if bucket.Type == "default" {
		c.JSON(http.StatusBadRequest, result.Error(400, "本机存储桶不能删除"))
		return
	}

	// Check the explicit guest target before cancelling tasks or deleting any
	// physical replica. Keep the existing system-default reset behavior below.
	var policy models.Settings
	if err := db.DB.Select("id", "guest_storage").First(&policy, 1).Error; err != nil {
		internalFailure(c, "获取游客存储策略失败", err)
		return
	}
	if policy.GuestStorage == id {
		c.JSON(http.StatusBadRequest, result.Error(400, "该存储源是游客指定存储，请先修改游客存储分配后再删除"))
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Minute)
	defer cancel()
	if err := services.CancelDirectUploadsForBucket(ctx, bucket.Id); err != nil {
		var taskError *services.DirectUploadError
		if errors.As(err, &taskError) {
			c.JSON(taskError.Status, result.Error(taskError.Status, taskError.Message))
		} else {
			c.JSON(http.StatusServiceUnavailable, result.Error(503, "暂停存储源后台任务失败，请稍后重试删除"))
		}
		return
	}
	if err := services.DeleteBucketReplicas(ctx, bucket); err != nil {
		log.Printf("删除存储桶 %d 的文件副本失败：%v", id, err)
		c.JSON(http.StatusBadGateway, result.Error(502, "部分文件副本删除失败，存储源已保留"))
		return
	}

	err = db.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var primaryImages []models.Image
		if err := tx.Where("bucket_id = ?", id).Find(&primaryImages).Error; err != nil {
			return err
		}
		for _, image := range primaryImages {
			var replacement models.ImageStorage
			replacementErr := tx.Where(
				"image_id = ? AND bucket_id != ? AND status = ?",
				image.Id, id, models.ImageStorageStatusSuccess,
			).Order("bucket_id ASC").First(&replacement).Error
			if replacementErr == nil {
				if err := tx.Model(&image).Updates(map[string]any{
					"bucket_id": replacement.BucketID,
					"storage":   replacement.Storage,
					"url":       replacement.URL,
					"thumbnail": replacement.Thumbnail,
				}).Error; err != nil {
					return err
				}
				continue
			}
			if !errors.Is(replacementErr, gorm.ErrRecordNotFound) {
				return replacementErr
			}
			if err := tx.Where("image_id = ?", image.Id).Delete(&models.ImageToTags{}).Error; err != nil {
				return err
			}
			if err := tx.Delete(&image).Error; err != nil {
				return err
			}
		}

		if err := tx.Where("bucket_id = ?", id).Delete(&models.ImageStorage{}).Error; err != nil {
			return err
		}
		if err := tx.Model(&models.Image{}).Where("access_bucket_id = ?", id).Update("access_bucket_id", 0).Error; err != nil {
			return err
		}
		var users []models.User
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Find(&users).Error; err != nil {
			return err
		}
		for _, user := range users {
			filtered := make([]int, 0, len(user.Permission.Buckets))
			for _, bucketID := range user.Permission.Buckets {
				if bucketID != id {
					filtered = append(filtered, bucketID)
				}
			}
			if len(filtered) != len(user.Permission.Buckets) {
				if err := tx.Model(&user).Update("permission", models.Permission{Codes: user.Permission.Codes, Buckets: filtered}).Error; err != nil {
					return err
				}
			}
		}
		return tx.Delete(&models.Buckets{}, id).Error
	})
	if err != nil {
		internalFailure(c, "删除存储桶失败", err)
		return
	}

	setting, settingErr := settings.GetSettings()
	if settingErr != nil {
		log.Printf("获取默认存储桶失败：%v", settingErr)
	} else if setting.DefaultStorage == id {
		if err := db.DB.Model(&models.Settings{}).Where("default_storage = ?", id).Update("default_storage", 1).Error; err != nil {
			log.Printf("更新默认存储桶失败：%v", err)
		}
	}

	c.JSON(http.StatusOK, result.Success("删除成功", nil))
}
