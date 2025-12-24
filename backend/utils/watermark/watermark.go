package watermark

import (
	"bytes"
	"embed"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
	"image/png"
	"io"
	"io/fs"
	"log"
	"math"
	"oneimg/backend/models"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/golang/freetype"
	"github.com/golang/freetype/truetype"
	"golang.org/x/image/math/fixed"
)

var frontendFS fs.FS

// WatermarkConfig 水印配置（新增动态字体相关参数）
type WatermarkConfig struct {
	Enable            bool    // 是否启用水印
	Text              string  // 水印文字
	Position          string  // 水印位置：top-left, top-right, bottom-left, bottom-right, center
	FontSize          int     // 字体大小（固定值，优先级低于动态计算）
	FontSizeRatio     float64 // 字体大小占图片最小边的比例（0-1，如0.02=2%）
	MinFontSize       int     // 最小字体大小（px）
	MaxFontSize       int     // 最大字体大小（px）
	FontColor         string  // 字体颜色 (RRGGBB 格式)
	Opacity           float64 // 透明度 (0-1)
	FontPath          string  // 字体文件路径
	EnableDynamicSize bool    // 是否启用动态字体大小（默认true）
}

// Init 初始化字体文件系统
func Init(fontFs embed.FS) {
	distFS, err := fs.Sub(fontFs, "frontend/src/assets/fonts")
	if err != nil {
		log.Printf("警告：字体文件目录加载失败，将尝试使用系统字体: %v", err)
		return
	}
	frontendFS = distFS
}

// ParseWatermarkParams 解析水印GET参数
func ParseWatermarkParams(c *gin.Context) WatermarkConfig {
	cfg := WatermarkConfig{
		Enable:            false,
		Text:              "初春图床",
		Position:          "bottom-right",
		FontSize:          24,
		FontSizeRatio:     0.02,
		MinFontSize:       10,
		MaxFontSize:       500,
		FontColor:         "FFFFFF",
		Opacity:           1.0,
		FontPath:          "jyhphy.ttf",
		EnableDynamicSize: true,
	}

	watermark := c.DefaultQuery("watermark", "false")
	if watermark == "true" || watermark == "1" {
		cfg.Enable = true

		if text := c.Query("wm_text"); text != "" {
			cfg.Text = text
		}

		if pos := c.Query("wm_pos"); pos != "" {
			validPositions := map[string]bool{
				"top-left":     true,
				"top-right":    true,
				"bottom-left":  true,
				"bottom-right": true,
				"center":       true,
			}
			if validPositions[pos] {
				cfg.Position = pos
			}
		}

		if sizeStr := c.Query("wm_size"); sizeStr != "" {
			if size, err := strconv.Atoi(sizeStr); err == nil && size > 0 && size <= 100 {
				cfg.FontSize = size
			}
		}

		if dynamic := c.Query("wm_dynamic"); dynamic != "" {
			if dynamic == "false" || dynamic == "0" {
				cfg.EnableDynamicSize = false
			}
		}

		if ratioStr := c.Query("wm_ratio"); ratioStr != "" {
			if ratio, err := strconv.ParseFloat(ratioStr, 64); err == nil && ratio > 0 && ratio <= 0.1 {
				cfg.FontSizeRatio = ratio
			}
		}

		if minSizeStr := c.Query("wm_min_size"); minSizeStr != "" {
			if minSize, err := strconv.Atoi(minSizeStr); err == nil && minSize > 0 {
				cfg.MinFontSize = minSize
			}
		}

		if maxSizeStr := c.Query("wm_max_size"); maxSizeStr != "" {
			if maxSize, err := strconv.Atoi(maxSizeStr); err == nil && maxSize > cfg.MinFontSize {
				if maxSize > 100 {
					maxSize = 100
				}
				cfg.MaxFontSize = maxSize
			}
		}

		if colorStr := c.Query("wm_color"); colorStr != "" {
			colorStr = strings.TrimPrefix(colorStr, "#")
			if len(colorStr) == 6 {
				cfg.FontColor = colorStr
			} else {
				cfg.FontColor = "FFFFFF"
			}
		}

		if opacityStr := c.Query("wm_opacity"); opacityStr != "" {
			if opacity, err := strconv.ParseFloat(opacityStr, 64); err == nil && opacity >= 0 && opacity <= 1 {
				cfg.Opacity = opacity
			}
		}

		if fontPath := c.Query("wm_font"); fontPath != "" {
			cfg.FontPath = fontPath
		}
	}

	return cfg
}

// WatermarkSetting 设置水印设置参数
func WatermarkSetting(setting models.Settings) WatermarkConfig {
	var (
		ratio     = 0.02
		minSize   = 10
		maxSize   = 500
		dynamicOn = true
		fontSize  = 24
	)

	if setting.WatermarkSize > 0 {
		ratio = math.Min(float64(setting.WatermarkSize)/100.0, 0.1)
	}

	return WatermarkConfig{
		Enable:            true,
		Text:              setting.WatermarkText,
		Position:          setting.WatermarkPos,
		FontSize:          fontSize,
		FontSizeRatio:     ratio,
		MinFontSize:       minSize,
		MaxFontSize:       maxSize,
		FontColor:         strings.TrimPrefix(setting.WatermarkColor, "#"),
		Opacity:           setting.WatermarkOpac,
		FontPath:          "jyhphy.ttf",
		EnableDynamicSize: dynamicOn,
	}
}

// calculateDynamicFontSize 动态计算字体大小
func calculateDynamicFontSize(imgBounds image.Rectangle, cfg WatermarkConfig) int {
	if !cfg.EnableDynamicSize {
		return cfg.FontSize
	}

	imgWidth := float64(imgBounds.Dx())
	imgHeight := float64(imgBounds.Dy())
	if imgWidth <= 0 || imgHeight <= 0 {
		return cfg.MinFontSize
	}

	minSide := math.Min(imgWidth, imgHeight)
	dynamicSize := minSide * cfg.FontSizeRatio

	dynamicSize = math.Max(float64(cfg.MinFontSize), dynamicSize)
	dynamicSize = math.Min(float64(cfg.MaxFontSize), dynamicSize)

	textLen := float64(len(cfg.Text))
	if textLen > 10 {
		scale := math.Max(0.5, 10/textLen)
		dynamicSize = dynamicSize * scale
		dynamicSize = math.Max(float64(cfg.MinFontSize), dynamicSize)
	}

	return int(math.Round(dynamicSize))
}

// loadFontBytes 加载字体文件字节数据
func loadFontBytes(cfg WatermarkConfig) ([]byte, error) {
	// 1. 优先从嵌入的文件系统加载
	if frontendFS != nil {
		fontFile, err := frontendFS.Open(cfg.FontPath)
		if err == nil {
			defer fontFile.Close()
			return io.ReadAll(fontFile)
		}
		log.Printf("从嵌入文件系统加载字体失败: %v", err)
	}

	// 2. 尝试从本地文件系统加载
	localPaths := []string{
		cfg.FontPath,
		filepath.Join("frontend", "src", "assets", "fonts", cfg.FontPath),
		filepath.Join(".", cfg.FontPath),
	}

	for _, path := range localPaths {
		if _, err := os.Stat(path); err == nil {
			fontFile, err := os.Open(path)
			if err == nil {
				defer fontFile.Close()
				return io.ReadAll(fontFile)
			}
			log.Printf("从本地路径 %s 加载字体失败: %v", path, err)
		}
	}

	// 3. 尝试系统默认字体
	defaultFontPaths := []string{
		"/usr/share/fonts/truetype/dejavu/DejaVuSans.ttf",
		"/System/Library/Fonts/Helvetica.ttc",
		"C:/Windows/Fonts/simhei.ttf",
		"C:/Windows/Fonts/msyh.ttc",
	}

	for _, path := range defaultFontPaths {
		if _, err := os.Stat(path); err == nil {
			fontFile, err := os.Open(path)
			if err == nil {
				defer fontFile.Close()
				return io.ReadAll(fontFile)
			}
		}
	}

	return nil, fmt.Errorf("无法加载字体文件 %s，所有备选路径都失败", cfg.FontPath)
}

// addWatermarkToImage 给图片添加水印
func addWatermarkToImage(img image.Image, cfg WatermarkConfig) (image.Image, error) {
	if !cfg.Enable {
		return img, nil
	}

	fontBytes, err := loadFontBytes(cfg)
	if err != nil {
		log.Printf("加载字体失败: %v", err)
		return img, fmt.Errorf("加载字体失败: %v", err)
	}

	ttfFont, err := truetype.Parse(fontBytes)
	if err != nil {
		log.Printf("解析字体失败: %v", err)
		return img, fmt.Errorf("解析字体失败: %v", err)
	}

	bounds := img.Bounds()
	rgba := image.NewRGBA(bounds)
	draw.Draw(rgba, bounds, img, image.Point{}, draw.Src)

	finalFontSize := calculateDynamicFontSize(bounds, cfg)

	// 创建绘制上下文
	c := freetype.NewContext()
	c.SetDPI(72)
	c.SetFont(ttfFont)
	c.SetFontSize(float64(finalFontSize))
	c.SetClip(rgba.Bounds())
	c.SetDst(rgba)
	c.SetSrc(&image.Uniform{C: parseColor(cfg.FontColor, cfg.Opacity)})

	// 计算水印位置
	x, y := calculateWatermarkPosition(rgba, ttfFont, cfg.Text, cfg.Position, finalFontSize)

	// 绘制水印文字
	_, err = c.DrawString(cfg.Text, fixed.Point26_6{
		X: fixed.Int26_6(x * 64),
		Y: fixed.Int26_6(y * 64),
	})
	if err != nil {
		log.Printf("绘制水印失败: %v", err)
		return img, fmt.Errorf("绘制水印失败: %v", err)
	}

	return rgba, nil
}

// parseColor 解析颜色字符串 (RRGGBB) 并添加透明度
func parseColor(colorStr string, opacity float64) color.Color {
	if len(colorStr) != 6 {
		colorStr = "FFFFFF"
	}

	r, _ := strconv.ParseUint(colorStr[0:2], 16, 8)
	g, _ := strconv.ParseUint(colorStr[2:4], 16, 8)
	b, _ := strconv.ParseUint(colorStr[4:6], 16, 8)

	opacity = math.Max(0, math.Min(1, opacity))

	return &color.NRGBA{
		R: uint8(r),
		G: uint8(g),
		B: uint8(b),
		A: uint8(opacity * 255),
	}
}

// calculateWatermarkPosition 计算水印位置（修复了Hinting错误）
func calculateWatermarkPosition(img *image.RGBA, font *truetype.Font, text string, position string, fontSize int) (int, int) {
	// 获取图片尺寸
	imgWidth := img.Bounds().Dx()
	imgHeight := img.Bounds().Dy()

	// 计算文字宽度和高度
	textWidth := 0
	textHeight := fontSize

	if font != nil && len(text) > 0 {
		// 修复：移除了错误的 font.Hinting，使用默认的 Hinting 设置
		face := truetype.NewFace(font, &truetype.Options{
			Size: float64(fontSize),
			DPI:  72,
		})
		defer face.Close()

		// 计算总宽度
		var totalAdvance fixed.Int26_6
		for _, r := range text {
			advance, ok := face.GlyphAdvance(r)
			if ok {
				totalAdvance += advance
			} else {
				totalAdvance += fixed.I(fontSize)
			}
		}
		textWidth = int(totalAdvance >> 6)

		// 计算文字高度（包含基线）
		metrics := face.Metrics()
		textHeight = int((metrics.Ascent + metrics.Descent) >> 6)
	}

	// 动态边距
	margin := int(math.Max(8, math.Min(20, float64(imgWidth)*0.015)))

	var x, y int

	// 计算基准位置
	switch position {
	case "top-left":
		x = margin
		y = textHeight + margin
	case "top-right":
		x = imgWidth - textWidth - margin
		y = textHeight + margin
	case "bottom-left":
		x = margin
		y = imgHeight - margin
	case "bottom-right":
		x = imgWidth - textWidth - margin
		y = imgHeight - margin
	case "center":
		x = (imgWidth - textWidth) / 2
		y = (imgHeight + textHeight/2) / 2
	}

	// 边界检查
	x = clamp(x, 0, imgWidth-textWidth-margin)
	y = clamp(y, textHeight, imgHeight-margin)

	return x, y
}

// clamp 辅助函数：将值限制在min和max之间
func clamp(value, min, max int) int {
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}

// ProcessImageWithWatermark 处理图片流并添加水印
func ProcessImageWithWatermark(reader io.Reader, mimeType string, cfg WatermarkConfig) (io.Reader, error) {
	if !cfg.Enable {
		return reader, nil
	}

	buf, err := io.ReadAll(reader)
	if err != nil {
		log.Printf("读取图片数据失败: %v", err)
		return nil, fmt.Errorf("读取图片数据失败: %v", err)
	}

	img, format, err := image.Decode(bytes.NewReader(buf))
	if err != nil {
		log.Printf("解码图片失败: %v", err)
		return nil, fmt.Errorf("解码图片失败: %v", err)
	}

	watermarkedImg, err := addWatermarkToImage(img, cfg)
	if err != nil {
		log.Printf("添加水印失败: %v", err)
		return nil, fmt.Errorf("添加水印失败: %v", err)
	}

	outBuf := new(bytes.Buffer)
	switch strings.ToLower(format) {
	case "jpeg", "jpg":
		err = jpeg.Encode(outBuf, watermarkedImg, &jpeg.Options{Quality: 90})
	case "png":
		err = png.Encode(outBuf, watermarkedImg)
	default:
		err = jpeg.Encode(outBuf, watermarkedImg, &jpeg.Options{Quality: 90})
	}

	if err != nil {
		log.Printf("编码水印图片失败: %v", err)
		return nil, fmt.Errorf("编码水印图片失败: %v", err)
	}

	return bytes.NewReader(outBuf.Bytes()), nil
}

// GetFontFile 辅助函数：获取字体文件
func GetFontFile() (fs.File, error) {
	if frontendFS == nil {
		return nil, fs.ErrNotExist
	}
	return frontendFS.Open("jyhphy.ttf")
}
