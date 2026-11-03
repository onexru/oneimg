package images

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"oneimg/backend/database"
	"oneimg/backend/models"
	"oneimg/backend/services"
	"oneimg/backend/utils/images"
	"oneimg/backend/utils/md5"
	"oneimg/backend/utils/safefetch"
	"oneimg/backend/utils/settings"
	"oneimg/backend/utils/telegram"
	"oneimg/backend/utils/uploads"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	storagectrl "oneimg/backend/controllers/storage"
	usersctrl "oneimg/backend/controllers/users"
)

// fetchUploadURL is injectable for isolated controller tests; production keeps
// safefetch's DNS, redirect, and private-address defenses.
var fetchUploadURL = safefetch.Get

// 通过URL上传图片
// urlFetchSlots 限制同时进行的远端下载数量。
//
// 与上传槽解耦：远端下载是「不可控时长的外部 IO」，不该占用上传槽；
// 但它同样需要上限，否则几打并发就能把出站带宽与内存吃满。
// 容量比上传槽宽一些（下载常比写盘慢，太紧会让正常批量 URL 上传排队）。
var urlFetchSlots = make(chan struct{}, 6)

// acquireURLFetchSlot 非阻塞地占一个下载槽；满了就返回 429，
// 不排队 —— 排队会让请求一直挂着，反而更容易被用来堆积连接。
func acquireURLFetchSlot(c *gin.Context) (func(), bool) {
	select {
	case urlFetchSlots <- struct{}{}:
		var once sync.Once
		release := func() { once.Do(func() { <-urlFetchSlots }) }
		time.AfterFunc(urlFetchSlotHoldLimit, release)
		return release, true
	default:
		c.JSON(http.StatusTooManyRequests, gin.H{"code": 429, "message": "URL 下载任务较多，请稍后重试"})
		return nil, false
	}
}

// urlFetchSlotHoldLimit 兜底释放：正常路径靠 defer，这里防止某条路径忘记释放。
const urlFetchSlotHoldLimit = 90 * time.Second

func UploadImagesByURL(c *gin.Context) {
	// 注意：这里**不**在入口抢全局上传槽。
	//
	// 原实现在第一行就 acquireUploadRequest，然后才去拉远端 URL（最长 60s）。
	// 而上传槽是进程级全局容量（2 个），于是攻击者只要架一个「先回 200 + image/png、
	// 再极慢吐 body」的服务器，两个并发请求就能把两个槽占满整整 60 秒，
	// 期间所有上传接口返回 429；循环重发即可持续饿死上传。
	// 现在改为：远端下载与格式校验全部做完（不再持有任何全局资源），
	// 真正要写盘前再申请上传槽 —— 持有时间收敛到本地写入这一步。
	uploads.WrapBody(c, 32<<10)
	defer func() {
		if c.Request.MultipartForm != nil {
			_ = c.Request.MultipartForm.RemoveAll()
		}
	}()
	uc := uploads.NewUploadContext(c)
	if c.GetInt("user_role") == models.RoleGuest && (c.GetInt("user_id") >= 0 || !validGuestUUID(c.GetString("username"))) {
		uc.Fail(403, "游客身份无效")
		return
	}
	db := database.GetDB()

	type URLUploadRequest struct {
		Urls     string `json:"url" binding:"required"`
		Tag      string `json:"tag_id"`
		FolderID int    `json:"folder_id"`
		BucketID string `json:"bucket_id"` // 兼容旧客户端，目标存储源以用户配置为准。
	}

	var req URLUploadRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		uc.Fail(400, "参数格式错误：%v", err)
		return
	}

	if !validateUploadFolder(c, req.FolderID) {
		return
	}
	if req.Urls == "" {
		uc.Fail(400, "URL不能为空")
		return
	}
	if req.Tag != "" && req.Tag != "0" {
		var tags models.Tags
		if err := db.DB.Where("id = ?", req.Tag).First(&tags).Error; err != nil {
			uc.Fail(400, "标签不存在")
			return
		}
	}

	setting, err := settings.GetSettings()
	if err != nil {
		uc.Fail(500, "获取上传配置失败：%v", err)
		return
	}
	if !setting.MultiStorageSync {
		uploadImageByURLLegacy(c, setting, req.Urls, req.Tag, req.BucketID, req.FolderID)
		return
	}

	localBucket, syncBuckets, err := storagectrl.ResolveUploadBuckets(c, setting)
	if err != nil {
		uc.Fail(500, "获取用户同步存储源失败：%v", err)
		return
	}

	// 下载图片（拒绝内网/metadata，防止 SSRF）
	if err := safefetch.ValidatePublicHTTPURL(req.Urls); err != nil {
		uc.Fail(400, "URL 不合法或禁止访问内网地址")
		return
	}
	// 下载阶段用独立配额，不再借占全局上传槽。
	fetchRelease, ok := acquireURLFetchSlot(c)
	if !ok {
		return
	}
	resp, err := fetchUploadURL(c.Request.Context(), req.Urls, 60*time.Second)
	if err != nil {
		fetchRelease()
		uc.Fail(400, "图片下载失败：%v", err)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		uc.Fail(400, "图片下载失败，远端状态码：%d", resp.StatusCode)
		return
	}

	fileName := filepath.Base(req.Urls)
	if fileName == "/" || fileName == "." || fileName == "" {
		fileName = fmt.Sprintf("url_image_%d.jpg", time.Now().Unix())
	}
	fileName = filepath.Base(strings.ReplaceAll(fileName, "\\", "/"))
	if strings.Contains(fileName, "..") || fileName == "." || fileName == "" {
		fileName = fmt.Sprintf("url_image_%d.jpg", time.Now().Unix())
	}

	maxBytes := int64(setting.MaxFileSize)
	if maxBytes <= 0 || maxBytes > images.MaxUploadBytes {
		maxBytes = images.MaxUploadBytes
	}
	fileBytes, err := safefetch.ReadLimited(resp.Body, maxBytes)
	// 内容已完整读入内存，远端连接到此结束，立刻交还下载配额 ——
	// 后面的 MIME 校验与写盘不该继续占着「同时在下载」的名额。
	resp.Body.Close()
	fetchRelease()
	if err != nil {
		uc.Fail(http.StatusBadRequest, "URL 图片超过限制或下载失败")
		return
	}
	info, err := inspectDownloadedImage(fileBytes)
	if err != nil || !images.AllowedMIME(info.MIME, strings.Split(setting.AllowedTypes, ",")) {
		uc.Fail(http.StatusBadRequest, "URL不是允许的安全图片")
		return
	}
	contentType := info.MIME

	// 内容校验通过才申请全局上传槽：此时只剩本地写入，
	// 持有时间短且可控，不会被慢速远端拖住。
	release, ok := uploads.AcquireRequest(c)
	if !ok {
		return
	}
	defer release()

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	part, _ := writer.CreatePart(map[string][]string{
		"Content-Disposition": {`form-data; name="file"; filename="` + fileName + `"`},
		"Content-Type":        {contentType},
	})
	part.Write(fileBytes)
	writer.Close()

	// 伪装请求
	c.Request.Body = io.NopCloser(body)
	c.Request.Header.Set("Content-Type", writer.FormDataContentType())
	c.Request.ContentLength = int64(body.Len())

	file, header, err := c.Request.FormFile("file")
	if err != nil {
		uc.Fail(500, "构造文件失败：%v", err)
		return
	}
	defer file.Close()

	// 账号配额检查：内容已在内存里，此时才知道确切字节数，
	// 且尚未写盘 —— 是配额校验最准确的时机。
	if err := usersctrl.EnforceUserQuota(c, db.DB, c.GetInt("user_id"), int64(len(fileBytes))); err != nil {
		if errors.Is(err, usersctrl.ErrQuotaExceeded) {
			usersctrl.FailQuota(uc, db.DB, c.GetInt("user_id"))
			return
		}
		uc.Fail(http.StatusInternalServerError, "配额校验失败，请稍后重试")
		return
	}

	uploader, err := uc.GetStorageUploader(&setting, &localBucket)
	if err != nil {
		uc.Fail(500, "初始化本机存储失败：%s", err.Error())
		return
	}

	fileResult, err := uploader.Upload(c, &setting, &localBucket, header)
	if err != nil {
		uc.Fail(500, "保存到本机失败[%s]：%v", fileName, err)
		return
	}

	imageModel := models.Image{
		Url:       fileResult.URL,
		Thumbnail: fileResult.ThumbnailURL,
		FileName:  fileResult.FileName,
		FileSize:  fileResult.FileSize,
		MimeType:  fileResult.MimeType,
		Width:     fileResult.Width,
		Height:    fileResult.Height,
		Storage:   fileResult.Storage,
		BucketId:  localBucket.Id,
		FolderId:  req.FolderID,
		UserId:    c.GetInt("user_id"),
		MD5:       md5.Md5(c.GetString("username") + fileResult.FileName),
		UUID:      c.GetString("username"),
	}

	now := time.Now()
	err = db.DB.Transaction(func(tx *gorm.DB) error {
		resolved, err := services.ResolvePublicationFolder(tx, imageModel.UserId, imageModel.FolderId)
		if err != nil {
			return err
		}
		imageModel.FolderId = resolved
		// 配额必须在 Create 之前判：SUM 会把刚插入的这一行也算进去。
		if err := usersctrl.EnforceQuotaInTx(tx, imageModel.UserId, imageModel.FileSize); err != nil {
			return err
		}
		if err := tx.Create(&imageModel).Error; err != nil {
			return err
		}
		localStatus := models.ImageStorage{
			ImageID:       imageModel.Id,
			BucketID:      localBucket.Id,
			Storage:       localBucket.Type,
			Status:        models.ImageStorageStatusSuccess,
			URL:           fileResult.URL,
			Thumbnail:     fileResult.ThumbnailURL,
			FileSize:      fileResult.FileSize,
			ThumbnailSize: fileResult.ThumbnailSize,
			SyncedAt:      &now,
		}
		if err := tx.Create(&localStatus).Error; err != nil {
			return err
		}
		for _, bucket := range syncBuckets {
			storageStatus := models.ImageStorage{
				ImageID:       imageModel.Id,
				BucketID:      bucket.Id,
				Storage:       bucket.Type,
				Status:        models.ImageStorageStatusPending,
				URL:           fileResult.URL,
				Thumbnail:     fileResult.ThumbnailURL,
				FileSize:      fileResult.FileSize,
				ThumbnailSize: fileResult.ThumbnailSize,
			}
			if err := tx.Create(&storageStatus).Error; err != nil {
				return err
			}
		}
		if req.Tag != "" && req.Tag != "0" {
			tagID, conversionErr := strconv.Atoi(req.Tag)
			if conversionErr != nil {
				return conversionErr
			}
			if err := tx.Create(&models.ImageToTags{ImageId: imageModel.Id, TagId: tagID}).Error; err != nil {
				return err
			}
		}
		return chargeUploadQuota(tx, localBucket, fileResult)
	})
	if err != nil {
		storagectrl.CleanupLocalUpload(imageModel)
		if errors.Is(err, usersctrl.ErrQuotaExceeded) {
			uc.Fail(http.StatusForbidden, "%s", usersctrl.QuotaExceededShortMessage(c, db.DB))
			return
		}
		uc.Fail(500, "保存文件记录失败")
		return
	}

	// TG通知
	if setting.TGNotice {
		placeholderData := telegram.PlaceholderData{
			Username:    c.GetString("username"),
			Date:        time.Now().Format("2006-01-02 15:04:05"),
			Filename:    fileResult.FileName,
			StorageType: localBucket.Type,
			URL:         buildImageResponseURL(c, setting, localBucket.Type, localBucket.Id, fileResult.URL),
		}
		sendTGNotification(setting, placeholderData)
	}

	responseResult := *fileResult
	responseResult.ID = imageModel.Id
	responseResult.FolderID = imageModel.FolderId
	services.WakeStorageSyncWorker()

	uc.Success("URL 图片已保存到本机，正在后台同步", map[string]any{
		"file":         responseResult,
		"sync_targets": len(syncBuckets),
	})
}
