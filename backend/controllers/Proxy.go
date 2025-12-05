package controllers

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"oneimg/backend/database"
	"oneimg/backend/models"
	"oneimg/backend/utils/result"
	"oneimg/backend/utils/s3"
	"oneimg/backend/utils/settings"
	"oneimg/backend/utils/webdav"

	"github.com/aws/aws-sdk-go-v2/aws"
	awss3 "github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	smithyhttp "github.com/aws/smithy-go/transport/http"
	"github.com/gin-gonic/gin"
)

func ImageProxy(c *gin.Context) {
	// 获取并清理路径（修复路径拼接逻辑）
	fullPath := c.Param("path")
	if fullPath == "" || fullPath == "/" {
		c.JSON(http.StatusBadRequest, result.Error(400, "请提供完整的访问路径，如 uploads/2025/11/abc.webp"))
		return
	}
	cleanPath := fmt.Sprintf("/%s", strings.TrimPrefix("uploads"+fullPath, "/"))

	// 获取数据库实例
	db := database.GetDB()
	if db == nil || db.DB == nil {
		c.JSON(http.StatusInternalServerError, result.Error(500, "数据库连接未初始化"))
		return
	}

	// 查询图片信息
	var image models.Image
	sqlResult := db.DB.Where("Url = ? OR Thumbnail = ?", cleanPath, cleanPath).First(&image)
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

	// 校验图片元信息
	if image.Width == 0 && image.Height == 0 {
		log.Printf("图片[%s]元信息不完整（宽高为0），继续代理访问", cleanPath)
		// 不直接返回错误，仅日志警告，避免影响正常访问
	}

	// 初始化WebDAV客户端
	var webDAVClient *webdav.WebDAVClient
	if image.Storage == "webdav" {
		if setting.WebdavURL == "" {
			c.JSON(http.StatusInternalServerError, result.Error(500, "WebDAV配置未设置（WebdavURL为空）"))
			return
		}
		webDAVClient = webdav.Client(webdav.Config{
			BaseURL:  setting.WebdavURL,
			Username: setting.WebdavUser,
			Password: setting.WebdavPass,
			Timeout:  30 * time.Second,
		})
		// 验证WebDAV连接（非阻塞，仅日志）
		go func() {
			ctx := context.Background()
			if _, err := webDAVClient.WebDAVStat(ctx, ""); err != nil {
				log.Printf("WebDAV连接验证失败: %v", err)
			}
		}()
	}

	var imageUrl string
	// 判断当前访问的是缩略图还是原图
	if image.Thumbnail == cleanPath {
		imageUrl = image.Thumbnail // 访问的是缩略图，直接用
	} else if image.Url == cleanPath {
		imageUrl = image.Url // 访问的是原图，直接用
	} else {
		// 兜底：优先缩略图，无则用原图（兼容异常场景）
		imageUrl = image.Thumbnail
		if imageUrl == "" {
			imageUrl = image.Url
		}
	}
	// URL为空则返回错误
	if imageUrl == "" {
		c.JSON(http.StatusNotFound, result.Error(404, "图片URL为空，无法访问"))
		return
	}

	switch image.Storage {
	case "default":
		proxyLocalFile(c, imageUrl, image.MimeType, setting)

	case "webdav":
		proxyWebDAVFile(c, imageUrl, image.MimeType, image.FileSize, setting, webDAVClient)

	case "s3", "r2":
		// 初始化S3客户端
		s3Client, err := s3.NewS3Client(setting)
		if err != nil {
			c.JSON(http.StatusInternalServerError, result.Error(500, fmt.Sprintf("S3/R2客户端初始化失败: %v", err)))
			return
		}
		// 代理S3/R2文件
		proxyS3File(c, imageUrl, image.MimeType, image.FileSize, setting, image.Storage, s3Client)

	default:
		c.JSON(http.StatusUnprocessableEntity, result.Error(422, fmt.Sprintf("不支持的存储类型: %s", image.Storage)))
	}
}

// proxyS3File S3/R2文件代理
func proxyS3File(c *gin.Context, objectKey, mimeType string, fileSize int64, cfg models.Settings, storageType string, s3Client *awss3.Client) {
	// 清理objectKey（去除开头的/，适配S3路径规则）
	objectKey = strings.TrimPrefix(objectKey, "/")

	// 获取bucket名称
	var bucket string = cfg.S3Bucket

	// 校验bucket和objectKey
	if bucket == "" || objectKey == "" {
		c.JSON(http.StatusInternalServerError, result.Error(500, "S3/R2配置缺失（Bucket或ObjectKey为空）"))
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// 1. 获取S3/R2文件对象
	getInput := awss3.GetObjectInput{
		Bucket: aws.String(bucket),
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

		log.Printf("S3/R2获取文件失败 [key:%s, bucket:%s]: %v", objectKey, bucket, err)
		c.JSON(http.StatusBadGateway, result.Error(502, "S3/R2文件获取失败"))
		return
	}
	defer resp.Body.Close()

	// 2. 设置响应头
	c.Header("Content-Type", mimeType)
	// 优先使用S3返回的文件大小，其次使用数据库中存储的大小
	if resp.ContentLength != nil && *resp.ContentLength > 0 {
		c.Header("Content-Length", strconv.FormatInt(*resp.ContentLength, 10))
	} else if fileSize > 0 {
		c.Header("Content-Length", strconv.FormatInt(fileSize, 10))
	}
	// 缓存控制（永久缓存）
	c.Header("Cache-Control", "public, max-age=31536000")
	// 存储类型标识
	c.Header("X-Storage-Type", storageType)
	// 跨域支持（可选）
	c.Header("Access-Control-Allow-Origin", "*")

	// 3. 流式传输文件（避免内存溢出）
	// 设置响应状态码
	c.Status(http.StatusOK)
	// 分块传输，每次4KB
	buf := make([]byte, 4096)
	_, err = io.CopyBuffer(c.Writer, resp.Body, buf)
	if err != nil && err != io.EOF {
		log.Printf("S3/R2文件传输失败 [key:%s]: %v", objectKey, err)
	}
}

// proxyWebDAVFile WebDAV文件代理
func proxyWebDAVFile(c *gin.Context, relPath, mimeType string, fileSize int64, cfg models.Settings, client *webdav.WebDAVClient) {
	// client为空时重新初始化
	if client == nil {
		client = webdav.Client(webdav.Config{
			BaseURL:  cfg.WebdavURL,
			Username: cfg.WebdavUser,
			Password: cfg.WebdavPass,
			Timeout:  30 * time.Second,
		})
	}

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

	// 设置响应头
	c.Header("Content-Type", mimeType)
	if resp.ContentLength > 0 {
		c.Header("Content-Length", strconv.FormatInt(resp.ContentLength, 10))
	} else if fileSize > 0 {
		c.Header("Content-Length", strconv.FormatInt(fileSize, 10))
	}
	c.Header("Cache-Control", "public, max-age=31536000")
	c.Header("X-Storage-Type", "webdav")
	c.Header("Access-Control-Allow-Origin", "*")

	// 流式传输文件
	_, err = io.Copy(c.Writer, resp.Body)
	if err != nil {
		log.Printf("WebDAV文件传输失败：%v", err)
	}
}

// proxyLocalFile 本地文件代理
func proxyLocalFile(c *gin.Context, realPath string, mimeType string, cfg models.Settings) {
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

	// 设置响应头
	c.Header("Content-Type", mimeType)
	c.Header("Content-Length", strconv.FormatInt(fileInfo.Size(), 10))
	c.Header("Cache-Control", "public, max-age=31536000")
	c.Header("X-Storage-Type", "default")
	c.Header("Access-Control-Allow-Origin", "*")

	// 流式传输
	c.File(fullPath)
}
