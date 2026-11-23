package images

import (
	"context"
	"errors"
	"fmt"
)

// 逐项上传失败的分类。分类是稳定契约，文案可以改；调用方按 Code 分支即可，
// 不必去匹配中文。
const (
	FailureTooLarge   = "file_too_large"
	FailureDimensions = "image_dimensions"
	FailureFormat     = "unsupported_format"
	FailureDecode     = "invalid_image"
	FailureTimeout    = "upload_timeout"
	// 以下两个由调用方补充：images 包不该知道存储驱动的事。
	FailureStorage = "storage_failed"
	FailureGeneric = "upload_failed"
)

// DimensionLimitText 把像素与边长上限写成一句话。
// 直传、普通上传、文档三处都用它，避免限制值改了文案没跟上。
func DimensionLimitText() string {
	return fmt.Sprintf("最多 %d 万像素、单边 %d px", DirectMaxPixels/10000, DirectMaxDimension)
}

// ByteLimitText 人类可读的体积上限。
func ByteLimitText(limit int64) string {
	if limit <= 0 {
		limit = MaxUploadBytes
	}
	if limit%(1<<20) == 0 {
		return fmt.Sprintf("%d MB", limit>>20)
	}
	return fmt.Sprintf("%.1f MB", float64(limit)/(1<<20))
}

// DescribeFailure 把底层错误翻译成错误码 + 可直接照做的说明。
//
// 这些错误此前在控制器里被一律替换成「文件上传失败」，于是「体积超限」
// 「像素超限」「格式不支持」「解码损坏」在响应里长得一模一样，接入方只能
// 靠二分试尺寸反推（见 issue #58）。识别不出的错误返回空码，由调用方决定
// 是存储失败还是通用失败——这里只翻译它确实认识的原因。
func DescribeFailure(err error, maxBytes int64) (code, message string) {
	switch {
	case err == nil:
		return "", ""
	case errors.Is(err, ErrFileTooLarge):
		return FailureTooLarge, fmt.Sprintf("文件超过大小限制（当前上限 %s）", ByteLimitText(maxBytes))
	case errors.Is(err, ErrImageDimensions):
		return FailureDimensions, fmt.Sprintf("图片像素过多：%s，请先缩小尺寸再上传", DimensionLimitText())
	case errors.Is(err, ErrUnsupportedFormat):
		return FailureFormat, "不支持的图片格式，或文件内容与扩展名不一致（仅支持 JPEG、PNG、GIF、WebP）"
	case errors.Is(err, ErrDecodeFailed):
		return FailureDecode, "图片解码失败，文件可能已损坏"
	case errors.Is(err, context.DeadlineExceeded), errors.Is(err, context.Canceled):
		return FailureTimeout, "上传超时，请重试"
	}
	return "", ""
}
