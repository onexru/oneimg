package controllers

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"oneimg/backend/database"
	"oneimg/backend/models"
	"oneimg/backend/utils/secureconfig"
)

func TestBuildBucketConnectionCandidateMergesStoredSecrets(t *testing.T) {
	initExternalAuthTestDB(t)
	rawConfig := map[string]any{
		"webdav_url":  "https://old.example.test/dav",
		"webdav_user": "stored-user",
		"webdav_pass": "stored-password",
	}
	encryptedConfig, err := secureconfig.EncryptBucketConfigValues(rawConfig)
	if err != nil {
		t.Fatalf("encrypt bucket config: %v", err)
	}
	bucket := models.Buckets{Name: "webdav-test", Type: "webdav", Capacity: 1024, Config: encryptedConfig}
	if err := database.GetDB().DB.Create(&bucket).Error; err != nil {
		t.Fatalf("create bucket: %v", err)
	}

	candidate, err := buildBucketConnectionCandidate(map[string]any{
		"id":          float64(bucket.Id),
		"type":        "webdav",
		"webdav_url":  "https://new.example.test/dav",
		"webdav_user": "",
		"webdav_pass": "",
	})
	if err != nil {
		t.Fatalf("buildBucketConnectionCandidate() error: %v", err)
	}
	if got := secureconfig.GetString(candidate.Config, "webdav_url"); got != "https://new.example.test/dav" {
		t.Fatalf("webdav_url = %q", got)
	}
	if got := secureconfig.GetString(candidate.Config, "webdav_user"); got != "stored-user" {
		t.Fatalf("stored webdav_user was not reused: %q", got)
	}
	if got := secureconfig.GetString(candidate.Config, "webdav_pass"); got != "stored-password" {
		t.Fatalf("stored webdav_pass was not reused: %q", got)
	}
}

func TestBuildBucketConnectionCandidateValidatesNewDraft(t *testing.T) {
	initExternalAuthTestDB(t)
	_, err := buildBucketConnectionCandidate(map[string]any{
		"type":        "s3",
		"s3_endpoint": "https://s3.example.test",
		"s3_bucket":   "images",
	})
	if err == nil || !strings.Contains(err.Error(), "s3_access_key") {
		t.Fatalf("missing draft secret error = %v", err)
	}

	if _, err := bucketTestPort("21.5"); err == nil {
		t.Fatal("fractional FTP port unexpectedly accepted")
	}
}

func TestLocalStorageConnectionTestCleansTemporaryFile(t *testing.T) {
	before, err := filepath.Glob(".oneimg-storage-test-*")
	if err != nil {
		t.Fatalf("glob before test: %v", err)
	}
	detail, err := testLocalStorage()
	if err != nil {
		t.Fatalf("testLocalStorage() error: %v", err)
	}
	if !strings.Contains(detail, "可读写") {
		t.Fatalf("testLocalStorage() detail = %q", detail)
	}
	after, err := filepath.Glob(".oneimg-storage-test-*")
	if err != nil {
		t.Fatalf("glob after test: %v", err)
	}
	if len(after) != len(before) {
		t.Fatalf("local connection test left temporary files: before=%v after=%v", before, after)
	}
}

func TestWebDAVStorageConnectionTestWritesAndDeletes(t *testing.T) {
	var putCalls atomic.Int32
	var deleteCalls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		username, password, ok := request.BasicAuth()
		if !ok || username != "dav-user" || password != "dav-password" {
			writer.WriteHeader(http.StatusUnauthorized)
			return
		}
		switch request.Method {
		case http.MethodPut:
			putCalls.Add(1)
			writer.WriteHeader(http.StatusCreated)
		case http.MethodDelete:
			deleteCalls.Add(1)
			writer.WriteHeader(http.StatusNoContent)
		default:
			writer.WriteHeader(http.StatusMethodNotAllowed)
		}
	}))
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	detail, err := testWebDAVStorage(ctx, models.Buckets{
		Type: "webdav",
		Config: map[string]any{
			"webdav_url":  server.URL + "/dav",
			"webdav_user": "dav-user",
			"webdav_pass": "dav-password",
		},
	})
	if err != nil {
		t.Fatalf("testWebDAVStorage() error: %v", err)
	}
	if putCalls.Load() != 1 || deleteCalls.Load() != 1 {
		t.Fatalf("WebDAV calls: PUT=%d DELETE=%d", putCalls.Load(), deleteCalls.Load())
	}
	if !strings.Contains(detail, "WebDAV") {
		t.Fatalf("testWebDAVStorage() detail = %q", detail)
	}
}

func TestSanitizeBucketTestErrorRedactsSecrets(t *testing.T) {
	message := sanitizeBucketTestError(errors.New("request with token-secret and password-secret failed"), map[string]any{
		"tg_bot_token": "token-secret",
		"webdav_pass":  "password-secret",
	})
	if strings.Contains(message, "token-secret") || strings.Contains(message, "password-secret") {
		t.Fatalf("sanitizeBucketTestError() leaked a secret: %q", message)
	}
}
