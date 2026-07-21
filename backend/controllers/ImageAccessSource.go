package controllers

import (
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strconv"
	"strings"

	"oneimg/backend/database"
	"oneimg/backend/models"
	"oneimg/backend/utils/result"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

const maxAccessSourceBatchSize = 100

type imageAccessSourceRequest struct {
	BucketID int `json:"bucket_id" binding:"required"`
}

type batchImageAccessSourceRequest struct {
	ImageIDs []int `json:"image_ids" binding:"required"`
	BucketID int   `json:"bucket_id" binding:"required"`
}

type imageAccessSourceError struct {
	status  int
	message string
}

func (err *imageAccessSourceError) Error() string {
	return err.message
}

// UpdateImageAccessSource selects the successful storage replica used by one
// image's stable URL. It never changes the canonical/original image fields.
func UpdateImageAccessSource(c *gin.Context) {
	imageID, err := strconv.Atoi(c.Param("id"))
	if err != nil || imageID <= 0 {
		c.JSON(http.StatusBadRequest, result.Error(400, "图片ID无效"))
		return
	}

	var req imageAccessSourceRequest
	if err := c.ShouldBindJSON(&req); err != nil || req.BucketID <= 0 {
		c.JSON(http.StatusBadRequest, result.Error(400, "存储源ID无效"))
		return
	}

	respondImageAccessSourceUpdate(c, []int{imageID}, req.BucketID, "图片访问源已更新")
}

// BatchUpdateImageAccessSource atomically selects one source shared by all
// chosen images. Every image must already have a successful replica there.
func BatchUpdateImageAccessSource(c *gin.Context) {
	var req batchImageAccessSourceRequest
	if err := c.ShouldBindJSON(&req); err != nil || req.BucketID <= 0 {
		c.JSON(http.StatusBadRequest, result.Error(400, "图片或存储源参数无效"))
		return
	}

	respondImageAccessSourceUpdate(c, req.ImageIDs, req.BucketID, "批量访问源已更新")
}

func respondImageAccessSourceUpdate(c *gin.Context, imageIDs []int, bucketID int, message string) {
	ids, bucket, err := setImageAccessSource(c, imageIDs, bucketID)
	if err != nil {
		var sourceErr *imageAccessSourceError
		if errors.As(err, &sourceErr) {
			c.JSON(sourceErr.status, result.Error(sourceErr.status, sourceErr.message))
			return
		}
		c.JSON(http.StatusInternalServerError, result.Error(500, "更新图片访问源失败"))
		return
	}

	c.JSON(http.StatusOK, result.Success(message, gin.H{
		"image_ids": ids,
		"source": gin.H{
			"bucket_id":   bucket.Id,
			"bucket_name": bucket.Name,
			"bucket_type": bucket.Type,
			"enabled":     !bucket.Disabled,
		},
	}))
}

func setImageAccessSource(c *gin.Context, imageIDs []int, bucketID int) ([]int, models.Buckets, error) {
	ids := normalizeImageIDs(imageIDs)
	if len(ids) == 0 {
		return nil, models.Buckets{}, &imageAccessSourceError{status: http.StatusBadRequest, message: "请选择图片"}
	}
	if len(ids) > maxAccessSourceBatchSize {
		return nil, models.Buckets{}, &imageAccessSourceError{
			status:  http.StatusBadRequest,
			message: fmt.Sprintf("单次最多设置 %d 张图片", maxAccessSourceBatchSize),
		}
	}

	db := database.GetDB()
	if db == nil || db.DB == nil {
		return nil, models.Buckets{}, errors.New("database is not initialized")
	}

	var bucket models.Buckets
	if err := db.DB.First(&bucket, bucketID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, models.Buckets{}, &imageAccessSourceError{status: http.StatusNotFound, message: "存储源不存在"}
		}
		return nil, models.Buckets{}, err
	}
	if bucket.Disabled {
		return nil, models.Buckets{}, &imageAccessSourceError{status: http.StatusConflict, message: "该存储源已停用，请先启用"}
	}

	var images []models.Image
	if err := db.DB.Where("id IN ?", ids).Find(&images).Error; err != nil {
		return nil, models.Buckets{}, err
	}
	if len(images) != len(ids) {
		return nil, models.Buckets{}, &imageAccessSourceError{status: http.StatusNotFound, message: "部分图片不存在"}
	}
	for _, image := range images {
		if !canManageImageAccessSource(c, image) {
			return nil, models.Buckets{}, &imageAccessSourceError{status: http.StatusForbidden, message: "无权修改部分图片的访问源"}
		}
	}

	var replicas []models.ImageStorage
	if err := db.DB.Select("image_id").Where(
		"image_id IN ? AND bucket_id = ? AND status = ?",
		ids, bucketID, models.ImageStorageStatusSuccess,
	).Find(&replicas).Error; err != nil {
		return nil, models.Buckets{}, err
	}
	synchronized := make(map[int]struct{}, len(replicas))
	for _, replica := range replicas {
		synchronized[replica.ImageID] = struct{}{}
	}
	missing := make([]string, 0)
	for _, id := range ids {
		if _, ok := synchronized[id]; !ok {
			missing = append(missing, fmt.Sprintf("#%d", id))
		}
	}
	if len(missing) > 0 {
		return nil, models.Buckets{}, &imageAccessSourceError{
			status:  http.StatusConflict,
			message: "以下图片尚未在该存储源同步成功：" + strings.Join(missing, "、"),
		}
	}

	if err := db.DB.Model(&models.Image{}).Where("id IN ?", ids).Update("access_bucket_id", bucketID).Error; err != nil {
		return nil, models.Buckets{}, err
	}
	return ids, bucket, nil
}

func normalizeImageIDs(imageIDs []int) []int {
	seen := make(map[int]struct{}, len(imageIDs))
	ids := make([]int, 0, len(imageIDs))
	for _, id := range imageIDs {
		if id <= 0 {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		ids = append(ids, id)
	}
	slices.Sort(ids)
	return ids
}

func canManageImageAccessSource(c *gin.Context, image models.Image) bool {
	switch c.GetInt("user_role") {
	case models.RoleAdmin:
		return true
	case models.RoleUser:
		return image.UserId == c.GetInt("user_id")
	case models.RoleGuest:
		return CheckImageAccessPermission(c, image, "image:access:source")
	default:
		return false
	}
}
