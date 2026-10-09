package controllers

import (
	"context"
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

	db := database.GetDB().DB
	var image models.Image
	if err := db.First(&image, uint(id)).Error; err != nil {
		c.JSON(http.StatusNotFound, result.Error(404, "图片不存在"))
		return
	}

	if !CheckImageAccessPermission(c, image, "image:delete") {
		c.JSON(http.StatusForbidden, result.Error(403, "无权访问或删除此图片"))
		return
	}

	deleteCtx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Minute)
	defer cancel()

	if err := services.CancelDirectUploadsForImage(deleteCtx, image.Id); err != nil {
		c.JSON(http.StatusServiceUnavailable, result.Error(503, "暂停图片后台任务失败，请稍后重试删除"))
		return
	}

	if err := services.DeleteImageReplicas(deleteCtx, image); err != nil {
		log.Printf("删除图片 %d 的存储副本失败：%v", image.Id, err)
		c.JSON(http.StatusBadGateway, result.Error(502, "部分存储源删除失败，文件记录已保留，可稍后重试"))
		return
	}

	err = db.Transaction(func(tx *gorm.DB) error {
		// Replica cleanup owns per-artifact quota release. Never charge or release
		// canonical file_size a second time here.
		var remaining int64
		if err := tx.Model(&models.ImageStorage{}).Where("image_id = ?",image.Id).Count(&remaining).Error; err != nil { return err }
		if remaining != 0 { return fmt.Errorf("replica cleanup incomplete") }
		if err := tx.Where("image_id = ?", image.Id).Delete(&models.ImageToTags{}).Error; err != nil {
			return err
		}
		if err := tx.Delete(&image).Error; err != nil {
			return err
		}
		return nil
	})

	if err != nil {
		log.Printf("图片 %d 事务处理失败 err=%v", image.Id, err)
		c.JSON(http.StatusInternalServerError, result.Error(500, "删除图片记录或更新存储容量失败"))
		return
	}

	c.JSON(http.StatusOK, result.Success("删除成功，对应存储容量已释放", nil))
}

// CheckImageAccessPermission 校验当前用户是否可操作目标图片。
// 规则：超管图片仅超管可动；本人或超管放行；否则需 requiredPerm；游客用 UUID+MD5。
func CheckImageAccessPermission(c *gin.Context, image models.Image, requiredPerm string) bool {
	userID, role := c.GetInt("user_id"), c.GetInt("user_role")
	// Guest IDs can never take the real-user/admin fast path, even for legacy
	// positive IDs or a chosen fingerprint matching a registered user.
	if role == models.RoleGuest { return guestOwnsImage(c,image) }
	if userID <= 0 || (role != models.RoleUser && role != models.RoleAdmin) { return false }
	if image.UserId == models.SuperAdminID && userID != models.SuperAdminID { return false }
	if userID == image.UserId || userID == models.SuperAdminID { return true }
	user, exists := middlewares.GetCurrentUser(c)
	return role == models.RoleAdmin && exists && requiredPerm != "" && user.Permission.HasPermission(requiredPerm)
}
