package controllers

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"oneimg/backend/database"
	"oneimg/backend/models"
	"oneimg/backend/utils/result"

	"github.com/gin-gonic/gin"
)

type ImageWithTags struct {
	Id        int           `json:"id" gorm:"primaryKey;autoIncrement;column:id"`
	Url       string        `json:"url" gorm:"column:url"`
	Thumbnail string        `json:"thumbnail" gorm:"column:thumbnail"`
	Filename  string        `json:"filename" gorm:"column:file_name"`
	FileSize  int64         `json:"file_size" gorm:"column:file_size"`
	MimeType  string        `json:"mimeType" gorm:"column:mime_type"`
	Width     int           `json:"width" gorm:"column:width"`
	Height    int           `json:"height" gorm:"column:height"`
	Storage   string        `json:"storage" gorm:"column:storage"`
	BucketId  int           `json:"bucket_id" gorm:"column:bucket_id"`
	UserId    int           `json:"user_id" gorm:"column:user_id"`
	Md5       string        `json:"md5" gorm:"column:md5"`
	Uuid      string        `json:"uuid" gorm:"column:uuid"`
	CreatedAt time.Time     `json:"created_at" gorm:"column:created_at"`
	Tags      []models.Tags `json:"tags" gorm:"-"`
}

// 映射到数据库表
func (ImageWithTags) TableName() string {
	return "images"
}

// GetImageList 获取图片列表
func GetImageList(c *gin.Context) {
	// 基础参数解析
	page, err := strconv.Atoi(c.DefaultQuery("page", "1"))
	if err != nil || page < 1 {
		page = 1
	}
	limit, err := strconv.Atoi(c.DefaultQuery("limit", "20"))
	if err != nil || limit < 1 || limit > 100 {
		limit = 20
	}
	offset := (page - 1) * limit

	// 排序参数
	sortBy := c.DefaultQuery("sort_by", "created_at")
	sortOrder := c.DefaultQuery("sort_order", "desc")
	fieldMapping := map[string]string{
		"created_at": "created_at",
		"file_size":  "file_size",
		"filename":   "file_name",
	}
	dbSortField := fieldMapping[sortBy]
	if dbSortField == "" {
		dbSortField = "created_at"
	}
	if sortOrder != "asc" && sortOrder != "desc" {
		sortOrder = "desc"
	}
	orderClause := dbSortField + " " + sortOrder

	// 搜索/角色参数
	search := c.Query("search")
	role := c.Query("role")

	// 标签参数解析
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

	// 筛选图片ID
	db := database.GetDB().DB
	idQuery := db.Model(&models.Image{}).Select("images.id")

	bucket := c.Query("bucket")
	if bucket != "" && bucket != "all" && bucket != "null" {
		idQuery = idQuery.Where("images.bucket_id = ?", bucket)
	}

	// 基础筛选：角色+权限+搜索
	if role != "" {
		switch role {
		case "admin":
			idQuery = idQuery.Where("images.user_id = ?", 1)
		case "guest":
			idQuery = idQuery.Where("images.user_id != ?", 1)
		}
	}
	if c.GetInt("user_role") != 1 || role == "" {
		idQuery = idQuery.Where("images.uuid = ?", GetUUID(c))
	}
	if search != "" {
		idQuery = idQuery.Where("images.file_name LIKE ?", "%"+search+"%")
	}

	if hasZeroTag || len(filterTagIds) > 0 {
		idQuery = idQuery.Joins("LEFT JOIN image_to_tags ON images.id = image_to_tags.image_id")

		// 合并筛选条件
		if hasZeroTag && len(filterTagIds) > 0 {
			// 无标签 OR 关联指定标签
			idQuery = idQuery.Where("image_to_tags.tag_id IS NULL OR image_to_tags.tag_id IN (?)", filterTagIds)
		} else if hasZeroTag {
			// 仅无标签
			idQuery = idQuery.Where("image_to_tags.tag_id IS NULL")
		} else {
			// 仅指定标签（用IS NOT NULL确保关联有效）
			idQuery = idQuery.Where("image_to_tags.tag_id IN (?) AND image_to_tags.tag_id IS NOT NULL", filterTagIds)
		}

		// 去重：避免重复的图片ID
		idQuery = idQuery.Distinct("images.id")
	}

	// 获取图片ID列表
	var imageIds []int
	imageIds = make([]int, 0)
	if err := idQuery.Order(orderClause).
		Offset(offset).
		Limit(limit).
		Find(&imageIds).Error; err != nil {
		c.JSON(http.StatusInternalServerError, result.Error(500, "筛选图片失败："+err.Error()))
		return
	}

	// 统计总数和分页
	total := int64(len(imageIds))
	countQuery := idQuery.Offset(-1).Limit(-1)
	if hasZeroTag || len(filterTagIds) > 0 {
		countQuery.Distinct("images.id").Count(&total)
	} else {
		countQuery.Count(&total)
	}
	totalPages := (total + int64(limit) - 1) / int64(limit)

	// 查询图片详情
	var images []ImageWithTags
	if len(imageIds) > 0 {
		imageFields := "id, url, thumbnail, file_name, file_size, mime_type, width, height, storage, bucket_id, user_id, md5, uuid, created_at"
		if err := db.Model(&ImageWithTags{}).
			Select(imageFields).
			Where("id IN (?)", imageIds).
			Order(orderClause).
			Find(&images).Error; err != nil {
			c.JSON(http.StatusInternalServerError, result.Error(500, "查询图片详情失败："+err.Error()))
			return
		}
	}

	// 批量查询标签
	defaultTags := []models.Tags{{Id: 0, Name: "默认"}}
	imgIds := make([]int, 0, len(images))
	for _, img := range images {
		imgIds = append(imgIds, img.Id)
	}

	// 查询标签关联
	var imageToTags []models.ImageToTags
	if len(imgIds) > 0 {
		if err := db.Where("image_id IN (?)", imgIds).Find(&imageToTags).Error; err != nil {
			c.JSON(http.StatusInternalServerError, result.Error(500, "查询标签关联失败："+err.Error()))
			return
		}
	}

	// 构建图片ID->标签ID映射
	imgTagMap := make(map[int][]int)
	for _, it := range imageToTags {
		tags := imgTagMap[it.ImageId]
		tags = append(tags, it.TagId)
		imgTagMap[it.ImageId] = tags
	}

	// 收集所有标签ID
	var allTagIds []int
	allTagIds = make([]int, 0)
	for _, tids := range imgTagMap {
		allTagIds = append(allTagIds, tids...)
	}

	// 查询标签信息
	tagMap := make(map[int]models.Tags)
	if len(allTagIds) > 0 {
		var tags []models.Tags
		if err := db.Where("id IN (?)", allTagIds).Find(&tags).Error; err == nil {
			for _, t := range tags {
				tagMap[t.Id] = t
			}
		}
	}

	// 组装标签
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

	// 返回结果
	c.JSON(http.StatusOK, result.Success("ok", gin.H{
		"images":      images,
		"total":       total,
		"page":        page,
		"limit":       limit,
		"total_pages": totalPages,
	}))
}
