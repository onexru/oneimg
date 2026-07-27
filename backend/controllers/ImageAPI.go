package controllers

import (
	"errors"
	"math/rand"
	"net/http"
	"oneimg/backend/database"
	"oneimg/backend/models"
	"oneimg/backend/utils/result"
	"oneimg/backend/utils/settings"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// 初始化随机数种子
var randomGenerator = rand.New(rand.NewSource(time.Now().UnixNano()))

// 定义返回的图片结构体
type RandomImageResponse struct {
	Image string `json:"image"` // 图片文件名
	Url   string `json:"url"`   // 图片完整访问地址
}

func GetRandomImages(c *gin.Context) {
	tag := c.Query("tag")
	model := c.DefaultQuery("model", "json")
	limitStr := c.DefaultQuery("limit", "1")
	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit < 1 || limit > 20 {
		c.JSON(http.StatusBadRequest, result.Error(400, "limit参数错误，必须是1-20之间的整数"))
		return
	}

	setting, err := settings.GetSettings()
	if err != nil {
		c.JSON(http.StatusInternalServerError, result.Error(500, "获取系统配置失败"))
		return
	}

	if setting.RandomGraph == false {
		c.JSON(http.StatusInternalServerError, result.Error(500, "随机图功能未开启"))
		return
	}

	db := database.GetDB().DB

	var randomGraph models.RandomGraph
	if err = db.Model(&models.RandomGraph{}).Where("id = ?", 1).Take(&randomGraph).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			randomGraph = models.RandomGraph{ID: 1, UserIds: []int{}, TagIds: []int{}}
		} else if strings.Contains(err.Error(), "cannot unmarshal") {
			randomGraph = models.RandomGraph{ID: 1, UserIds: []int{}, TagIds: []int{}}
		} else {
			c.JSON(http.StatusInternalServerError, result.Error(500, "获取随机图配置失败"))
			return
		}
	}
	query := db.Model(&models.Image{})
	if len(randomGraph.UserIds) > 0 {
		query = query.Where("images.user_id IN ?", randomGraph.UserIds)
	}

	if len(randomGraph.TagIds) > 0 {
		query = applyRandomGraphTagFilter(query, db, randomGraph.TagIds)
	}

	if tag != "" {
		if tag == "默认标签" {
			query = query.Where("NOT EXISTS (SELECT 1 FROM image_to_tags WHERE image_to_tags.image_id = images.id)")
		} else {
			tagSubQuery := db.Model(&models.ImageToTags{}).Select("1").
				Joins("JOIN tags ON image_to_tags.tag_id = tags.id").
				Where("image_to_tags.image_id = images.id").
				Where("tags.name = ?", tag)
			query = query.Where("EXISTS (?)", tagSubQuery)
		}
	}

	var total int64
	if err := query.Session(&gorm.Session{}).Count(&total).Error; err != nil {
		c.JSON(http.StatusInternalServerError, result.Error(500, "获取图片总数失败"))
		return
	}

	if total == 0 {
		c.JSON(http.StatusNotFound, result.Error(404, "暂无图片"))
		return
	}

	var images []models.Image
	if err := query.Order("RANDOM()").Limit(limit).Find(&images).Error; err != nil {
		c.JSON(http.StatusInternalServerError, result.Error(500, "获取图片失败"))
		return
	}

	if len(images) == 0 {
		c.JSON(http.StatusNotFound, result.Error(404, "暂无图片"))
		return
	}

	if model == "image" {
		publicURL := images[0].Url
		if images[0].AccessBucketId == 0 {
			publicURL = applyPublicImageURL(setting, images[0].Storage, images[0].BucketId, images[0].Url)
		}
		if strings.HasPrefix(publicURL, "http://") || strings.HasPrefix(publicURL, "https://") {
			c.Redirect(http.StatusFound, publicURL)
			return
		}

		originalPath := c.Request.URL.Path
		originalRawPath := c.Request.URL.RawPath
		imageURL := ensureLeadingSlash(images[0].Url)

		c.Request.URL.Path = imageURL
		c.Request.URL.RawPath = imageURL
		if !ImageProxy(c) {
			c.Request.URL.Path = originalPath
			c.Request.URL.RawPath = originalRawPath
			c.JSON(http.StatusNotFound, result.Error(404, "图片代理失败"))
			return
		}
		return
	}

	var respData []RandomImageResponse
	for _, img := range images {
		fullUrl := buildImageResponseURL(c, setting, img.Storage, img.BucketId, img.Url)
		respData = append(respData, RandomImageResponse{
			Image: img.FileName,
			Url:   fullUrl,
		})
	}

	c.JSON(http.StatusOK, result.Success("ok", respData))
}
func applyRandomGraphTagFilter(query *gorm.DB, db *gorm.DB, tagIds []int) *gorm.DB {
	var validTagIds []int
	hasDefaultTag := false
	for _, id := range tagIds {
		if id == 0 {
			hasDefaultTag = true
		} else {
			validTagIds = append(validTagIds, id)
		}
	}

	var conditions []string
	var args []interface{}

	if len(validTagIds) > 0 {
		subQuery := db.Model(&models.ImageToTags{}).Select("1").
			Where("image_to_tags.image_id = images.id").
			Where("image_to_tags.tag_id IN ?", validTagIds)
		conditions = append(conditions, "EXISTS (?)")
		args = append(args, subQuery)
	}

	if hasDefaultTag {
		noTagSubQuery := db.Model(&models.ImageToTags{}).Select("1").
			Where("image_to_tags.image_id = images.id")
		conditions = append(conditions, "NOT EXISTS (?)")
		args = append(args, noTagSubQuery)
	}

	if len(conditions) == 0 {
		return query
	}

	combinedCondition := strings.Join(conditions, " OR ")
	return query.Where("("+combinedCondition+")", args...)
}

// SetRandomGraph 设置随机图范围
func SetRandomGraph(c *gin.Context) {
	var req models.RandomGraph
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, result.Error(400, "请求参数错误"))
		return
	}

	db := database.GetDB().DB
	var err error
	if req.ID == 0 {
		err = db.Create(&req).Error
	} else {
		err = db.Model(&models.RandomGraph{}).Where("id = ?", req.ID).
			Select("UserIds", "TagIds").
			Updates(models.RandomGraph{
				UserIds: req.UserIds,
				TagIds:  req.TagIds,
			}).Error
		if err == nil {
			db.First(&req, req.ID)
		}
	}

	if err != nil {
		c.JSON(http.StatusInternalServerError, result.Error(500, "设置随机图范围失败"))
		return
	}

	c.JSON(http.StatusOK, result.Success("ok", nil))
}

func GetRandomGraph(c *gin.Context) {
	var randomGraph models.RandomGraph
	db := database.GetDB().DB
	// 获取随机图范围
	err := db.Model(&models.RandomGraph{}).Where("id = ?", 1).Take(&randomGraph).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			randomGraph = models.RandomGraph{UserIds: []int{}, TagIds: []int{}}
		} else if strings.Contains(err.Error(), "cannot unmarshal") {
			randomGraph = models.RandomGraph{ID: 1, UserIds: []int{}, TagIds: []int{}}
		} else {
			c.JSON(http.StatusInternalServerError, result.Error(500, "获取随机图范围失败"))
			return
		}
	}
	type UserSimple struct {
		ID       int    `json:"id"`
		Username string `json:"username"`
	}
	type TagSimple struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	}

	// 获取系统内用户ID
	var users []UserSimple
	if err := db.Model(&models.User{}).Find(&users).Error; err != nil {
		c.JSON(http.StatusInternalServerError, result.Error(500, "获取用户失败"))
		return
	}
	// 获取系统内标签ID
	var tags []TagSimple
	if err := db.Model(&models.Tags{}).Find(&tags).Error; err != nil {
		c.JSON(http.StatusInternalServerError, result.Error(500, "获取标签失败"))
		return
	}

	// 构造返回参数
	req := map[string]any{
		"random_graph": randomGraph,
		"user_ids":     users,
		"tag_ids":      tags,
	}
	c.JSON(http.StatusOK, result.Success("ok", req))
}
