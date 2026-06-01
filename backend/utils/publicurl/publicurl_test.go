package publicurl

import (
	"testing"

	"oneimg/backend/models"
)

func TestBuildForStorageOnlyRewritesDefaultSupportedBucket(t *testing.T) {
	setting := models.Settings{
		DefaultStorage:    2,
		PublicImageDomain: "img.example.com",
	}

	tests := []struct {
		name    string
		storage string
		bucket  int
		want    string
	}{
		{
			name:    "default supported bucket",
			storage: "s3",
			bucket:  2,
			want:    "https://img.example.com/uploads/a.webp",
		},
		{
			name:    "same storage different bucket",
			storage: "s3",
			bucket:  8,
			want:    "/uploads/a.webp",
		},
		{
			name:    "unsupported storage default bucket",
			storage: "default",
			bucket:  2,
			want:    "/uploads/a.webp",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := BuildForStorage(setting, tt.storage, tt.bucket, "/uploads/a.webp")
			if got != tt.want {
				t.Fatalf("BuildForStorage() = %q, want %q", got, tt.want)
			}
		})
	}
}
