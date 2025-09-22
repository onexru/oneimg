package controllers

import (
	"fmt"
	"math/rand"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"oneimg/backend/config"
	"oneimg/backend/database"
	"oneimg/backend/models"
	"oneimg/backend/services"

	"github.com/gin-gonic/gin"
)

// UploadResponse 上传响应结构
type UploadResponse struct {
	Code    int           `json:"code"`
	Message string        `json:"message"`
	Data    []ImageResult `json:"data"`
}

// ImageResult 单个图片上传结果
type ImageResult struct {
	Success   bool   `json:"success"`
	Message   string `json:"message,omitempty"`
	ID        int    `json:"id,omitempty"`
	URL       string `json:"url,omitempty"`
	FileName  string `json:"filename,omitempty"`
	FileSize  int64  `json:"file_size,omitempty"`
	MimeType  string `json:"mime_type,omitempty"`
	Width     int    `json:"width,omitempty"`
	Height    int    `json:"height,omitempty"`
	CreatedAt string `json:"created_at,omitempty"`
}

// UploadImages 批量上传图片
func UploadImages(c *gin.Context) {
	// 获取配置
	cfg := c.MustGet("config").(*config.Config)

	// 解析multipart表单
	form, err := c.MultipartForm()
	if err != nil {
		c.JSON(http.StatusBadRequest, UploadResponse{
			Code:    400,
			Message: "解析表单失败: " + err.Error(),
			Data:    []ImageResult{},
		})
		return
	}

	// 获取images[]字段的文件
	files := form.File["images[]"]
	if len(files) == 0 {
		c.JSON(http.StatusBadRequest, UploadResponse{
			Code:    400,
			Message: "没有找到上传的图片文件",
			Data:    []ImageResult{},
		})
		return
	}

	// 检查上传文件数量限制
	maxFiles := 10 // 最多同时上传10个文件
	if len(files) > maxFiles {
		c.JSON(http.StatusBadRequest, UploadResponse{
			Code:    400,
			Message: fmt.Sprintf("一次最多只能上传%d个文件", maxFiles),
			Data:    []ImageResult{},
		})
		return
	}

	// 确保上传目录存在
	if err := ensureUploadDir(cfg.UploadPath); err != nil {
		c.JSON(http.StatusInternalServerError, UploadResponse{
			Code:    500,
			Message: "创建上传目录失败: " + err.Error(),
			Data:    []ImageResult{},
		})
		return
	}

	// 获取数据库实例
	db := database.GetDB()
	if db == nil {
		c.JSON(http.StatusInternalServerError, UploadResponse{
			Code:    500,
			Message: "数据库连接失败",
			Data:    []ImageResult{},
		})
		return
	}

	var results []ImageResult

	// 处理每个上传的文件
	for _, fileHeader := range files {
		result := processUploadFile(fileHeader, cfg, db)
		results = append(results, result)
	}

	// 统计成功和失败的数量
	successCount := 0
	for _, result := range results {
		if result.Success {
			successCount++
		}
	}

	message := fmt.Sprintf("上传完成，成功: %d，失败: %d", successCount, len(files)-successCount)

	c.JSON(http.StatusOK, UploadResponse{
		Code:    200,
		Message: message,
		Data:    results,
	})
}

// processUploadFile 处理单个上传文件
func processUploadFile(fileHeader *multipart.FileHeader, cfg *config.Config, db *database.Database) ImageResult {
	// 验证图片
	if err := services.ImageSvc.ValidateImage(fileHeader, cfg.AllowedTypes, cfg.MaxFileSize); err != nil {
		return ImageResult{
			Success: false,
			Message: "文件验证失败: " + err.Error(),
		}
	}

	// 打开文件
	file, err := fileHeader.Open()
	if err != nil {
		return ImageResult{
			Success: false,
			Message: "打开文件失败: " + err.Error(),
		}
	}
	defer file.Close()

	// 处理图片（压缩、获取尺寸等）
	processedImage, err := services.ImageSvc.ProcessImage(file, fileHeader)
	if err != nil {
		return ImageResult{
			Success: false,
			Message: "处理图片失败: " + err.Error(),
		}
	}

	// 确定输出格式和扩展名
	originalExt := filepath.Ext(fileHeader.Filename)
	outputExt := determineOutputFormat(fileHeader.Header.Get("Content-Type"), originalExt)
	uniqueFileName := generateUniqueFileName(outputExt)

	// 创建年/月子目录
	now := time.Now()
	year := now.Format("2006")
	month := now.Format("01")
	subDir := filepath.Join(cfg.UploadPath, year, month)

	// 确保年月子目录存在
	if err := ensureUploadDir(subDir); err != nil {
		return ImageResult{
			Success: false,
			Message: "创建年月目录失败: " + err.Error(),
		}
	}

	// 构建文件路径
	filePath := filepath.Join(subDir, uniqueFileName)

	// 保存处理后的图片文件
	if err := saveFile(filePath, processedImage.CompressedBytes); err != nil {
		return ImageResult{
			Success: false,
			Message: "保存文件失败: " + err.Error(),
		}
	}

	// 构建访问URL (包含年/月子目录)
	fileURL := "/uploads/" + year + "/" + month + "/" + uniqueFileName

	// 保存到数据库
	imageModel := models.Image{
		Url:       fileURL,
		FileName:  uniqueFileName,
		FileSize:  int64(len(processedImage.CompressedBytes)),
		MimeType:  processedImage.MimeType,
		Width:     processedImage.Width,
		Height:    processedImage.Height,
		CreatedAt: time.Now(),
	}

	result := db.DB.Create(&imageModel)
	if result.Error != nil {
		// 如果数据库保存失败，删除已保存的文件
		os.Remove(filePath)
		return ImageResult{
			Success: false,
			Message: "保存到数据库失败: " + result.Error.Error(),
		}
	}

	return ImageResult{
		Success:   true,
		ID:        imageModel.Id,
		URL:       imageModel.Url,
		FileName:  imageModel.FileName,
		FileSize:  imageModel.FileSize,
		MimeType:  imageModel.MimeType,
		Width:     imageModel.Width,
		Height:    imageModel.Height,
		CreatedAt: imageModel.CreatedAt.Format("2006-01-02 15:04:05"),
	}
}

// ensureUploadDir 确保上传目录存在
func ensureUploadDir(uploadPath string) error {
	if _, err := os.Stat(uploadPath); os.IsNotExist(err) {
		return os.MkdirAll(uploadPath, 0755)
	}
	return nil
}

// generateUniqueFileName 生成唯一文件名 (哈希+3位随机数)
func generateUniqueFileName(ext string) string {
	// 使用当前时间戳生成哈希
	timestamp := time.Now().UnixNano()
	hash := fmt.Sprintf("%x", timestamp)

	// 生成3位随机数 (100-999)
	rand.Seed(time.Now().UnixNano())
	randomNum := rand.Intn(900) + 100

	return fmt.Sprintf("%s%d%s", hash, randomNum, ext)
}

// determineOutputFormat 确定输出格式
func determineOutputFormat(contentType, originalExt string) string {
	// 保持原格式的特殊类型
	specialFormats := map[string]string{
		"image/gif":     ".gif",
		"image/svg+xml": ".svg",
	}

	// 检查Content-Type
	if ext, exists := specialFormats[contentType]; exists {
		return ext
	}

	// 检查文件扩展名
	switch strings.ToLower(originalExt) {
	case ".gif":
		return ".gif"
	case ".svg":
		return ".svg"
	default:
		// 其他格式转换为webp
		return ".webp"
	}
}

// saveFile 保存文件到磁盘
func saveFile(filePath string, data []byte) error {
	file, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	_, err = file.Write(data)
	return err
}

// UploadImage 单个图片上传（兼容性接口）
func UploadImage(c *gin.Context) {
	// 获取上传文件
	file, header, err := c.Request.FormFile("image")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "上传失败: " + err.Error(),
			"data":    []string{},
		})
		return
	}
	defer file.Close()

	// 获取配置
	cfg := c.MustGet("config").(*config.Config)

	// 验证图片
	if err := services.ImageSvc.ValidateImage(header, cfg.AllowedTypes, cfg.MaxFileSize); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "文件验证失败: " + err.Error(),
			"data":    []string{},
		})
		return
	}

	// 获取数据库实例
	db := database.GetDB()
	if db == nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "数据库连接失败",
			"data":    []string{},
		})
		return
	}

	// 处理单个文件
	result := processUploadFile(header, cfg, db)

	if result.Success {
		c.JSON(http.StatusOK, gin.H{
			"code":    200,
			"message": "上传成功",
			"data":    result,
		})
	} else {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": result.Message,
			"data":    []string{},
		})
	}
}
