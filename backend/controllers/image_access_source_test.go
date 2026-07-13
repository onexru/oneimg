package controllers

import (
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"oneimg/backend/database"
	"oneimg/backend/models"

	"github.com/gin-gonic/gin"
)

func TestSetImageAccessSourcePreservesCanonicalStorageAndFallsBackWhenDisabled(t *testing.T) {
	initExternalAuthTestDB(t)
	db := database.GetDB().DB
	local := models.Buckets{Id: 1, Name: "local", Type: "default", Config: map[string]any{}}
	remote := models.Buckets{Id: 2, Name: "remote", Type: "s3", Capacity: 1024, Config: map[string]any{}}
	if err := db.Create(&[]models.Buckets{local, remote}).Error; err != nil {
		t.Fatalf("create buckets: %v", err)
	}
	image := models.Image{
		Url:      "/uploads/original.webp",
		FileName: "original.webp",
		FileSize: 42,
		Storage:  "default",
		BucketId: 1,
		UserId:   10,
	}
	if err := db.Create(&image).Error; err != nil {
		t.Fatalf("create image: %v", err)
	}
	replicas := []models.ImageStorage{
		{ImageID: image.Id, BucketID: 1, Storage: "default", Status: models.ImageStorageStatusSuccess, URL: image.Url},
		{ImageID: image.Id, BucketID: 2, Storage: "s3", Status: models.ImageStorageStatusSuccess, URL: "/remote/original.webp"},
	}
	if err := db.Create(&replicas).Error; err != nil {
		t.Fatalf("create replicas: %v", err)
	}

	context := &gin.Context{}
	context.Set("user_role", models.RoleUser)
	context.Set("user_id", image.UserId)
	if _, _, err := setImageAccessSource(context, []int{image.Id}, remote.Id); err != nil {
		t.Fatalf("select remote access source: %v", err)
	}

	var stored models.Image
	if err := db.First(&stored, image.Id).Error; err != nil {
		t.Fatalf("reload image: %v", err)
	}
	if stored.AccessBucketId != remote.Id {
		t.Fatalf("access bucket = %d, want %d", stored.AccessBucketId, remote.Id)
	}
	if stored.BucketId != local.Id || stored.Storage != "default" || stored.Url != image.Url {
		t.Fatalf("canonical storage changed: bucket=%d storage=%q url=%q", stored.BucketId, stored.Storage, stored.Url)
	}

	resolved, err := resolveImageAccess(db, stored, false)
	if err != nil {
		t.Fatalf("resolve selected source: %v", err)
	}
	if resolved.bucket.Id != remote.Id || resolved.path != "/remote/original.webp" {
		t.Fatalf("resolved source = bucket %d path %q, want remote", resolved.bucket.Id, resolved.path)
	}

	if err := db.Model(&models.Buckets{}).Where("id = ?", remote.Id).Update("disabled", true).Error; err != nil {
		t.Fatalf("disable remote: %v", err)
	}
	resolved, err = resolveImageAccess(db, stored, false)
	if err != nil {
		t.Fatalf("resolve fallback: %v", err)
	}
	if resolved.bucket.Id != local.Id || resolved.storageType != "default" || resolved.path != image.Url {
		t.Fatalf("fallback source = bucket %d type %q path %q, want local", resolved.bucket.Id, resolved.storageType, resolved.path)
	}
	if stored.AccessBucketId != remote.Id {
		t.Fatalf("temporary disable should preserve selection, got %d", stored.AccessBucketId)
	}
}

func TestBatchSetImageAccessSourceIsAtomicWhenReplicaMissing(t *testing.T) {
	initExternalAuthTestDB(t)
	db := database.GetDB().DB
	if err := db.Create(&[]models.Buckets{
		{Id: 1, Name: "local", Type: "default", Config: map[string]any{}},
		{Id: 2, Name: "remote", Type: "webdav", Capacity: 1024, Config: map[string]any{}},
	}).Error; err != nil {
		t.Fatalf("create buckets: %v", err)
	}
	images := []models.Image{
		{Url: "/uploads/a.webp", FileName: "a.webp", FileSize: 1, Storage: "default", BucketId: 1, UserId: 10},
		{Url: "/uploads/b.webp", FileName: "b.webp", FileSize: 1, Storage: "default", BucketId: 1, UserId: 10},
	}
	if err := db.Create(&images).Error; err != nil {
		t.Fatalf("create images: %v", err)
	}
	if err := db.Create(&models.ImageStorage{
		ImageID: images[0].Id, BucketID: 2, Storage: "webdav", Status: models.ImageStorageStatusSuccess, URL: images[0].Url,
	}).Error; err != nil {
		t.Fatalf("create one remote replica: %v", err)
	}

	context := &gin.Context{}
	context.Set("user_role", models.RoleUser)
	context.Set("user_id", 10)
	_, _, err := setImageAccessSource(context, []int{images[0].Id, images[1].Id}, 2)
	var sourceErr *imageAccessSourceError
	if !errors.As(err, &sourceErr) || sourceErr.status != http.StatusConflict {
		t.Fatalf("batch error = %v, want conflict", err)
	}

	var changed int64
	if err := db.Model(&models.Image{}).Where("access_bucket_id != 0").Count(&changed).Error; err != nil {
		t.Fatalf("count changed images: %v", err)
	}
	if changed != 0 {
		t.Fatalf("batch update was not atomic; changed=%d", changed)
	}
}

func TestSetImageAccessSourceRejectsDisabledBucketAndOtherUsersImage(t *testing.T) {
	initExternalAuthTestDB(t)
	db := database.GetDB().DB
	bucket := models.Buckets{Id: 2, Name: "disabled", Type: "ftp", Disabled: true, Capacity: 1024, Config: map[string]any{}}
	image := models.Image{Url: "/uploads/a.webp", FileName: "a.webp", FileSize: 1, Storage: "default", BucketId: 1, UserId: 20}
	if err := db.Create(&bucket).Error; err != nil {
		t.Fatalf("create bucket: %v", err)
	}
	if err := db.Create(&image).Error; err != nil {
		t.Fatalf("create image: %v", err)
	}
	if err := db.Create(&models.ImageStorage{
		ImageID: image.Id, BucketID: bucket.Id, Storage: bucket.Type, Status: models.ImageStorageStatusSuccess, URL: image.Url,
	}).Error; err != nil {
		t.Fatalf("create replica: %v", err)
	}

	context := &gin.Context{}
	context.Set("user_role", models.RoleUser)
	context.Set("user_id", image.UserId)
	_, _, err := setImageAccessSource(context, []int{image.Id}, bucket.Id)
	var sourceErr *imageAccessSourceError
	if !errors.As(err, &sourceErr) || sourceErr.status != http.StatusConflict {
		t.Fatalf("disabled source error = %v, want conflict", err)
	}

	if err := db.Model(&bucket).Update("disabled", false).Error; err != nil {
		t.Fatalf("enable bucket: %v", err)
	}
	context.Set("user_id", 99)
	_, _, err = setImageAccessSource(context, []int{image.Id}, bucket.Id)
	if !errors.As(err, &sourceErr) || sourceErr.status != http.StatusForbidden {
		t.Fatalf("permission error = %v, want forbidden", err)
	}
}

func TestUpdateBucketEnabledIsReversibleAndProtectsLocalSource(t *testing.T) {
	initExternalAuthTestDB(t)
	db := database.GetDB().DB
	local := models.Buckets{Id: 1, Name: "local", Type: "default", Config: map[string]any{}}
	remote := models.Buckets{Id: 2, Name: "remote", Type: "s3", Capacity: 1024, Config: map[string]any{}}
	if err := db.Create(&[]models.Buckets{local, remote}).Error; err != nil {
		t.Fatalf("create buckets: %v", err)
	}

	disableRecorder, disableContext := newExternalAuthTestContext(http.MethodPut, "/api/buckets/2/enabled")
	disableContext.Params = gin.Params{{Key: "id", Value: "2"}}
	disableContext.Request.Body = io.NopCloser(strings.NewReader(`{"enabled":false}`))
	disableContext.Request.Header.Set("Content-Type", "application/json")
	UpdateBucketEnabled(disableContext)
	if disableRecorder.Code != http.StatusOK {
		t.Fatalf("disable status = %d body=%s", disableRecorder.Code, disableRecorder.Body.String())
	}
	if err := db.First(&remote, remote.Id).Error; err != nil {
		t.Fatalf("reload remote: %v", err)
	}
	if !remote.Disabled {
		t.Fatal("remote bucket was not disabled")
	}

	localRecorder, localContext := newExternalAuthTestContext(http.MethodPut, "/api/buckets/1/enabled")
	localContext.Params = gin.Params{{Key: "id", Value: "1"}}
	localContext.Request.Body = io.NopCloser(strings.NewReader(`{"enabled":false}`))
	localContext.Request.Header.Set("Content-Type", "application/json")
	UpdateBucketEnabled(localContext)
	if localRecorder.Code != http.StatusBadRequest {
		t.Fatalf("disable local status = %d body=%s", localRecorder.Code, localRecorder.Body.String())
	}
}

func TestExplicitAccessSourceKeepsProgramProxyURL(t *testing.T) {
	setting := models.Settings{
		DefaultStorage:    2,
		PublicImageDomain: "https://images.example.com",
	}
	image := models.Image{
		Url:            "/uploads/example.webp",
		Thumbnail:      "/uploads/thumbnails/example.webp",
		Storage:        "s3",
		BucketId:       2,
		AccessBucketId: 1,
	}

	rewriteImageURLs(setting, &image)
	if image.Url != "/uploads/example.webp" || image.Thumbnail != "/uploads/thumbnails/example.webp" {
		t.Fatalf("explicit access source was bypassed by direct domain: url=%q thumbnail=%q", image.Url, image.Thumbnail)
	}
}
