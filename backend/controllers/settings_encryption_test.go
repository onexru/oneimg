package controllers

import (
	"strings"
	"testing"

	"oneimg/backend/database"
	"oneimg/backend/models"
)

func TestEncryptedStorageAndDirectDomainAreMutuallyExclusive(t *testing.T) {
	t.Run("encryption can be enabled with a persistent key and proxy URLs", func(t *testing.T) {
		initExternalAuthTestDB(t)
		if err := database.GetDB().DB.Create(&models.Settings{
			ID:             1,
			DefaultStorage: 1,
		}).Error; err != nil {
			t.Fatalf("create settings: %v", err)
		}
		if err := validateSettingData("encrypted_storage", true); err != nil {
			t.Fatalf("enable encrypted storage: %v", err)
		}
	})

	t.Run("cannot enable encryption while a direct domain is configured", func(t *testing.T) {
		initExternalAuthTestDB(t)
		if err := database.GetDB().DB.Create(&models.Settings{
			ID:                1,
			DefaultStorage:    1,
			PublicImageDomain: "https://img.example.com",
		}).Error; err != nil {
			t.Fatalf("create settings: %v", err)
		}
		err := validateSettingData("encrypted_storage", true)
		if err == nil || !strings.Contains(err.Error(), "清空图片直链域名") {
			t.Fatalf("validation error = %v, want direct-domain conflict", err)
		}
	})

	t.Run("cannot configure a direct domain while encryption is enabled", func(t *testing.T) {
		initExternalAuthTestDB(t)
		if err := database.GetDB().DB.Create(&models.Settings{
			ID:               1,
			DefaultStorage:   1,
			EncryptedStorage: true,
		}).Error; err != nil {
			t.Fatalf("create settings: %v", err)
		}
		err := validateSettingData("public_image_domain", "https://img.example.com")
		if err == nil || !strings.Contains(err.Error(), "不能配置直链域名") {
			t.Fatalf("validation error = %v, want encrypted-storage conflict", err)
		}
	})
}
