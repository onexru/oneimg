package watermark

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
	"image/png"
	"io"
	"log"
	"math"
	"oneimg/backend/models"
	"os"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/golang/freetype"
	"github.com/golang/freetype/truetype"
	"golang.org/x/image/math/fixed"
)

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

// parseWatermarkParams 解析水印GET参数（新增动态字体参数）
func ParseWatermarkParams(c *gin.Context) WatermarkConfig {
	cfg := WatermarkConfig{
		Enable:            false,
		Text:              "初春图床",                                   // 默认水印文字
		Position:          "bottom-right",                           // 默认位置
		FontSize:          24,                                       // 默认固定字体大小
		FontSizeRatio:     0.1,                                      // 默认2%比例
		MinFontSize:       10,                                       // 最小10px
		MaxFontSize:       500,                                      // 最大50px
		FontColor:         "FFFFFF",                                 // 默认白色
		Opacity:           1,                                        // 默认透明度
		FontPath:          "./frontend/src/assets/fonts/jyhphy.ttf", // 默认字体文件路径
		EnableDynamicSize: true,                                     // 默认启用动态大小
	}

	// 获取水印开关参数
	watermark := c.DefaultQuery("watermark", "false")
	if watermark == "true" || watermark == "1" {
		cfg.Enable = true

		// 解析水印文字
		if text := c.Query("wm_text"); text != "" {
			cfg.Text = text
		}

		// 解析水印位置
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

		// 解析固定字体大小
		if sizeStr := c.Query("wm_size"); sizeStr != "" {
			if size, err := strconv.Atoi(sizeStr); err == nil && size > 0 && size <= 100 {
				cfg.FontSize = size
			}
		}

		// 新增：解析动态字体开关
		if dynamic := c.Query("wm_dynamic"); dynamic != "" {
			if dynamic == "false" || dynamic == "0" {
				cfg.EnableDynamicSize = false
			}
		}

		// 新增：解析字体比例
		if ratioStr := c.Query("wm_ratio"); ratioStr != "" {
			if ratio, err := strconv.ParseFloat(ratioStr, 64); err == nil && ratio > 0 && ratio <= 0.1 {
				cfg.FontSizeRatio = ratio
			}
		}

		// 新增：解析最小字体大小
		if minSizeStr := c.Query("wm_min_size"); minSizeStr != "" {
			if minSize, err := strconv.Atoi(minSizeStr); err == nil && minSize > 0 {
				cfg.MinFontSize = minSize
			}
		}

		// 新增：解析最大字体大小
		if maxSizeStr := c.Query("wm_max_size"); maxSizeStr != "" {
			if maxSize, err := strconv.Atoi(maxSizeStr); err == nil && maxSize > cfg.MinFontSize {
				cfg.MaxFontSize = maxSize
			}
		}

		// 解析字体颜色
		if colorStr := c.Query("wm_color"); colorStr != "" {
			// 自动去除#前缀，兼容#RRGGBB格式
			cfg.FontColor = strings.TrimPrefix(colorStr, "#")
			// 校验长度（6位）
			if len(cfg.FontColor) != 6 {
				cfg.FontColor = "FFFFFF" // 非法值重置为默认
			}
		}

		// 解析透明度
		if opacityStr := c.Query("wm_opacity"); opacityStr != "" {
			if opacity, err := strconv.ParseFloat(opacityStr, 64); err == nil && opacity >= 0 && opacity <= 1 {
				cfg.Opacity = opacity
			}
		}

		// 解析字体路径
		if fontPath := c.Query("wm_font"); fontPath != "" {
			cfg.FontPath = fontPath
		}
	}

	return cfg
}

// WatermarkSetting 设置水印设置参数（新增动态字体参数映射）
func WatermarkSetting(setting models.Settings) WatermarkConfig {
	// 从配置读取动态参数（若models.Settings无对应字段，可先写死默认值，后续扩展）
	var (
		ratio     = float64(setting.WatermarkSize) / 100.0 // 默认2%
		minSize   = 10                                     // 最小10px
		maxSize   = 600                                    // 最大50px
		dynamicOn = true                                   // 默认启用动态
	)

	// 扩展：若你的Settings模型已添加动态字体字段，可替换为：
	// ratio = setting.WatermarkSizeRatio
	// minSize = setting.WatermarkMinSize
	// maxSize = setting.WatermarkMaxSize
	// dynamicOn = setting.EnableDynamicWatermarkSize

	return WatermarkConfig{
		Enable:            true,
		Text:              setting.WatermarkText,                           // 默认水印文字
		Position:          setting.WatermarkPos,                            // 默认位置
		FontSize:          100,                                             // 固定字体大小
		FontSizeRatio:     ratio,                                           // 动态比例
		MinFontSize:       minSize,                                         // 最小字体
		MaxFontSize:       maxSize,                                         // 最大字体
		FontColor:         strings.TrimPrefix(setting.WatermarkColor, "#"), // 自动去#
		Opacity:           setting.WatermarkOpac,                           // 默认透明度
		FontPath:          "./frontend/src/assets/fonts/jyhphy.ttf",        // 默认字体文件路径
		EnableDynamicSize: dynamicOn,                                       // 启用动态大小
	}
}

// calculateDynamicFontSize 新增：动态计算字体大小
func calculateDynamicFontSize(imgBounds image.Rectangle, cfg WatermarkConfig) int {
	// 未启用动态，返回固定值
	if !cfg.EnableDynamicSize {
		return cfg.FontSize
	}

	// 获取图片宽高
	imgWidth := float64(imgBounds.Dx())
	imgHeight := float64(imgBounds.Dy())
	if imgWidth <= 0 || imgHeight <= 0 {
		return cfg.MinFontSize // 异常尺寸返回最小值
	}

	// 取最小边计算基准大小
	minSide := math.Min(imgWidth, imgHeight)
	dynamicSize := minSide * cfg.FontSizeRatio

	// 限制上下限
	if dynamicSize < float64(cfg.MinFontSize) {
		return cfg.MinFontSize
	}
	if dynamicSize > float64(cfg.MaxFontSize) {
		return cfg.MaxFontSize
	}

	// 按文字长度微调（文字越长，字体略小）
	textLen := float64(len(cfg.Text))
	if textLen > 10 {
		dynamicSize = dynamicSize * (10 / textLen)
		// 微调后仍保证不小于最小值
		if dynamicSize < float64(cfg.MinFontSize) {
			dynamicSize = float64(cfg.MinFontSize)
		}
	}

	return int(math.Round(dynamicSize)) // 四舍五入为整数
}

// addWatermarkToImage 给图片添加水印（集成动态字体计算）
func addWatermarkToImage(img image.Image, cfg WatermarkConfig) (image.Image, error) {
	if !cfg.Enable {
		return img, nil
	}

	// 加载字体文件
	fontFile, err := os.Open(cfg.FontPath)
	if err != nil {
		// 尝试使用系统默认字体路径
		defaultFontPaths := []string{
			"./frontend/src/assets/fonts/jyhphy.ttf",
			"/usr/share/fonts/truetype/dejavu/DejaVuSans.ttf",
			"/System/Library/Fonts/Helvetica.ttc",
			"C:/Windows/Fonts/simhei.ttf",
		}
		var fontErr error
		for _, path := range defaultFontPaths {
			fontFile, fontErr = os.Open(path)
			if fontErr == nil {
				break
			}
		}
		if fontFile == nil {
			log.Printf("无法加载字体文件: %v, 尝试默认字体也失败", err)
			return img, fmt.Errorf("无法加载字体文件")
		}
	}
	defer fontFile.Close()

	// 读取字体数据
	fontBytes, err := io.ReadAll(fontFile)
	if err != nil {
		log.Printf("读取字体文件失败: %v", err)
		return img, fmt.Errorf("读取字体文件失败: %v", err)
	}

	// 解析字体
	ttfFont, err := truetype.Parse(fontBytes)
	if err != nil {
		log.Printf("解析字体失败: %v", err)
		return img, fmt.Errorf("解析字体失败: %v", err)
	}

	// 创建RGBA图像用于绘制
	bounds := img.Bounds()
	rgba := image.NewRGBA(bounds)
	draw.Draw(rgba, bounds, img, image.Point{}, draw.Src)

	// 新增：动态计算最终字体大小
	finalFontSize := calculateDynamicFontSize(bounds, cfg)

	// 创建绘制上下文
	c := freetype.NewContext()
	c.SetDPI(72)
	c.SetFont(ttfFont)
	c.SetFontSize(float64(finalFontSize)) // 使用动态计算的字体大小
	c.SetClip(rgba.Bounds())
	c.SetDst(rgba)
	// 创建颜色源（修复color.Color到image.Image的转换）
	c.SetSrc(&image.Uniform{
		C: parseColor(cfg.FontColor, cfg.Opacity),
	})

	// 计算水印位置（传入最终字体大小）
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

// parseColor 解析颜色字符串 (RRGGBB) 并添加透明度（无修改）
func parseColor(colorStr string, opacity float64) color.Color {
	// 解析RRGGBB颜色（转为浮点数0-1范围）
	r, _ := strconv.ParseUint(colorStr[0:2], 16, 8)
	g, _ := strconv.ParseUint(colorStr[2:4], 16, 8)
	b, _ := strconv.ParseUint(colorStr[4:6], 16, 8)

	return &color.NRGBA{
		R: uint8(r),
		G: uint8(g),
		B: uint8(b),
		A: uint8(opacity * 255), // 直接设置alpha通道
	}
}

// calculateWatermarkPosition 计算水印位置（适配动态字体大小）
func calculateWatermarkPosition(img *image.RGBA, font *truetype.Font, text string, position string, fontSize int) (int, int) {
	// 获取图片尺寸
	imgWidth := img.Bounds().Dx()
	imgHeight := img.Bounds().Dy()

	// 优化：精确计算文字宽度（替代原简化计算）
	textWidth := 0
	if font != nil {
		// 使用字体度量计算精确宽度
		f := truetype.NewFace(font, &truetype.Options{Size: float64(fontSize)})
		for _, r := range text {
			advance, _ := f.GlyphAdvance(r)
			textWidth += int(advance >> 6) // 转换为像素
		}
	} else {
		// 降级简化计算
		textWidth = len(text) * fontSize * 2 / 3
	}
	textHeight := fontSize

	// 边距（动态适配图片大小，避免小图边距过大）
	margin := int(math.Max(10, float64(imgWidth)*0.01)) // 最小10px，或图片宽度的1%

	var x, y int

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
		y = (imgHeight + textHeight) / 2
	}

	// 确保位置在图片范围内
	if x < 0 {
		x = margin
	}
	if y < textHeight {
		y = textHeight + margin
	}
	if x+textWidth > imgWidth {
		x = imgWidth - textWidth - margin
	}
	if y > imgHeight {
		y = imgHeight - margin
	}

	return x, y
}

// processImageWithWatermark 处理图片流并添加水印（无修改）
func ProcessImageWithWatermark(reader io.Reader, mimeType string, cfg WatermarkConfig) (io.Reader, error) {
	if !cfg.Enable {
		return reader, nil
	}

	// 读取所有数据到缓冲区（避免reader只能读取一次的问题）
	buf, err := io.ReadAll(reader)
	if err != nil {
		log.Printf("读取图片数据失败: %v", err)
		return nil, fmt.Errorf("读取图片数据失败: %v", err)
	}
	imgReader := bytes.NewReader(buf)

	// 解码图片
	img, format, err := image.Decode(imgReader)
	if err != nil {
		log.Printf("解码图片失败: %v", err)
		return nil, fmt.Errorf("解码图片失败: %v", err)
	}

	// 添加水印
	watermarkedImg, err := addWatermarkToImage(img, cfg)
	if err != nil {
		log.Printf("添加水印失败: %v", err)
		return nil, fmt.Errorf("添加水印失败: %v", err)
	}

	// 编码图片 - 完全移除imaging.WebP相关代码
	outBuf := new(bytes.Buffer)
	switch format {
	case "jpeg", "jpg":
		err = jpeg.Encode(outBuf, watermarkedImg, &jpeg.Options{Quality: 90})
	case "png":
		err = png.Encode(outBuf, watermarkedImg)
	default:
		// 所有其他格式都使用JPEG编码
		err = jpeg.Encode(outBuf, watermarkedImg, &jpeg.Options{Quality: 90})
	}

	if err != nil {
		log.Printf("编码水印图片失败: %v", err)
		return nil, fmt.Errorf("编码水印图片失败: %v", err)
	}

	return bytes.NewReader(outBuf.Bytes()), nil
}
