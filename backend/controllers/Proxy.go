package controllers

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"oneimg/backend/database"
	"oneimg/backend/models"
	"oneimg/backend/utils/buckets"
	"oneimg/backend/utils/ftp"
	"oneimg/backend/utils/result"
	"oneimg/backend/utils/s3"
	"oneimg/backend/utils/securestorage"
	"oneimg/backend/utils/settings"
	"oneimg/backend/utils/telegram"
	"oneimg/backend/utils/watermark"
	"oneimg/backend/utils/webdav"

	"github.com/aws/aws-sdk-go-v2/aws"
	awss3 "github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	smithyhttp "github.com/aws/smithy-go/transport/http"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type resolvedImageAccess struct {
	bucket      models.Buckets
	replica     *models.ImageStorage
	storageType string
	path        string
}

func resolveImageAccess(db *gorm.DB, image models.Image, thumbnail bool) (resolvedImageAccess, error) {
	canonicalPath := image.Url
	if thumbnail {
		canonicalPath = image.Thumbnail
	}

	// An explicitly selected source must have both an enabled bucket and a
	// successful replica. If it cannot currently serve the requested object,
	// transparently fall back to the durable local copy.
	if image.AccessBucketId > 0 {
		if resolved, ok := resolveImageReplicaAccess(db, image.Id, image.AccessBucketId, thumbnail); ok {
			return resolved, nil
		}
		if resolved, ok := resolveLocalImageAccess(db, image.Id, thumbnail); ok {
			return resolved, nil
		}
	} else if image.Storage != "default" {
		// Zero means the default access policy. Prefer a successful local
		// replica whenever one exists, including migrated legacy images whose
		// canonical record still points at a remote bucket.
		if resolved, ok := resolveLocalImageAccess(db, image.Id, thumbnail); ok {
			return resolved, nil
		}
	}

	var canonicalBucket models.Buckets
	canonicalErr := db.First(&canonicalBucket, image.BucketId).Error
	if canonicalErr == nil && !canonicalBucket.Disabled && canonicalPath != "" {
		storageType := image.Storage
		if storageType == "" {
			storageType = canonicalBucket.Type
		}
		resolved := resolvedImageAccess{
			bucket:      canonicalBucket,
			storageType: storageType,
			path:        canonicalPath,
		}
		var replica models.ImageStorage
		if err := db.Where(
			"image_id = ? AND bucket_id = ? AND status = ?",
			image.Id, canonicalBucket.Id, models.ImageStorageStatusSuccess,
		).First(&replica).Error; err == nil {
			resolved.replica = &replica
		}
		return resolved, nil
	}

	if resolved, ok := resolveLocalImageAccess(db, image.Id, thumbnail); ok {
		return resolved, nil
	}
	if canonicalErr != nil && !errors.Is(canonicalErr, gorm.ErrRecordNotFound) {
		return resolvedImageAccess{}, canonicalErr
	}
	return resolvedImageAccess{}, errors.New("没有可用的图片存储源")
}

func resolveImageReplicaAccess(db *gorm.DB, imageID, bucketID int, thumbnail bool) (resolvedImageAccess, bool) {
	var replica models.ImageStorage
	if err := db.Where(
		"image_id = ? AND bucket_id = ? AND status = ?",
		imageID, bucketID, models.ImageStorageStatusSuccess,
	).First(&replica).Error; err != nil {
		return resolvedImageAccess{}, false
	}

	var bucket models.Buckets
	if err := db.Where("id = ? AND disabled = ?", bucketID, false).First(&bucket).Error; err != nil {
		return resolvedImageAccess{}, false
	}
	path := replica.URL
	if thumbnail {
		path = replica.Thumbnail
	}
	if path == "" {
		return resolvedImageAccess{}, false
	}
	storageType := replica.Storage
	if storageType == "" {
		storageType = bucket.Type
	}
	return resolvedImageAccess{bucket: bucket, replica: &replica, storageType: storageType, path: path}, true
}

func resolveLocalImageAccess(db *gorm.DB, imageID int, thumbnail bool) (resolvedImageAccess, bool) {
	var replica models.ImageStorage
	if err := db.Model(&models.ImageStorage{}).
		Select("image_storages.*").
		Joins("JOIN buckets ON buckets.id = image_storages.bucket_id").
		Where(
			"image_storages.image_id = ? AND image_storages.status = ? AND buckets.type = ? AND buckets.disabled = ?",
			imageID, models.ImageStorageStatusSuccess, "default", false,
		).
		Order("buckets.id ASC").
		First(&replica).Error; err != nil {
		return resolvedImageAccess{}, false
	}

	var bucket models.Buckets
	if err := db.First(&bucket, replica.BucketID).Error; err != nil {
		return resolvedImageAccess{}, false
	}
	path := replica.URL
	if thumbnail {
		path = replica.Thumbnail
	}
	if path == "" {
		return resolvedImageAccess{}, false
	}
	return resolvedImageAccess{bucket: bucket, replica: &replica, storageType: "default", path: path}, true
}

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

	// 获取配置信息
	setting, setErr := settings.GetSettings()
	if setErr != nil {
		c.JSON(http.StatusInternalServerError, result.Error(500, fmt.Sprintf("获取系统配置失败: %v", setErr)))
		return true
	}

	// 检查是否开启来源白名单
	if setting.RefererWhiteEnable && setting.RefererWhiteList != "" {
		// 校验Referer白名单
		if !checkReferer(c.Request.Referer(), setting.RefererWhiteList, GetSelfDomain(c)) {
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

	// 传递水印配置到各个代理函数
	switch access.storageType {
	case "default":
		proxyLocalFile(c, imageUrl, imageModel.MimeType, watermarkCfg)

	case "webdav":
		proxyWebDAVFile(c, imageUrl, imageModel.MimeType, bucket, watermarkCfg)
	case "r2":
		// 初始化S3客户端
		s3Client, err := s3.NewS3Client(setting, bucket)
		if err != nil {
			c.JSON(http.StatusInternalServerError, result.Error(500, fmt.Sprintf("R2客户端初始化失败: %v", err)))
			return true
		}
		proxyR2File(c, imageUrl, imageModel.MimeType, bucket, s3Client, watermarkCfg)

	case "s3":
		// 初始化S3客户端
		s3Client, err := s3.NewS3Client(setting, bucket)
		if err != nil {
			c.JSON(http.StatusInternalServerError, result.Error(500, fmt.Sprintf("S3客户端初始化失败: %v", err)))
			return true
		}
		// 代理S3/R2文件
		proxyS3File(c, imageUrl, imageModel.MimeType, bucket, s3Client, watermarkCfg)

	case "ftp":
		proxyFTPFile(c, imageUrl, imageModel.MimeType, bucket, watermarkCfg)

	case "telegram":
		ProxyTelegramFile(c, imageUrl, imageModel.FileName, imageModel.MimeType, bucket, access.replica, watermarkCfg)

	default:
		c.JSON(http.StatusUnprocessableEntity, result.Error(422, fmt.Sprintf("不支持的存储类型: %s", access.storageType)))
	}

	return true
}

// serveStoredImage is the single plaintext boundary for every storage
// backend. Storage objects may be legacy plaintext or versioned ciphertext;
// browsers always receive the decoded image bytes.
func serveStoredImage(c *gin.Context, stored io.Reader, mimeType, storageType string, watermarkCfg watermark.WatermarkConfig) error {
	content, _, err := securestorage.ReadAll(stored)
	if err != nil {
		return err
	}

	c.Header("Content-Type", mimeType)
	c.Header("Cache-Control", "public, max-age=31536000")
	c.Header("X-Storage-Type", storageType)
	c.Header("Access-Control-Allow-Origin", "*")

	if watermarkCfg.Enable {
		processedReader, watermarkErr := watermark.ProcessImageWithWatermark(bytes.NewReader(content), mimeType, watermarkCfg)
		if watermarkErr == nil {
			c.Writer.Header().Del("Content-Length")
			c.Header("Transfer-Encoding", "chunked")
			c.Status(http.StatusOK)
			_, err = io.Copy(c.Writer, processedReader)
			return err
		}
		log.Printf("处理%s文件水印失败，返回解密后的原图: %v", storageType, watermarkErr)
	}

	// ServeContent keeps HEAD and Range requests working even though encrypted
	// objects have to be authenticated before any plaintext can be returned.
	c.Writer.Header().Del("Transfer-Encoding")
	http.ServeContent(
		c.Writer,
		c.Request,
		filepath.Base(c.Request.URL.Path),
		time.Time{},
		bytes.NewReader(content),
	)
	return nil
}

// proxyR2File R2文件代理
func proxyR2File(c *gin.Context, objectKey, mimeType string, bucket models.Buckets, s3Client *awss3.Client, watermarkCfg watermark.WatermarkConfig) {
	// 清理objectKey（去除开头的/，适配S3路径规则）
	objectKey = strings.TrimPrefix(objectKey, "/")

	// 获取存储配置
	storageConfig := buckets.ConvertToR2Bucket(bucket.Config)

	// 校验bucket和objectKey
	if storageConfig.R2Bucket == "" || objectKey == "" {
		c.JSON(http.StatusInternalServerError, result.Error(500, "R2配置缺失（Bucket或ObjectKey为空）"))
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// 1. 获取R2文件对象
	getInput := awss3.GetObjectInput{
		Bucket: aws.String(storageConfig.R2Bucket),
		Key:    aws.String(objectKey),
	}

	resp, err := s3Client.GetObject(ctx, &getInput)
	if err != nil {
		// 区分不同错误类型
		var noSuchKeyErr *types.NoSuchKey
		if errors.As(err, &noSuchKeyErr) {
			c.JSON(http.StatusNotFound, result.Error(404, "R2文件不存在"))
			return
		}

		var respErr *smithyhttp.ResponseError
		if errors.As(err, &respErr) {
			statusCode := respErr.HTTPStatusCode()
			switch statusCode {
			case http.StatusForbidden:
				c.JSON(http.StatusForbidden, result.Error(403, "R2文件访问权限不足"))
				return
			case http.StatusRequestTimeout:
				c.JSON(http.StatusGatewayTimeout, result.Error(504, "R2请求超时"))
				return
			}
		}

		log.Printf("R2获取文件失败 [key:%s, bucket:%s]: %v", objectKey, bucket.Name, err)
		c.JSON(http.StatusBadGateway, result.Error(502, "R2文件获取失败"))
		return
	}
	defer resp.Body.Close()

	if err := serveStoredImage(c, resp.Body, mimeType, bucket.Type, watermarkCfg); err != nil {
		log.Printf("R2文件解密或传输失败 [key:%s]: %v", objectKey, err)
		if !c.Writer.Written() {
			c.JSON(http.StatusInternalServerError, result.Error(500, "R2文件解密失败"))
		}
	}
}

// proxyS3File S3文件代理（添加水印支持）
func proxyS3File(c *gin.Context, objectKey, mimeType string, bucket models.Buckets, s3Client *awss3.Client, watermarkCfg watermark.WatermarkConfig) {
	// 清理objectKey（去除开头的/，适配S3路径规则）
	objectKey = strings.TrimPrefix(objectKey, "/")

	// 获取存储配置
	storageConfig := buckets.ConvertToS3Bucket(bucket.Config)

	// 校验bucket和objectKey
	if storageConfig.S3Bucket == "" || objectKey == "" {
		c.JSON(http.StatusInternalServerError, result.Error(500, "S3配置缺失（Bucket或ObjectKey为空）"))
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// 1. 获取S3文件对象
	getInput := awss3.GetObjectInput{
		Bucket: aws.String(storageConfig.S3Bucket),
		Key:    aws.String(objectKey),
	}

	resp, err := s3Client.GetObject(ctx, &getInput)
	if err != nil {
		// 区分不同错误类型
		var noSuchKeyErr *types.NoSuchKey
		if errors.As(err, &noSuchKeyErr) {
			c.JSON(http.StatusNotFound, result.Error(404, "S3文件不存在"))
			return
		}

		var respErr *smithyhttp.ResponseError
		if errors.As(err, &respErr) {
			statusCode := respErr.HTTPStatusCode()
			switch statusCode {
			case http.StatusForbidden:
				c.JSON(http.StatusForbidden, result.Error(403, "S3文件访问权限不足"))
				return
			case http.StatusRequestTimeout:
				c.JSON(http.StatusGatewayTimeout, result.Error(504, "S3请求超时"))
				return
			}
		}

		log.Printf("S3获取文件失败 [key:%s, bucket:%s]: %v", objectKey, bucket.Name, err)
		c.JSON(http.StatusBadGateway, result.Error(502, "S3文件获取失败"))
		return
	}
	defer resp.Body.Close()

	if err := serveStoredImage(c, resp.Body, mimeType, bucket.Type, watermarkCfg); err != nil {
		log.Printf("S3文件解密或传输失败 [key:%s]: %v", objectKey, err)
		if !c.Writer.Written() {
			c.JSON(http.StatusInternalServerError, result.Error(500, "S3文件解密失败"))
		}
	}
}

// proxyWebDAVFile WebDAV文件代理（添加水印支持）
func proxyWebDAVFile(c *gin.Context, relPath, mimeType string, bucket models.Buckets, watermarkCfg watermark.WatermarkConfig) {
	// 获取存储配置
	storageConfig := buckets.ConvertToWebDavBucket(bucket.Config)

	// 初始化WebDav客户端
	if storageConfig.WebdavURL == "" {
		c.JSON(http.StatusInternalServerError, result.Error(500, "WebDAV配置未设置（WebdavURL为空）"))
		return
	}
	client := webdav.Client(webdav.Config{
		BaseURL:  storageConfig.WebdavURL,
		Username: storageConfig.WebdavUser,
		Password: storageConfig.WebdavPass,
		Timeout:  30 * time.Second,
	})
	// 验证WebDAV连接（非阻塞，仅日志）
	go func() {
		ctx := context.Background()
		if _, err := client.WebDAVStat(ctx, ""); err != nil {
			log.Printf("WebDAV连接验证失败: %v", err)
		}
	}()

	ctx := context.Background()

	// 验证文件存在
	exists, err := client.WebDAVStat(ctx, relPath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, result.Error(500, "WebDAV文件状态验证失败"))
		return
	}
	if !exists {
		c.JSON(http.StatusNotFound, result.Error(404, "WebDAV文件不存在"))
		return
	}

	// 获取文件流
	resp, err := client.WebDAVGetFile(ctx, relPath)
	if err != nil {
		c.JSON(http.StatusBadGateway, result.Error(502, "WebDAV文件获取失败"))
		return
	}
	defer resp.Body.Close()

	// 校验响应状态
	if resp.StatusCode != http.StatusOK {
		c.JSON(resp.StatusCode, result.Error(resp.StatusCode, "WebDAV文件获取失败"))
		return
	}

	if err := serveStoredImage(c, resp.Body, mimeType, bucket.Type, watermarkCfg); err != nil {
		log.Printf("WebDAV文件解密或传输失败：%v", err)
		if !c.Writer.Written() {
			c.JSON(http.StatusInternalServerError, result.Error(500, "WebDAV文件解密失败"))
		}
	}
}

// proxyLocalFile 本地文件代理（添加水印支持）
func proxyLocalFile(c *gin.Context, realPath string, mimeType string, watermarkCfg watermark.WatermarkConfig) {
	fullPath := filepath.Join(filepath.Clean(realPath))
	// 去除第一个/和\
	fullPath = strings.TrimPrefix(fullPath, "/")
	fullPath = strings.TrimPrefix(fullPath, "\\")

	fileInfo, err := os.Stat(fullPath)
	if os.IsNotExist(err) {
		c.JSON(http.StatusNotFound, result.Error(404, "文件不存在"))
		return
	} else if err != nil {
		c.JSON(http.StatusInternalServerError, result.Error(500, "文件状态验证失败"))
		return
	}

	if fileInfo.IsDir() {
		c.JSON(http.StatusForbidden, result.Error(403, "文件不可访问"))
		return
	}

	file, err := os.Open(fullPath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, result.Error(500, "打开文件失败"))
		return
	}
	defer file.Close()
	if err := serveStoredImage(c, file, mimeType, "default", watermarkCfg); err != nil {
		log.Printf("本地文件解密或传输失败 [%s]: %v", fullPath, err)
		if !c.Writer.Written() {
			c.JSON(http.StatusInternalServerError, result.Error(500, "本地文件解密失败"))
		}
	}
}

// FTP代理（添加水印支持）
func proxyFTPFile(c *gin.Context, ftpPath string, mimeType string, bucket models.Buckets, watermarkCfg watermark.WatermarkConfig) {
	// 清理FTP路径
	ftpPath = cleanFTPPath(ftpPath)

	// 获取存储配置
	storageConfig := buckets.ConvertToFTPBucket(bucket.Config)

	ftpUtil := ftp.NewFTPUtil(ftp.FTPConfig{
		Host:     storageConfig.FTPHost,
		Port:     storageConfig.FTPPort,
		User:     storageConfig.FTPUser,
		Password: storageConfig.FTPPass,
		Timeout:  60,
	})
	defer func() {
		if err := ftpUtil.Close(); err != nil {
			if !strings.Contains(err.Error(), "227 Entering Passive Mode") {
				log.Printf("FTP连接关闭失败：%v", err)
			}
		}
	}()

	// 获取文件流
	fileReader, _, err := ftpUtil.GetFileStreamReader(ftpPath)
	if err != nil {
		log.Printf("获取FTP文件流失败（路径：%s）：%v", ftpPath, err)
		if strings.Contains(err.Error(), "550") {
			c.AbortWithStatusJSON(http.StatusBadGateway, result.Error(502, "文件不存在或PureFTPd权限不足"))
		} else {
			c.AbortWithStatusJSON(http.StatusBadGateway, result.Error(502, "FTP文件获取失败："+err.Error()))
		}
		return
	}
	defer func() {
		if err := fileReader.Close(); err != nil {
			if !strings.Contains(err.Error(), "227 Entering Passive Mode") {
				log.Printf("FTP文件流关闭失败：%v", err)
			}
		}
	}()

	if err := serveStoredImage(c, fileReader, mimeType, bucket.Type, watermarkCfg); err != nil {
		log.Printf("FTP文件解密或传输失败（路径：%s）：%v", ftpPath, err)
		if !c.Writer.Written() {
			c.JSON(http.StatusInternalServerError, result.Error(500, "FTP文件解密失败"))
		}
	}
}

// Telegram 代理（添加水印支持）
func ProxyTelegramFile(c *gin.Context, realPath string, telegramFileName string, mimeType string, bucket models.Buckets, replica *models.ImageStorage, watermarkCfg watermark.WatermarkConfig) {
	// 获取存储配置

	storageConfig := buckets.ConvertToTelegramBucket(bucket.Config)

	// 校验Telegram配置，弃用
	// if storageConfig.TGBotToken == "" {
	// 	log.Printf("Telegram BotToken 为空")
	// 	c.AbortWithStatusJSON(http.StatusBadGateway, result.Error(502, "telegram配置异常：bot token为空"))
	// 	return
	// }

	// 获取数据库
	db := database.GetDB()
	if db == nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, result.Error(500, "获取数据库连接失败"))
		return
	}
	mainFileID := imageStorageMetadataString(replica, "tg_file_id")
	thumbnailFileID := imageStorageMetadataString(replica, "tg_thumbnail_file_id")
	if mainFileID == "" || (strings.Contains(realPath, "/thumbnails/") && thumbnailFileID == "") {
		var telegramModel models.ImageTeleGram
		if err := db.DB.Where("file_name = ?", telegramFileName).First(&telegramModel).Error; err != nil {
			if strings.Contains(err.Error(), "record not found") {
				c.AbortWithStatusJSON(http.StatusBadGateway, result.Error(502, "telegram文件不存在或file id无效"))
			} else {
				log.Printf("查询telegram文件信息失败：%v", err)
				c.AbortWithStatusJSON(http.StatusInternalServerError, result.Error(500, "查询telegram文件信息失败"))
			}
			return
		}
		if mainFileID == "" {
			mainFileID = telegramModel.TGFileId
		}
		if thumbnailFileID == "" {
			thumbnailFileID = telegramModel.TGThumbnailFileId
		}
	}

	if mainFileID == "" {
		c.AbortWithStatusJSON(http.StatusBadGateway, result.Error(502, "telegram文件无有效file id"))
		return
	}

	// 3. 调用telegram包解析FileId
	// 检查是否为缩略图（链接格式：/uploads/Y/d/thumbnails/xxxxx.webp）
	var fileId string
	if strings.Contains(realPath, "/thumbnails/") {
		fileId = telegram.ParseFileIdFromTelegramPath(thumbnailFileID)
	} else {
		fileId = telegram.ParseFileIdFromTelegramPath(mainFileID)
	}

	if fileId == "" {
		log.Printf("无效的Telegram路径：%s", realPath)
		c.AbortWithStatusJSON(http.StatusBadGateway, result.Error(502, "无效的telegram文件路径"))
		return
	}

	// 4. 初始化Telegram客户端
	tgClient := telegram.NewClient(storageConfig.TGBotToken)
	tgClient.Timeout = 60 * time.Second // 延长超时
	tgClient.Retry = 3                  // 重试次数

	// 5. 调用telegram包获取文件流
	fileReader, err := telegram.GetTelegramFileStreamReader(tgClient, fileId)
	if err != nil {
		log.Printf("获取Telegram文件流失败（FileId：%s）：%v", fileId, err)
		if strings.Contains(err.Error(), "file not found") || strings.Contains(err.Error(), "invalid file id") {
			c.AbortWithStatusJSON(http.StatusBadGateway, result.Error(502, "telegram文件不存在或file id无效"))
		} else {
			c.AbortWithStatusJSON(http.StatusBadGateway, result.Error(502, "telegram文件获取失败："+err.Error()))
		}
		return
	}
	defer func() {
		if err := fileReader.Close(); err != nil {
			log.Printf("Telegram文件流关闭失败：%v", err)
		}
	}()

	if err := serveStoredImage(c, fileReader, mimeType, bucket.Type, watermarkCfg); err != nil {
		log.Printf("Telegram文件解密或传输失败（FileId：%s）：%v", fileId, err)
		if !c.Writer.Written() {
			c.JSON(http.StatusInternalServerError, result.Error(500, "Telegram文件解密失败"))
		}
	}
}

func imageStorageMetadataString(replica *models.ImageStorage, key string) string {
	if replica == nil || replica.Metadata == nil {
		return ""
	}
	value, ok := replica.Metadata[key]
	if !ok || value == nil {
		return ""
	}
	if text, ok := value.(string); ok {
		return strings.TrimSpace(text)
	}
	return strings.TrimSpace(fmt.Sprint(value))
}

// 辅助函数
func cleanFTPPath(path string) string {
	path = strings.ReplaceAll(path, "\\", "/")
	path = strings.TrimPrefix(path, "/")
	path = strings.ReplaceAll(path, "//", "/")
	path = strings.TrimSuffix(path, "/")
	return path
}

// 辅助函数，校验来源
func checkReferer(referer string, whiteList string, selfDomain string) bool {
	if referer == "" {
		return true
	}

	refererDomain, err := extractDomainFromReferer(referer)
	if err != nil {
		return false
	}

	selfDomain = strings.TrimSpace(strings.ToLower(selfDomain))
	if selfDomain != "" {
		if refererDomain == selfDomain || strings.HasSuffix(refererDomain, "."+selfDomain) {
			return true
		}
	}

	whiteListDomains := strings.Split(strings.TrimSpace(whiteList), ",")

	domainSet := make(map[string]bool)
	for _, d := range whiteListDomains {
		domain := strings.TrimSpace(d)
		if domain != "" {
			domainSet[domain] = true
		}
	}

	for allowedDomain := range domainSet {
		if refererDomain == allowedDomain {
			return true
		}
		if strings.HasSuffix(refererDomain, "."+allowedDomain) {
			return true
		}
	}

	return false
}

func extractDomainFromReferer(referer string) (string, error) {
	if !strings.HasPrefix(referer, "http") {
		referer = "http://" + referer
	}

	// 解析URL
	parsedURL, err := url.Parse(referer)
	if err != nil {
		return "", err
	}

	host := parsedURL.Hostname()

	return strings.ToLower(host), nil
}

// 辅助函数，获取本站域名
func GetSelfDomain(c *gin.Context) string {
	host := c.GetHeader("X-Forwarded-Host")
	if host == "" {
		host = c.Request.Host
	}
	domain := strings.Split(host, ":")[0]
	return strings.ToLower(domain)
}
