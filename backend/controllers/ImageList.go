package controllers

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"oneimg/backend/database"
	"oneimg/backend/models"
	"oneimg/backend/services"
	"oneimg/backend/utils/result"
	"oneimg/backend/utils/settings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// ImageWithTags 列表接口返回的图片及其标签、存储同步状态。
type ImageWithTags struct {
	Deleting        bool                         `json:"deleting" gorm:"column:deleting"`
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
	FolderId        int                          `json:"folder_id" gorm:"column:folder_id"`
	AccessBucketId  int                          `json:"access_bucket_id" gorm:"column:access_bucket_id"`
	UserId          int                          `json:"user_id" gorm:"column:user_id"`
	UploaderName    string                       `json:"uploader_name" gorm:"-"`
	UploaderRole    int                          `json:"uploader_role" gorm:"-"`
	Md5             string                       `json:"md5" gorm:"column:md5"`
	Uuid            string                       `json:"uuid" gorm:"column:uuid"`
	CreatedAt       time.Time                    `json:"created_at" gorm:"column:created_at"`
	Tags            []models.Tags                `json:"tags" gorm:"-"`
	StorageStatuses []ImageStorageStatusResponse `json:"storage_statuses" gorm:"-"`
}

// TableName 映射 images 表。
func (ImageWithTags) TableName() string {
	return "images"
}

// GetManagedImageList is the dedicated admin view; query parameters cannot
// accidentally turn it into the personal folder browser.
func GetManagedImageList(c *gin.Context) {
	c.Set("gallery_scope", "all")
	GetImageList(c)
}

// GetImageList 分页获取图片列表（按角色过滤可见范围）。
func GetImageList(c *gin.Context) {
	c.Header("Cache-Control", "private, no-store")
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
		dbSortField = "images.created_at"
	}
	if sortOrder != "asc" && sortOrder != "desc" {
		sortOrder = "desc"
	}
	orderClause := dbSortField + " " + sortOrder + ", images.id " + sortOrder

	search := c.Query("search")
	roleFilter := c.Query("role")
	roleID := c.GetInt("user_role")
	userID := c.GetInt("user_id")
	userUUID := CurrentOwnerKey(c)

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
	idQuery := db.Model(&models.Image{})
	if rawFolder, present := c.GetQuery("folder_id"); present {
		folderID, err := parseFolderID(rawFolder, true)
		if err != nil {
			folderHTTPError(c, err)
			return
		}
		if err := services.ValidateFolderRead(db.WithContext(c.Request.Context()), userID, roleID, folderID); err != nil {
			folderHTTPError(c, err)
			return
		}
		idQuery = idQuery.Where("images.folder_id = ? AND images.user_id = ?", folderID, userID)
	} else if _, present := c.Request.URL.Query()["folder_id"]; present {
		c.JSON(400, result.Error(400, "文件夹ID无效"))
		return
	}
	switch c.DefaultQuery("deletion", "active") {
	case "active":
		idQuery = idQuery.Where("images.deleting = ?", false)
	case "deleting":
		idQuery = idQuery.Where("images.deleting = ?", true)
	case "all": // Explicit management list; owner/role permissions below still apply.
	default:
		c.JSON(400, result.Error(400, "删除状态筛选无效"))
		return
	}

	// 存储桶筛选
	bucket := c.Query("bucket")
	if bucket != "" && bucket != "all" && bucket != "null" {
		idQuery = idQuery.Where(
			"EXISTS (SELECT 1 FROM image_storages WHERE image_storages.image_id = images.id AND image_storages.bucket_id = ?)",
			bucket,
		)
	}

	// Explicit personal scope never expands, even if a stale role filter is sent.
	scope := c.Query("scope")
	if fixed := c.GetString("gallery_scope"); fixed != "" {
		scope = fixed
	}
	if scope != "" && scope != "mine" && scope != "all" {
		c.JSON(400, result.Error(400, "图库范围无效"))
		return
	}
	global := scope == "all" || (scope == "" && roleFilter != "")
	if global {
		if roleID != models.RoleAdmin || userID <= 0 {
			status := 403
			if scope == "" {
				status = 400
			} // Legacy clients keep their error contract.
			c.JSON(status, result.Error(status, "仅管理员可查看总图库"))
			return
		}
		switch roleFilter {
		case "", "all":
		case "admin":
			idQuery = idQuery.Where("EXISTS (SELECT 1 FROM users WHERE users.id = images.user_id AND users.role = ?)", models.RoleAdmin)
		case "user":
			idQuery = idQuery.Where("EXISTS (SELECT 1 FROM users WHERE users.id = images.user_id AND users.role = ?)", models.RoleUser)
		case "guest":
			idQuery = idQuery.Where("images.user_id < 0 OR NOT EXISTS (SELECT 1 FROM users WHERE users.id = images.user_id) OR EXISTS (SELECT 1 FROM users WHERE users.id = images.user_id AND users.role = ?)", models.RoleGuest)
		default:
			c.JSON(400, result.Error(400, "上传者角色无效"))
			return
		}
	} else {
		switch roleID {
		case models.RoleUser, models.RoleAdmin:
			idQuery = idQuery.Where("images.user_id = ? AND images.user_id > 0", userID)
		case models.RoleGuest:
			idQuery = scopeGuestImages(idQuery, userID, userUUID)
		default:
			c.JSON(403, result.Error(403, "无权查看图库"))
			return
		}
	}

	if search != "" {
		idQuery = idQuery.Where("images.file_name LIKE ?", "%"+search+"%")
	}

	idQuery = filterImageTags(idQuery, filterTagIds, hasZeroTag)
	var total int64
	if err := idQuery.Session(&gorm.Session{}).Count(&total).Error; err != nil {
		c.JSON(http.StatusInternalServerError, result.Error(500, "统计图片失败"))
		return
	}
	var imageIds []int
	if err := idQuery.Session(&gorm.Session{}).Order(orderClause).Offset(offset).Limit(limit).Pluck("images.id", &imageIds).Error; err != nil {
		c.JSON(http.StatusInternalServerError, result.Error(500, "筛选图片失败"))
		return
	}
	totalPages := (total + int64(limit) - 1) / int64(limit)

	var images []ImageWithTags
	if len(imageIds) > 0 {
		imageFields := "id, deleting, url, thumbnail, file_name, file_size, mime_type, width, height, storage, bucket_id, access_bucket_id, folder_id, user_id, md5, uuid, created_at"
		if err := db.Model(&models.Image{}).
			Select(imageFields).
			Where("id IN (?)", imageIds).
			Order(orderClause).
			Find(&images).Error; err != nil {
			internalFailure(c, "查询图片详情失败", err)
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
			internalFailure(c, "查询标签关联失败", err)
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
		ID       int    `gorm:"column:id"`
		Role     int    `gorm:"column:role"`
		Username string `gorm:"column:username"`
	}
	var userRoleList []userRoleDTO
	roleMap := make(map[int]int)
	nameMap := make(map[int]string)
	if len(uidList) > 0 {
		err := db.Model(&models.User{}).
			Select("id, role, username").
			Where("id IN (?)", uidList).
			Find(&userRoleList).Error
		if err == nil {
			for _, item := range userRoleList {
				roleMap[item.ID] = item.Role
				nameMap[item.ID] = item.Username
			}
		}
	}

	// 赋值，无用户记录则兜底游客角色 models.RoleGuest
	for i := range images {
		uid := images[i].UserId
		if r, exist := roleMap[uid]; exist {
			images[i].UploaderRole = r
			images[i].UploaderName = nameMap[uid]
		} else {
			images[i].UploaderRole = models.RoleGuest
			images[i].UploaderName = "游客"
		}
	}

	setting, err := settings.GetSettings()
	if err != nil {
		internalFailure(c, "获取系统配置失败", err)
		return
	}
	storageStatuses, err := loadImageStorageStatuses(imgIds, setting)
	if err != nil {
		internalFailure(c, "获取存储同步状态失败", err)
		return
	}
	for i := range images {
		if images[i].AccessBucketId == 0 {
			images[i].Url = applyPublicImageURL(setting, images[i].Storage, images[i].BucketId, images[i].Url)
			images[i].Thumbnail = applyPublicImageURL(setting, images[i].Storage, images[i].BucketId, images[i].Thumbnail)
		}
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
