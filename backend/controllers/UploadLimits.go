package controllers

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"mime/multipart"
	"oneimg/backend/models"
	"oneimg/backend/utils/uploadpolicy"
)

var uploadRequestSlots = make(chan struct{}, 2)

// uploadSlotHoldLimit bounds how long a single request may hold a global
// upload slot. It is a fallback for transports where the stall-abort body
// reader cannot enforce an idle deadline: a slow-loris style multipart body
// would otherwise pin both slots indefinitely and starve every other upload.
const uploadSlotHoldLimit = 15 * time.Minute

func acquireUploadRequest(c *gin.Context) (func(), bool) {
	select {
	case uploadRequestSlots <- struct{}{}:
		var once sync.Once
		release := func() { once.Do(func() { <-uploadRequestSlots }) }
		time.AfterFunc(uploadSlotHoldLimit, release)
		return release, true
	default:
		c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{"code": 429, "message": "上传处理中，请稍后重试"})
		return nil, false
	}
}

// uploadStallIdle is the maximum gap between two body reads before the
// request is considered stalled and aborted.
const uploadStallIdle = 60 * time.Second

// stallAbortBody aborts the underlying connection when an upload body stops
// delivering data for the idle period. The server deliberately has no global
// ReadTimeout because large uploads are legitimate; this per-request guard
// closes the slow-loris gap without imposing a total transfer deadline.
// It uses http.ResponseController.SetReadDeadline (supported by gin's
// ResponseWriter via Unwrap); transports without deadline support simply
// keep the previous behavior, and uploadSlotHoldLimit still applies.
type stallAbortBody struct {
	rc    *http.ResponseController
	body  io.ReadCloser
	idle  time.Duration
	timer *time.Timer
}

func (b *stallAbortBody) Read(p []byte) (int, error) {
	if b.timer == nil {
		b.timer = time.AfterFunc(b.idle, func() {
			// A deadline in the past unblocks the pending body read with a
			// timeout error, which fails multipart parsing and releases the
			// upload slot.
			_ = b.rc.SetReadDeadline(time.Now().Add(-time.Second))
		})
	} else {
		b.timer.Reset(b.idle)
	}
	return b.body.Read(p)
}

func (b *stallAbortBody) Close() error {
	if b.timer != nil {
		b.timer.Stop()
	}
	return b.body.Close()
}

// wrapUploadBody applies the response-aware size cap and the stall abort to
// an upload request body. Call it before any multipart parsing.
func wrapUploadBody(c *gin.Context, maxBytes int64) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxBytes)
	c.Request.Body = &stallAbortBody{
		rc:   http.NewResponseController(c.Writer),
		body: c.Request.Body,
		idle: uploadStallIdle,
	}
}

// parseConfiguredUploadFiles replaces the legacy fixed-count parser at both
// controller entry points without changing storage uploader interfaces.
func parseConfiguredUploadFiles(c *gin.Context, s models.Settings) ([]*multipart.FileHeader, error) {
	if err := c.Request.ParseMultipartForm(1 << 20); err != nil {
		return nil, err
	}
	if c.Request.MultipartForm == nil {
		return nil, fmt.Errorf("请选择要上传的图片")
	}
	files := c.Request.MultipartForm.File["images[]"]
	if err := uploadpolicy.ValidateCount(s, len(files)); err != nil {
		return nil, err
	}
	return files, nil
}
func uploadParseErrorStatus(err error) int {
	var tooLarge *http.MaxBytesError
	if errors.As(err, &tooLarge) {
		return http.StatusRequestEntityTooLarge
	}
	return http.StatusBadRequest
}
func validateUploadTags(s models.Settings, tags []string) error {
	if len(tags) > 100 {
		return fmt.Errorf("每次最多选择100个标签")
	}
	for _, tag := range tags {
		if err := uploadpolicy.ValidateTag(s, tag); err != nil {
			return err
		}
	}
	return nil
}
