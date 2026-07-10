package controllers

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"oneimg/backend/database"
	"oneimg/backend/models"
	"oneimg/backend/utils/result"
	"oneimg/backend/utils/settings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type ImageWithTags struct {
	Id              int                          `json:"id" gorm:"primaryKey;autoIncrement;column:id"`
	Url             string                       `json:"url" gorm:"column:url"`
	Thumbnail       string                       `json:"thumbnail" gorm:"column:thumbnail"`
	Filename        string                       `json:"filename" gorm:"column:file_name"`
	FileSize        int64                        `json:"file_size" gorm:"column:file_size"`
	MimeType        string                       `json:"mimeType" gorm:"column:mime_type"`
	Width           int                          `json:"width" gorm:"column:width"`
	Height          int                          `json:"height" gorm:"column:height"`
	Storage         string                       `json:"storage" gorm:"column:storage"`
	BucketId        int                          `json:"bucket_id" gorm:"column:bucket_id"`
	UserId          int                          `json:"user_id" gorm:"column:user_id"`
	UploaderRole    int                          `json:"uploader_role" gorm:"-"`
	Md5             string                       `json:"md5" gorm:"column:md5"`
	Uuid            string                       `json:"uuid" gorm:"column:uuid"`
	CreatedAt       time.Time                    `json:"created_at" gorm:"column:created_at"`
	Tags            []models.Tags                `json:"tags" gorm:"-"`
	StorageStatuses []ImageStorageStatusResponse `json:"storage_statuses" gorm:"-"`
}

// 映射到数据库表
func (ImageWithTags) TableName() string {
	return "images"
}

// GetImageList 获取图片列表
func GetImageList(c *gin.Context) {
	page, err := strconv.Atoi(c.DefaultQuery("page", "1"))
	if err != nil || page < 1 {
		page = 1
	}
	limit, err := strconv.Atoi(c.DefaultQuery("limit", "20"))
	if err != nil || limit < 1 || limit > 100 {
		limit = 20
	}
	offset := (page - 1) * limit

	sortBy := c.DefaultQuery("sort_by", "created_at")
	sortOrder := c.DefaultQuery("sort_order", "desc")
	fieldMapping := map[string]string{
		"created_at": "images.created_at",
		"file_size":  "images.file_size",
		"filename":   "images.file_name",
	}
	dbSortField, ok := fieldMapping[sortBy]
	if !ok {
		dbSortField = "created_at"
	}
	if sortOrder != "asc" && sortOrder != "desc" {
		sortOrder = "desc"
	}
	orderClause := dbSortField + " " + sortOrder

	search := c.Query("search")
	roleFilter := c.Query("role")
	roleID := c.GetInt("user_role")
	userID := c.GetInt("user_id")
	userUUID := GetUUID(c)

	var hasZeroTag bool
	var filterTagIds []int
	tagIdStr := c.Query("tags")
	if tagIdStr != "" {
		tagIdList := strings.SplitSeq(tagIdStr, ",")
		for s := range tagIdList {
			trimmed := strings.TrimSpace(s)
			tid, err := strconv.Atoi(trimmed)
			if err != nil {
				c.JSON(http.StatusBadRequest, result.Error(400, "标签ID格式错误："+trimmed))
				return
			}
			if tid == 0 {
				hasZeroTag = true
			} else if tid > 0 {
				filterTagIds = append(filterTagIds, tid)
			}
		}
	}

	db := database.GetDB().DB
	idQuery := db.Model(&models.Image{}).Select("images.id")

	// 存储桶筛选
	bucket := c.Query("bucket")
	if bucket != "" && bucket != "all" && bucket != "null" {
		idQuery = idQuery.Where(
			"EXISTS (SELECT 1 FROM image_storages WHERE image_storages.image_id = images.id AND image_storages.bucket_id = ?)",
			bucket,
		)
	}

	// 仅超级管理员可使用 role=admin / role=guest 全局筛选
	if roleFilter != "" {
		if roleID != models.RoleAdmin {
			c.JSON(http.StatusBadRequest, result.Error(400, "无权限查看全局用户图片"))
			return
		}
		switch roleFilter {
		case "admin":
			idQuery = idQuery.Where("images.user_id = ?", userID)
		case "guest":
			idQuery = idQuery.Joins("LEFT JOIN users ON images.user_id = users.id").
				Where("users.id IS NULL")
		case "user":
			idQuery = idQuery.Joins("LEFT JOIN users ON images.user_id = users.id").
				Where("users.id IS NOT NULL")
			if userID != models.SuperAdminID {
				idQuery = idQuery.Where("images.user_id = ? and users.id != ?", models.SuperAdminID, userID)
			} else {
				idQuery = idQuery.Where("images.user_id != ? or users.id != ?", models.SuperAdminID, userID)
			}
		}
	} else {
		// 普通角色：只能查看自身数据
		switch roleID {
		case models.RoleUser, models.RoleAdmin:
			idQuery = idQuery.Where("images.user_id = ?", userID)
		case models.RoleGuest:
			idQuery = idQuery.Where("images.uuid = ?", userUUID)
		}
	}

	if search != "" {
		idQuery = idQuery.Where("images.file_name LIKE ?", "%"+search+"%")
	}

	if hasZeroTag || len(filterTagIds) > 0 {
		idQuery = idQuery.Joins("LEFT JOIN image_to_tags ON images.id = image_to_tags.image_id")
		if hasZeroTag && len(filterTagIds) > 0 {
			idQuery = idQuery.Where("image_to_tags.tag_id IS NULL OR image_to_tags.tag_id IN (?)", filterTagIds)
		} else if hasZeroTag {
			idQuery = idQuery.Where("image_to_tags.tag_id IS NULL")
		} else {
			idQuery = idQuery.Where("image_to_tags.tag_id IN (?) AND image_to_tags.tag_id IS NOT NULL", filterTagIds)
		}
		idQuery = idQuery.Distinct("images.id")
	}

	var imageIds []int
	if err := idQuery.Order(orderClause).Offset(offset).Limit(limit).Find(&imageIds).Error; err != nil {
		c.JSON(http.StatusInternalServerError, result.Error(500, "筛选图片失败："+err.Error()))
		return
	}

	var total int64
	countQuery := idQuery.Session(&gorm.Session{})
	countQuery.Offset(-1).Limit(-1)
	if hasZeroTag || len(filterTagIds) > 0 {
		countQuery.Distinct("images.id").Count(&total)
	} else {
		countQuery.Count(&total)
	}
	totalPages := (total + int64(limit) - 1) / int64(limit)

	var images []ImageWithTags
	if len(imageIds) > 0 {
		imageFields := "id, url, thumbnail, file_name, file_size, mime_type, width, height, storage, bucket_id, user_id, md5, uuid, created_at"
		if err := db.Model(&models.Image{}).
			Select(imageFields).
			Where("id IN (?)", imageIds).
			Order(orderClause).
			Find(&images).Error; err != nil {
			c.JSON(http.StatusInternalServerError, result.Error(500, "查询图片详情失败："+err.Error()))
			return
		}
	}

	defaultTags := []models.Tags{{Id: 0, Name: "默认"}}
	imgIds := make([]int, 0, len(images))
	for _, img := range images {
		imgIds = append(imgIds, img.Id)
	}

	var imageToTags []models.ImageToTags
	if len(imgIds) > 0 {
		if err := db.Where("image_id IN (?)", imgIds).Find(&imageToTags).Error; err != nil {
			c.JSON(http.StatusInternalServerError, result.Error(500, "查询标签关联失败："+err.Error()))
			return
		}
	}

	// 映射图片-标签
	imgTagMap := make(map[int][]int)
	for _, it := range imageToTags {
		imgTagMap[it.ImageId] = append(imgTagMap[it.ImageId], it.TagId)
	}

	// 批量查标签基础信息
	var allTagIds []int
	for _, tids := range imgTagMap {
		allTagIds = append(allTagIds, tids...)
	}
	tagMap := make(map[int]models.Tags)
	if len(allTagIds) > 0 {
		var tags []models.Tags
		if err := db.Where("id IN (?)", allTagIds).Find(&tags).Error; err == nil {
			for _, t := range tags {
				tagMap[t.Id] = t
			}
		}
	}

	for i := range images {
		tids := imgTagMap[images[i].Id]
		if len(tids) == 0 {
			images[i].Tags = defaultTags
		} else {
			tagList := make([]models.Tags, 0, len(tids))
			for _, tid := range tids {
				if t, ok := tagMap[tid]; ok {
					tagList = append(tagList, t)
				}
			}
			images[i].Tags = tagList
		}
	}

	// 收集所有唯一上传用户ID
	uidSet := make(map[int]bool)
	var uidList []int
	for _, img := range images {
		uid := img.UserId
		if !uidSet[uid] {
			uidSet[uid] = true
			uidList = append(uidList, uid)
		}
	}

	// 批量查询用户ID-角色映射
	type userRoleDTO struct {
		ID   int `gorm:"column:id"`
		Role int `gorm:"column:role"`
	}
	var userRoleList []userRoleDTO
	roleMap := make(map[int]int)
	if len(uidList) > 0 {
		err := db.Model(&models.User{}).
			Select("id, role").
			Where("id IN (?)", uidList).
			Find(&userRoleList).Error
		if err == nil {
			for _, item := range userRoleList {
				roleMap[item.ID] = item.Role
			}
		}
	}

	// 赋值，无用户记录则兜底游客角色 models.RoleGuest
	for i := range images {
		uid := images[i].UserId
		if r, exist := roleMap[uid]; exist {
			images[i].UploaderRole = r
		} else {
			images[i].UploaderRole = models.RoleGuest
		}
	}

	setting, err := settings.GetSettings()
	if err != nil {
		c.JSON(http.StatusInternalServerError, result.Error(500, "获取系统配置失败："+err.Error()))
		return
	}
	storageStatuses, err := loadImageStorageStatuses(imgIds, setting)
	if err != nil {
		c.JSON(http.StatusInternalServerError, result.Error(500, "获取存储同步状态失败："+err.Error()))
		return
	}
	for i := range images {
		images[i].Url = applyPublicImageURL(setting, images[i].Storage, images[i].BucketId, images[i].Url)
		images[i].Thumbnail = applyPublicImageURL(setting, images[i].Storage, images[i].BucketId, images[i].Thumbnail)
		images[i].StorageStatuses = storageStatuses[images[i].Id]
	}

	c.JSON(http.StatusOK, result.Success("ok", gin.H{
		"images":      images,
		"total":       total,
		"page":        page,
		"limit":       limit,
		"total_pages": totalPages,
	}))
}
