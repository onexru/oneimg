package images

import (
	"bytes"
	"encoding/binary"
	"errors"
	"image"
	"image/gif"
	"io"
	"strings"

	"github.com/chai2010/webp"
	"oneimg/backend/models"
)

const DirectMaxPixels = 16_000_000
const DirectMaxDimension = 8192

var ErrImageDimensions = errors.New("image dimensions exceed limit")

// ValidateDirectImage checks actual format and allocation bounds before decoding.
// SVG is deliberately unsupported: never trust a browser MIME for active content.
func ValidateDirectImage(data []byte, expected string) (image.Config, string, error) {
	expected = declaredImageMIME(expected)
	var cfg image.Config
	if int64(len(data)) > MaxUploadBytes { return cfg, "", ErrFileTooLarge }
	var format string
	var err error
	if len(data) >= 12 && string(data[:4]) == "RIFF" && string(data[8:12]) == "WEBP" {
		cfg, err = webp.DecodeConfig(bytes.NewReader(data))
		format = "webp"
	} else {
		cfg, format, err = image.DecodeConfig(bytes.NewReader(data))
	}
	mime := map[string]string{"png": "image/png", "jpeg": "image/jpeg", "gif": "image/gif", "webp": "image/webp"}[format]
	if err != nil || mime == "" || (expected != "" && expected != mime) {
		return cfg, "", ErrUnsupportedFormat
	}
	if cfg.Width <= 0 || cfg.Height <= 0 || cfg.Width > DirectMaxDimension || cfg.Height > DirectMaxDimension || int64(cfg.Width)*int64(cfg.Height) > DirectMaxPixels {
		return cfg, "", ErrImageDimensions
	}
	if format == "gif" {
		if err := validateDirectGIF(data, cfg); err != nil {
			return cfg, "", err
		}
	}
	return cfg, mime, nil
}

// Scan GIF framing without allocating every animation frame. The first frame is
// decoded below; all frame rectangles and total animation pixels are bounded.
func validateDirectGIF(data []byte, cfg image.Config) error {
	if len(data) < 13 {
		return ErrUnsupportedFormat
	}
	p := 13
	if data[10]&128 != 0 {
		p += 3 << ((data[10] & 7) + 1)
	}
	frames := 0
	var pixels int64
	blocks := func() bool {
		for p < len(data) {
			n := int(data[p])
			p++
			if n == 0 {
				return true
			}
			if n > len(data)-p {
				return false
			}
			p += n
		}
		return false
	}
	for p < len(data) {
		kind := data[p]
		p++
		switch kind {
		case 0x3b:
			if frames == 0 {
				return ErrUnsupportedFormat
			}
			return nil
		case 0x21:
			if p >= len(data) {
				return ErrUnsupportedFormat
			}
			p++
			if !blocks() {
				return ErrUnsupportedFormat
			}
		case 0x2c:
			if len(data)-p < 9 {
				return ErrUnsupportedFormat
			}
			x := int(binary.LittleEndian.Uint16(data[p:]))
			y := int(binary.LittleEndian.Uint16(data[p+2:]))
			w := int(binary.LittleEndian.Uint16(data[p+4:]))
			h := int(binary.LittleEndian.Uint16(data[p+6:]))
			packed := data[p+8]
			p += 9
			frames++
			pixels += int64(w) * int64(h)
			if w <= 0 || h <= 0 || x+w > cfg.Width || y+h > cfg.Height || frames > 200 || pixels > 64_000_000 {
				return ErrImageDimensions
			}
			if packed&128 != 0 {
				p += 3 << ((packed & 7) + 1)
			}
			if p >= len(data) {
				return ErrUnsupportedFormat
			}
			p++
			if !blocks() {
				return ErrUnsupportedFormat
			}
		default:
			return ErrUnsupportedFormat
		}
	}
	return ErrUnsupportedFormat
}

func decodeDirectImage(data []byte, mime string) (image.Image, string, error) {
	if _, actual, err := ValidateDirectImage(data, mime); err != nil {
		return nil, "", err
	} else {
		mime = actual
	}
	if mime == "image/webp" {
		im, err := webp.Decode(bytes.NewReader(data))
		return im, "webp", err
	}
	if mime == "image/gif" {
		// DecodeAll validates every frame (not just the thumbnail's first frame).
		// validateDirectGIF bounds total indexed-pixel allocation before this call.
		animation, err := gif.DecodeAll(bytes.NewReader(data))
		if err != nil {
			return nil, "gif", err
		}
		return animation.Image[0], "gif", nil
	}
	return image.Decode(bytes.NewReader(data))
}

// ProcessDirectImage reuses the main-image pipeline, not the synchronous thumbnail
// pipeline. It also corrects the legacy compress-without-SaveWebp MIME mismatch.
func ProcessDirectImage(data []byte, mime string, setting models.Settings) (*ProcessedImage, error) {
	release := AcquireProcessing(); defer release()
	im, format, err := decodeDirectImage(data, mime)
	if err != nil {
		return nil, err
	}
	setting.Thumbnail = false
	svc := ImageService{}
	out, _, _, err := svc.processMainImage(data, im, format, mime, int64(len(data)), setting)
	if err != nil {
		return nil, err
	}
	cfg, actual, err := ValidateDirectImage(out, "")
	if err != nil {
		return nil, err
	}
	ext := map[string]string{"image/jpeg": ".jpg", "image/png": ".png", "image/gif": ".gif", "image/webp": ".webp"}[actual]
	return &ProcessedImage{CompressedBytes: out, Width: cfg.Width, Height: cfg.Height, MimeType: actual, OutputExt: ext, Format: strings.TrimPrefix(actual, "image/")}, nil
}

// GenerateDirectThumbnail always returns a real WebP thumbnail or an error;
// unlike the legacy helper it never silently publishes the original as a thumb.
func GenerateDirectThumbnail(data []byte) ([]byte, error) {
	release := AcquireProcessing(); defer release()
	im, _, err := decodeDirectImage(data, "")
	if err != nil {
		return nil, err
	}
	return (&ImageService{}).generateWebPThumbnail(im, ThumbnailMaxWidth, ThumbnailMaxHeight, ThumbnailQuality)
}

// ReadDirectLimited never allocates based on a remote Content-Length.
func ReadDirectLimited(r io.Reader, limit int64) ([]byte, error) {
	if limit <= 0 {
		return nil, ErrFileTooLarge
	}
	data, err := io.ReadAll(io.LimitReader(r, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > limit {
		return nil, ErrFileTooLarge
	}
	return data, nil
}
