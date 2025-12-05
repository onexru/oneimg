package controllers

import (
	"log"
	"oneimg/backend/config"
	"oneimg/backend/database"
	"oneimg/backend/interfaces"
	"oneimg/backend/models"
	"oneimg/backend/utils/settings"
	"oneimg/backend/utils/telegram"
	"oneimg/backend/utils/uploads"
	"time"

	"github.com/gin-gonic/gin"
)

// UploadImages 图片上传主入口
func UploadImages(c *gin.Context) {
	// 初始化上传上下文
	uc := uploads.NewUploadContext(c)

	// 获取系统配置
	setting, err := settings.GetSettings()
	if err != nil {
		uc.Fail(500, "获取上传配置失败：%v", err)
		return
	}

	// 解析并校验上传文件
	files, err := uc.ParseAndValidateFiles()
	if err != nil {
		uc.Fail(400, "文件解析失败")
		return
	}

	// 获取存储上传器
	uploader, err := uc.GetStorageUploader(&setting)
	if err != nil {
		uc.Fail(400, "%s", err.Error())
		return
	}

	// 获取全局配置
	cfg, ok := c.MustGet("config").(*config.Config)
	if !ok {
		uc.Fail(500, "全局配置获取失败")
		return
	}

	// 批量处理文件上传（参数匹配接口定义）
	uploadResults := make([]interfaces.ImageUploadResult, 0, len(files))
	successCount := 0

	for _, file := range files {
		fileResult, err := uploader.Upload(c, cfg, &setting, file)
		if err != nil {
			// 单个文件上传失败不影响其他文件
			uc.Fail(500, "文件[%s]上传失败：%v", file.Filename, err)
			return
		}

		// 保存图片信息到数据库
		imageModel := models.Image{
			Url:       fileResult.URL,
			Thumbnail: fileResult.ThumbnailURL,
			FileName:  fileResult.FileName,
			FileSize:  fileResult.FileSize,
			MimeType:  fileResult.MimeType,
			Width:     fileResult.Width,
			Height:    fileResult.Height,
			Storage:   fileResult.Storage,
			UserId:    c.GetInt("user_id"),
		}

		db := database.GetDB()
		if db != nil {
			db.DB.Create(&imageModel)
		}

		uploadResults = append(uploadResults, *fileResult)

		if setting.TGNotice {
			placeholderData := telegram.PlaceholderData{
				Username:    c.GetString("username"),
				Date:        time.Now().Format("2006-01-02 15:04:05"),
				Filename:    fileResult.FileName,
				StorageType: setting.StorageType,
				URL:         c.Request.Host + fileResult.URL,
			}

			err := telegram.SendSimpleMsg(
				setting.TGBotToken,   // 机器人Token
				setting.TGReceivers,  // 接收者ChatID
				setting.TGNoticeText, // 模板文本
				placeholderData,      // 占位符数据
			)
			if err != nil {
				log.Println(err)
				// 忽略错误
			}
		}

		successCount++
	}

	if successCount == 0 {
		uc.Fail(500, "所有文件上传失败")
		return
	}

	// 返回上传结果
	uc.Success("上传成功", map[string]any{
		"files": uploadResults,
		"count": successCount,
	})
}

// UploadImage 单文件上传
func UploadImage(c *gin.Context) {
	UploadImages(c)
}
