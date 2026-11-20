package images

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"

	"oneimg/backend/database"
	"oneimg/backend/middlewares"
	"oneimg/backend/models"
	"oneimg/backend/services"
	"oneimg/backend/utils/result"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// imageDeleteError 让「取图 / 鉴权 / 删副本」各步的失败带上 HTTP 状态码：
// 单条删除直接透传给客户端，批量删除则把状态码折算成每一张图的结果。
type imageDeleteError struct {
	status  int
	message string
}

func (e *imageDeleteError) Error() string { return e.message }

// DeleteImage 删除图片：先删各存储物理副本，再事务释放容量并删库记录。
func DeleteImage(c *gin.Context) {
	idStr := c.Param("id")
	if idStr == "" {
		c.JSON(http.StatusBadRequest, result.Error(400, "图片ID不能为空"))
		return
	}

	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, result.Error(400, "图片ID无效"))
		return
	}

	image, err := loadImageForDeletion(c, int(id))
	if err != nil {
		respondImageDeleteError(c, err)
		return
	}

	deleteCtx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Minute)
	defer cancel()

	if err := removeImageArtifacts(deleteCtx, image); err != nil {
		respondImageDeleteError(c, err)
		return
	}

	c.JSON(http.StatusOK, result.Success("删除成功，对应存储容量已释放", nil))
}

func respondImageDeleteError(c *gin.Context, err error) {
	var deleteErr *imageDeleteError
	if errors.As(err, &deleteErr) {
		c.JSON(deleteErr.status, result.Error(deleteErr.status, deleteErr.message))
		return
	}
	log.Printf("删除图片失败：%v", err)
	c.JSON(http.StatusInternalServerError, result.Error(500, "删除图片失败"))
}

// loadImageForDeletion 取图并校验删除权限，单条与批量删除共用同一套判定。
func loadImageForDeletion(c *gin.Context, id int) (models.Image, error) {
	var image models.Image
	db := database.GetDB()
	if db == nil || db.DB == nil {
		return image, &imageDeleteError{http.StatusInternalServerError, "数据库未初始化"}
	}
	if err := db.DB.First(&image, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return image, &imageDeleteError{http.StatusNotFound, "图片不存在"}
		}
		log.Printf("查询图片 %d 失败：%v", id, err)
		return image, &imageDeleteError{http.StatusInternalServerError, "查询图片失败"}
	}
	if !CheckImageAccessPermission(c, image, "image:delete") {
		return image, &imageDeleteError{http.StatusForbidden, "无权访问或删除此图片"}
	}
	return image, nil
}

// removeImageArtifacts 删除该图的存储副本，再在事务里删标签关联与图片记录。
//
// 副本清理（DeleteImageReplicas）自己按副本释放配额，这里绝不能再按 file_size
// 扣一次 —— 那会变成双倍扣费。
func removeImageArtifacts(ctx context.Context, image models.Image) error {
	if err := services.CancelDirectUploadsForImage(ctx, image.Id); err != nil {
		log.Printf("暂停图片 %d 的后台直传任务失败：%v", image.Id, err)
		return &imageDeleteError{http.StatusServiceUnavailable, "暂停图片后台任务失败，请稍后重试删除"}
	}

	if err := services.DeleteImageReplicas(ctx, image); err != nil {
		log.Printf("删除图片 %d 的存储副本失败：%v", image.Id, err)
		return &imageDeleteError{http.StatusBadGateway, "部分存储源删除失败，文件记录已保留，可稍后重试"}
	}

	db := database.GetDB().DB
	err := db.Transaction(func(tx *gorm.DB) error {
		// Replica cleanup owns per-artifact quota release. Never charge or release
		// canonical file_size a second time here.
		var remaining int64
		if err := tx.Model(&models.ImageStorage{}).Where("image_id = ?", image.Id).Count(&remaining).Error; err != nil {
			return err
		}
		if remaining != 0 {
			return fmt.Errorf("replica cleanup incomplete")
		}
		if err := tx.Where("image_id = ?", image.Id).Delete(&models.ImageToTags{}).Error; err != nil {
			return err
		}
		return tx.Delete(&image).Error
	})
	if err != nil {
		log.Printf("图片 %d 事务处理失败 err=%v", image.Id, err)
		return &imageDeleteError{http.StatusInternalServerError, "删除图片记录或更新存储容量失败"}
	}
	return nil
}

// CheckImageAccessPermission 校验当前用户是否可操作目标图片。
// 规则：超管图片仅超管可动；本人或超管放行；否则需 requiredPerm；游客用 UUID+MD5。
func CheckImageAccessPermission(c *gin.Context, image models.Image, requiredPerm string) bool {
	userID, role := c.GetInt("user_id"), c.GetInt("user_role")
	// Guest IDs can never take the real-user/admin fast path, even for legacy
	// positive IDs or a chosen fingerprint matching a registered user.
	if role == models.RoleGuest {
		return guestOwnsImage(c, image)
	}
	if userID <= 0 || (role != models.RoleUser && role != models.RoleAdmin) {
		return false
	}
	if image.UserId == models.SuperAdminID && userID != models.SuperAdminID {
		return false
	}
	if userID == image.UserId || userID == models.SuperAdminID {
		return true
	}
	user, exists := middlewares.GetCurrentUser(c)
	return role == models.RoleAdmin && exists && requiredPerm != "" && user.Permission.HasPermission(requiredPerm)
}
