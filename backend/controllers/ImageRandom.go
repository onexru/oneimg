package controllers

import (
	"crypto/rand"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"math/big"
	"oneimg/backend/models"
)

// Uniform sample without replacement. Ordered primary-key offsets avoid a full
// random sort and preserve fairness across sparse/deleted IDs. Bounded by configured random_image_limit.
func sampleRandomImages(query *gorm.DB, total int64, limit int) ([]models.Image, error) {
	if int64(limit) > total {
		limit = int(total)
	}
	chosen := make(map[int64]bool, limit)
	offsets := make([]int64, 0, limit)
	for j := total - int64(limit); j < total; j++ {
		n, err := rand.Int(rand.Reader, big.NewInt(j+1))
		if err != nil {
			return nil, err
		}
		k := n.Int64()
		if chosen[k] {
			k = j
		}
		chosen[k] = true
		offsets = append(offsets, k)
	}
	result := make([]models.Image, 0, limit)
	for _, offset := range offsets {
		var image models.Image
		err := query.Session(&gorm.Session{}).Order("images.id ASC").Offset(int(offset)).Limit(1).Take(&image).Error
		if err == gorm.ErrRecordNotFound {
			continue
		}
		if err != nil {
			return nil, err
		}
		result = append(result, image)
	}
	return result, nil
}
func imagePublicResponseURL(c *gin.Context, s models.Settings, image models.Image) string {
	if image.AccessBucketId > 0 || isHistoricalSVG(image, image.Url) {
		return getRequestBaseURL(c) + ensureLeadingSlash(image.Url)
	}
	return buildImageResponseURL(c, s, image.Storage, image.BucketId, image.Url)
}
