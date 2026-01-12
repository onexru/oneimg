package controllers

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"oneimg/backend/database"
	"oneimg/backend/models"
	"oneimg/backend/utils/buckets"
	"oneimg/backend/utils/ftp"
	"oneimg/backend/utils/result"
	"oneimg/backend/utils/s3"
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

func ImageProxy(c *gin.Context) {
	// 获取并清理路径（修复路径拼接逻辑）
	fullPath := c.Param("path")
	if fullPath == "" || fullPath == "/" {
		c.JSON(http.StatusBadRequest, result.Error(400, "请提供完整的访问路径，如 uploads/2025/11/abc.webp"))
		return
	}
	cleanPath := fmt.Sprintf("/%s", strings.TrimPrefix("uploads"+fullPath, "/"))

	// 解析水印参数
	watermarkCfg := watermark.ParseWatermarkParams(c)

	// 获取数据库实例
	db := database.GetDB()
	if db == nil || db.DB == nil {
		c.JSON(http.StatusInternalServerError, result.Error(500, "数据库连接未初始化"))
		return
	}

	// 查询图片信息
	var imageModel models.Image
	sqlResult := db.DB.Where("Url = ? OR Thumbnail = ?", cleanPath, cleanPath).First(&imageModel)
	if sqlResult.Error != nil {
		c.JSON(http.StatusNotFound, result.Error(404, "图片不存在或已被删除"))
		return
	}

	// 获取配置信息
	setting, setErr := settings.GetSettings()
	if setErr != nil {
		c.JSON(http.StatusInternalServerError, result.Error(500, fmt.Sprintf("获取系统配置失败: %v", setErr)))
		return
	}

	// 检查是否开启来源白名单
	if setting.RefererWhiteEnable && setting.RefererWhiteList != "" {
		// 过滤本站域名
		// 校验Referer白名单
		if !checkReferer(c.Request.Referer(), setting.RefererWhiteList, GetSelfDomain(c)) {
			c.JSON(http.StatusForbidden, result.Error(403, "来源非法"))
			return
		}
	}

	// 校验图片元信息
	if imageModel.Width == 0 && imageModel.Height == 0 {
		log.Printf("图片[%s]元信息不完整（宽高为0），继续代理访问", cleanPath)
		// 不直接返回错误，仅日志警告，避免影响正常访问
	}

	// 获取存储配置
	var bucket models.Buckets
	if err := db.DB.Where("id = ?", imageModel.BucketId).First(&bucket).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusForbidden, result.Error(400, "存储配置不存在"))
			return
		}
		c.JSON(http.StatusForbidden, result.Error(400, "存储配置查询失败"))
		return
	}

	var imageUrl string
	// 判断当前访问的是缩略图还是原图
	if imageModel.Thumbnail == cleanPath {
		imageUrl = imageModel.Thumbnail // 访问的是缩略图，直接用
	} else if imageModel.Url == cleanPath {
		imageUrl = imageModel.Url // 访问的是原图，直接用
	} else {
		// 兜底：优先缩略图，无则用原图（兼容异常场景）
		imageUrl = imageModel.Thumbnail
		if imageUrl == "" {
			imageUrl = imageModel.Url
		}
	}
	// URL为空则返回错误
	if imageUrl == "" {
		c.JSON(http.StatusNotFound, result.Error(404, "图片URL为空，无法访问"))
		return
	}

	// 传递水印配置到各个代理函数
	switch imageModel.Storage {
	case "default":
		proxyLocalFile(c, imageUrl, imageModel.MimeType, watermarkCfg)

	case "webdav":
		proxyWebDAVFile(c, imageUrl, imageModel.MimeType, imageModel.FileSize, bucket, watermarkCfg)

	case "s3", "r2":
		// 初始化S3客户端
		s3Client, err := s3.NewS3Client(setting, bucket)
		if err != nil {
			c.JSON(http.StatusInternalServerError, result.Error(500, fmt.Sprintf("S3/R2客户端初始化失败: %v", err)))
			return
		}
		// 代理S3/R2文件
		proxyS3File(c, imageUrl, imageModel.MimeType, imageModel.FileSize, bucket, s3Client, watermarkCfg)

	case "ftp":
		proxyFTPFile(c, imageUrl, imageModel.MimeType, bucket, watermarkCfg)

	case "telegram":
		ProxyTelegramFile(c, imageUrl, imageModel.FileName, imageModel.MimeType, setting, bucket, watermarkCfg)

	default:
		c.JSON(http.StatusUnprocessableEntity, result.Error(422, fmt.Sprintf("不支持的存储类型: %s", imageModel.Storage)))
	}
}

// proxyS3File S3/R2文件代理（添加水印支持）
func proxyS3File(c *gin.Context, objectKey, mimeType string, fileSize int64, bucket models.Buckets, s3Client *awss3.Client, watermarkCfg watermark.WatermarkConfig) {
	// 清理objectKey（去除开头的/，适配S3路径规则）
	objectKey = strings.TrimPrefix(objectKey, "/")

	// 获取存储配置
	storageConfig := buckets.ConvertToS3Bucket(bucket.Config)

	// 校验bucket和objectKey
	if storageConfig.S3Bucket == "" || objectKey == "" {
		c.JSON(http.StatusInternalServerError, result.Error(500, "S3/R2配置缺失（Bucket或ObjectKey为空）"))
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// 1. 获取S3/R2文件对象
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

		log.Printf("S3/R2获取文件失败 [key:%s, bucket:%s]: %v", objectKey, bucket.Name, err)
		c.JSON(http.StatusBadGateway, result.Error(502, "S3/R2文件获取失败"))
		return
	}
	defer resp.Body.Close()

	// 2. 处理水印
	var contentReader io.Reader = resp.Body
	if watermarkCfg.Enable {
		processedReader, err := watermark.ProcessImageWithWatermark(resp.Body, mimeType, watermarkCfg)
		if err != nil {
			log.Printf("处理S3文件水印失败: %v", err)
			// 失败时重新获取原始流（需要重新请求）
			resp2, _ := s3Client.GetObject(ctx, &getInput)
			if resp2 != nil {
				defer resp2.Body.Close()
				contentReader = resp2.Body
			}
		} else {
			contentReader = processedReader
		}
	}

	// 3. 设置响应头
	c.Header("Content-Type", mimeType)
	// 如果添加了水印，不设置Content-Length（因为内容已改变）
	if !watermarkCfg.Enable {
		// 优先使用S3返回的文件大小，其次使用数据库中存储的大小
		if resp.ContentLength != nil && *resp.ContentLength > 0 {
			c.Header("Content-Length", strconv.FormatInt(*resp.ContentLength, 10))
		} else if fileSize > 0 {
			c.Header("Content-Length", strconv.FormatInt(fileSize, 10))
		}
	} else {
		c.Header("Transfer-Encoding", "chunked")
	}
	// 缓存控制（永久缓存）
	c.Header("Cache-Control", "public, max-age=31536000")
	// 存储类型标识
	c.Header("X-Storage-Type", bucket.Type)
	// 跨域支持（可选）
	c.Header("Access-Control-Allow-Origin", "*")

	// 4. 流式传输文件（避免内存溢出）
	// 设置响应状态码
	c.Status(http.StatusOK)
	// 分块传输，每次4KB
	buf := make([]byte, 4096)
	_, err = io.CopyBuffer(c.Writer, contentReader, buf)
	if err != nil && err != io.EOF {
		log.Printf("S3/R2文件传输失败 [key:%s]: %v", objectKey, err)
	}
}

// proxyWebDAVFile WebDAV文件代理（添加水印支持）
func proxyWebDAVFile(c *gin.Context, relPath, mimeType string, fileSize int64, bucket models.Buckets, watermarkCfg watermark.WatermarkConfig) {
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

	// 处理水印
	var contentReader io.Reader = resp.Body
	if watermarkCfg.Enable {
		processedReader, err := watermark.ProcessImageWithWatermark(resp.Body, mimeType, watermarkCfg)
		if err != nil {
			log.Printf("处理WebDAV文件水印失败: %v", err)
			// 重新获取原始文件
			resp2, _ := client.WebDAVGetFile(ctx, relPath)
			if resp2 != nil {
				defer resp2.Body.Close()
				contentReader = resp2.Body
			}
		} else {
			contentReader = processedReader
		}
	}

	// 设置响应头
	c.Header("Content-Type", mimeType)
	if !watermarkCfg.Enable {
		if resp.ContentLength > 0 {
			c.Header("Content-Length", strconv.FormatInt(resp.ContentLength, 10))
		} else if fileSize > 0 {
			c.Header("Content-Length", strconv.FormatInt(fileSize, 10))
		}
	} else {
		c.Header("Transfer-Encoding", "chunked")
	}
	c.Header("Cache-Control", "public, max-age=31536000")
	c.Header("X-Storage-Type", "webdav")
	c.Header("Access-Control-Allow-Origin", "*")

	// 流式传输文件
	_, err = io.Copy(c.Writer, contentReader)
	if err != nil {
		log.Printf("WebDAV文件传输失败：%v", err)
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

	// 如果启用水印
	if watermarkCfg.Enable {
		file, err := os.Open(fullPath)
		if err != nil {
			c.JSON(http.StatusInternalServerError, result.Error(500, "打开文件失败"))
			return
		}
		defer file.Close()

		// 处理水印
		processedReader, err := watermark.ProcessImageWithWatermark(file, mimeType, watermarkCfg)
		if err != nil {
			log.Printf("处理本地文件水印失败: %v", err)
			// 失败时返回原始文件
			c.File(fullPath)
			return
		}

		// 设置响应头
		c.Header("Content-Type", mimeType)
		c.Header("Cache-Control", "public, max-age=31536000")
		c.Header("X-Storage-Type", "default")
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Transfer-Encoding", "chunked")

		// 传输处理后的图片
		c.Status(http.StatusOK)
		io.Copy(c.Writer, processedReader)
		return
	}

	// 未启用水印，使用原始逻辑
	// 设置响应头
	c.Header("Content-Type", mimeType)
	c.Header("Content-Length", strconv.FormatInt(fileInfo.Size(), 10))
	c.Header("Cache-Control", "public, max-age=31536000")
	c.Header("X-Storage-Type", "default")
	c.Header("Access-Control-Allow-Origin", "*")

	// 流式传输
	c.File(fullPath)
}

// FTP代理（添加水印支持）
func proxyFTPFile(c *gin.Context, ftpPath string, mimeType string, bucket models.Buckets, watermarkCfg watermark.WatermarkConfig) {
	// 如果启用水印，不强制删除Content-Length
	if !watermarkCfg.Enable {
		c.Header("Transfer-Encoding", "chunked")
		// 强制删除Content-Length
		c.Writer.Header().Del("Content-Length")
	}

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

	// 处理水印
	var contentReader io.Reader = fileReader
	if watermarkCfg.Enable {
		processedReader, err := watermark.ProcessImageWithWatermark(fileReader, mimeType, watermarkCfg)
		if err != nil {
			log.Printf("处理FTP文件水印失败: %v", err)
			// 重新获取原始文件流
			fileReader2, _, _ := ftpUtil.GetFileStreamReader(ftpPath)
			if fileReader2 != nil {
				defer fileReader2.Close()
				contentReader = fileReader2
			}
		} else {
			contentReader = processedReader
		}
	}

	c.Header("Content-Type", mimeType)
	c.Header("Cache-Control", "public, max-age=31536000")
	c.Header("X-Storage-Type", "ftp")
	c.Header("Access-Control-Allow-Origin", "*")
	c.Header("Connection", "close")

	if watermarkCfg.Enable {
		c.Header("Transfer-Encoding", "chunked")
	}

	c.Status(http.StatusOK)

	buf := make([]byte, 4096)
	totalWritten := int64(0)
	for {
		n, err := contentReader.Read(buf)
		if n > 0 {
			if _, writeErr := c.Writer.Write(buf[:n]); writeErr != nil {
				break
			}
			c.Writer.Flush()
			totalWritten += int64(n)
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			c.Writer.WriteHeader(http.StatusInternalServerError)
			break
		}
	}
	c.Writer.Flush()
	c.Abort()
}

// Telegram 代理（添加水印支持）
func ProxyTelegramFile(c *gin.Context, realPath string, telegramFileName string, mimeType string, cfg models.Settings, bucket models.Buckets, watermarkCfg watermark.WatermarkConfig) {
	// 1. 统一响应头
	if !watermarkCfg.Enable {
		c.Header("Transfer-Encoding", "chunked")
		c.Writer.Header().Del("Content-Length")
	}
	c.Header("Content-Type", mimeType)
	c.Header("Cache-Control", "public, max-age=31536000")
	c.Header("X-Storage-Type", "telegram")
	c.Header("Access-Control-Allow-Origin", "*")
	c.Header("Connection", "close")

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

	if telegramModel.TGFileId == "" {
		c.AbortWithStatusJSON(http.StatusBadGateway, result.Error(502, "telegram文件无有效file id"))
		return
	}

	// 3. 调用telegram包解析FileId
	// 检查是否为缩略图（链接格式：/uploads/Y/d/thumbnails/xxxxx.webp）
	var fileId string
	if strings.Contains(realPath, "/thumbnails/") && strings.HasSuffix(realPath, ".webp") {
		fileId = telegram.ParseFileIdFromTelegramPath(telegramModel.TGThumbnailFileId)
	} else {
		fileId = telegram.ParseFileIdFromTelegramPath(telegramModel.TGFileId)
	}

	if fileId == "" {
		log.Printf("无效的Telegram路径：%s", telegramModel.TGFileId)
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

	// 6. 处理水印
	var contentReader io.Reader = fileReader
	if watermarkCfg.Enable {
		processedReader, err := watermark.ProcessImageWithWatermark(fileReader, mimeType, watermarkCfg)
		if err != nil {
			log.Printf("处理Telegram文件水印失败: %v", err)
			// 重新获取原始文件流
			fileReader2, _ := telegram.GetTelegramFileStreamReader(tgClient, fileId)
			if fileReader2 != nil {
				defer fileReader2.Close()
				contentReader = fileReader2
			}
		} else {
			contentReader = processedReader
		}
	}

	// 7. 流式返回
	c.Status(http.StatusOK)
	buf := make([]byte, 4096)
	totalWritten := int64(0)
	for {
		n, err := contentReader.Read(buf)
		if n > 0 {
			if _, writeErr := c.Writer.Write(buf[:n]); writeErr != nil {
				log.Printf("响应写入失败：%v", writeErr)
				break
			}
			c.Writer.Flush()
			totalWritten += int64(n)
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Printf("Telegram文件流读取失败：%v", err)
			c.Writer.WriteHeader(http.StatusInternalServerError)
			break
		}
	}
	c.Writer.Flush()
	c.Abort()
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
