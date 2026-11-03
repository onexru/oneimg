package images

import (
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

	"github.com/gin-gonic/gin"
)

func ImageProxy(c *gin.Context) bool {
	// 获取并清理路径
	cleanPath := c.Request.URL.Path
	if cleanPath == "" || cleanPath == "/" {
		// 根路径不应由图片代理处理，由 NoRoute 后续逻辑处理
		return false
	}

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
		// 不透出底层错误：%v 会把表名/驱动细节带到公开的图片代理响应里。
		c.JSON(http.StatusInternalServerError, result.Error(500, "获取系统配置失败"))
		log.Printf("图片代理：读取系统配置失败：%v", setErr)
		return true
	}

	// 检查是否开启来源白名单
	if setting.RefererWhiteEnable {
		// 防盗链会按 Referer 决定放行与否，必须让缓存按 Referer 分桶：
		// 否则 CDN 可能把「合法来源的 200 图片」缓存后返回给非法来源，
		// 防盗链形同虚设；反之也可能把 403 缓存给合法访客。
		c.Header("Vary", "Referer")
		// 校验Referer白名单
		if !checkRefererPolicy(c.Request.Referer(), setting.RefererWhiteList, GetSelfDomain(c), setting.RefererAllowEmpty) {
			c.JSON(http.StatusForbidden, result.Error(403, "来源非法"))
			return true
		}
	}

	// 水印配置：**全局开关是唯一的总闸**，query 只能在开关打开后调整样式参数。
	//
	// 原实现无条件 ParseWatermarkParams(c)，只读 query 不看 setting.WatermarkEnable ——
	// 管理员把水印关掉也拦不住任何人用 ?watermark=true 自己打开，
	// 等于给公网留了一个「免鉴权、用户可控的解码器 + 编码器」：
	// 单次请求就是一次 io.ReadAll(≤32MB) + image.Decode(≤16MP) + 重编码，
	// 而这张图还会先 driver.Get 回源一次。配合转码槽只有 2 个、排队不可取消，
	// 就是稳定的放大 DoS（见 M1 攻击手册）。
	//
	// 关闭时传零值 WatermarkConfig（Enable=false），后续所有转码路径自然短路。
	var watermarkCfg watermark.WatermarkConfig
	if setting.WatermarkEnable {
		watermarkCfg = watermark.ParseWatermarkParams(c)
	}

	// 转码路径单独限流（按 IP）。
	//
	// /uploads/* 走的是 NoRoute，不经过 AuthLimiter（那个只覆盖
	// /api/login|register|auth/|verify/），所以此前这条路径**完全零限流**。
	// 这里只对「真正会触发转码」的请求收费：普通看图不受影响，
	// 而转码请求被压到每 IP 一个很小的速率，配合槽位超时，
	// 单个 IP 再也无法堆出「停手后服务器还在烧 CPU」的长队。
	if watermarkCfg.Enable {
		if !allowWatermarkTranscode(c.ClientIP()) {
			c.Header("Retry-After", "1")
			c.JSON(http.StatusTooManyRequests, result.Error(http.StatusTooManyRequests, "请求过于频繁，请稍后重试"))
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
