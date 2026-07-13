package controllers

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"oneimg/backend/config"
	"oneimg/backend/utils/securestorage"
	"oneimg/backend/utils/watermark"

	"github.com/gin-gonic/gin"
)

func TestServeStoredImageReturnsPlaintextForEncryptedObject(t *testing.T) {
	gin.SetMode(gin.TestMode)
	previousConfig := config.App
	config.App = &config.Config{ConfigSecret: "proxy-encryption-test-key"}
	t.Cleanup(func() { config.App = previousConfig })

	want := []byte("plaintext image payload")
	stored, err := securestorage.Encrypt(want)
	if err != nil {
		t.Fatalf("encrypt object: %v", err)
	}
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodGet, "/uploads/test.webp", nil)

	if err := serveStoredImage(context, bytes.NewReader(stored), "image/webp", "s3", watermark.WatermarkConfig{}); err != nil {
		t.Fatalf("serve encrypted object: %v", err)
	}
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", recorder.Code)
	}
	if !bytes.Equal(recorder.Body.Bytes(), want) {
		t.Fatalf("browser payload = %q, want plaintext %q", recorder.Body.Bytes(), want)
	}
	if got := recorder.Header().Get("Content-Type"); got != "image/webp" {
		t.Fatalf("Content-Type = %q, want image/webp", got)
	}
	if got := recorder.Header().Get("X-Storage-Type"); got != "s3" {
		t.Fatalf("X-Storage-Type = %q, want s3", got)
	}
}

func TestServeStoredImageKeepsLegacyPlaintextCompatible(t *testing.T) {
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodGet, "/uploads/legacy.png", nil)
	want := []byte("legacy plaintext image")

	if err := serveStoredImage(context, bytes.NewReader(want), "image/png", "default", watermark.WatermarkConfig{}); err != nil {
		t.Fatalf("serve plaintext object: %v", err)
	}
	if !bytes.Equal(recorder.Body.Bytes(), want) {
		t.Fatalf("legacy payload = %q, want %q", recorder.Body.Bytes(), want)
	}
}
