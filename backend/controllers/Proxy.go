package controllers

import (
	"fmt"
	"github.com/gin-gonic/gin"
	"log"
	"net/http"
	"oneimg/backend/database"
	"oneimg/backend/models"
	"oneimg/backend/storage"
	"oneimg/backend/utils/buckets"
	"oneimg/backend/utils/result"
	"oneimg/backend/utils/s3"
	"oneimg/backend/utils/settings"
	"oneimg/backend/utils/watermark"
)

func ImageProxy(c *gin.Context) bool {
	// 获取并清理路径
	cleanPath := c.Request.URL.Path
	if cleanPath == "" || cleanPath == "/" {
		// 根路径不应由图片代理处理，由 NoRoute 后续逻辑处理
		return false
	}

	// 解析水印参数
	watermarkCfg := watermark.ParseWatermarkParams(c)

	// 获取数据库实例
	db := database.GetDB()
	if db == nil || db.DB == nil {
		return false
	}

	// 查询图片信息
	var imageModel models.Image
	sqlResult := db.DB.Where("Url = ? OR Thumbnail = ?", cleanPath, cleanPath).First(&imageModel)
	if sqlResult.Error != nil {
		// 图片不存在，直接返回，交给 NoRoute 后续逻辑处理（如渲染 SPA）
		return false
	}

	if imageModel.Deleting {
		c.AbortWithStatus(http.StatusGone)
		return true
	}
	// 获取配置信息
	setting, setErr := settings.GetSettings()
	if setErr != nil {
		c.JSON(http.StatusInternalServerError, result.Error(500, fmt.Sprintf("获取系统配置失败: %v", setErr)))
		return true
	}

	// 检查是否开启来源白名单
	if setting.RefererWhiteEnable {
		// 校验Referer白名单
		if !checkRefererPolicy(c.Request.Referer(), setting.RefererWhiteList, GetSelfDomain(c), setting.RefererAllowEmpty) {
			c.JSON(http.StatusForbidden, result.Error(403, "来源非法"))
			return true
		}
	}

	// 校验图片元信息
	if imageModel.Width == 0 && imageModel.Height == 0 {
		log.Printf("图片[%s]元信息不完整（宽高为0），继续代理访问", cleanPath)
	}

	// 判断当前访问的是缩略图还是原图
	access, err := resolveImageAccess(db.DB, imageModel, imageModel.Thumbnail == cleanPath)
	if err != nil {
		log.Printf("图片[%s]没有可用的访问存储源: %v", cleanPath, err)
		c.JSON(http.StatusServiceUnavailable, result.Error(503, "图片存储源暂不可用"))
		return true
	}
	bucket := access.bucket
	imageUrl := access.path

	// R2 signing remains a separate policy-aware capability. Raw proxied bytes
	// (encrypted objects/watermark/historical SVG) use the same driver as uploads.
	var driver storage.Driver
	if access.storageType == "r2" {
		client, err := s3.NewS3Client(setting, bucket)
		if err != nil {
			c.JSON(http.StatusInternalServerError, result.Error(500, "R2客户端初始化失败"))
			return true
		}
		cfg := buckets.ConvertToR2Bucket(bucket.Config)
		if !isHistoricalSVG(imageModel, imageUrl) && redirectR2Object(c, imageUrl, cfg.R2Bucket, bucket, client, watermarkCfg) {
			return true
		}
		driver, err = storage.NewS3(client, cfg.R2Bucket)
		if err != nil {
			c.JSON(http.StatusInternalServerError, result.Error(500, "存储配置缺失"))
			return true
		}
	} else {
		driver, err = storage.New(setting, bucket)
		if err != nil {
			c.JSON(http.StatusInternalServerError, result.Error(500, "存储客户端初始化失败"))
			return true
		}
	}
	var metadata map[string]any
	if access.replica != nil {
		metadata = access.replica.Metadata
	}
	if access.storageType == "telegram" {
		metadata, err = storage.ResolveTelegramMetadata(db.DB, imageModel.FileName, metadata)
		if err != nil {
			c.JSON(http.StatusInternalServerError, result.Error(500, "查询文件元数据失败"))
			return true
		}
	}
	proxyStorageFile(c, driver, storage.ArtifactRef(imageUrl, metadata, imageModel.Thumbnail == cleanPath), imageModel.MimeType, access.storageType, watermarkCfg)
	return true
}
