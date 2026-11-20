package uploadpolicy

import (
	"strings"
	"testing"

	"oneimg/backend/config"
	"oneimg/backend/models"
)

func TestMaxFilesBounds(t *testing.T) {
	cases := []struct {
		stored int
		want   int
	}{
		{0, config.DefaultMaxUploadFiles},
		{-1, config.DefaultMaxUploadFiles},
		{1, 1},
		{config.HardMaxUploadFiles, config.HardMaxUploadFiles},
		{config.HardMaxUploadFiles + 1, config.DefaultMaxUploadFiles},
	}
	for _, tc := range cases {
		if got := MaxFiles(models.Settings{MaxUploadFiles: tc.stored}); got != tc.want {
			t.Errorf("MaxFiles(%d) = %d，期望 %d", tc.stored, got, tc.want)
		}
	}
}

func TestValidateCount(t *testing.T) {
	s := models.Settings{MaxUploadFiles: 3}
	if err := ValidateCount(s, 0); err == nil {
		t.Fatal("0 个文件应被拒绝")
	}
	if err := ValidateCount(s, 3); err != nil {
		t.Fatalf("等于上限应通过：%v", err)
	}
	err := ValidateCount(s, 4)
	if err == nil {
		t.Fatal("超过上限应被拒绝")
	}
	if !strings.Contains(err.Error(), "最多只能上传3个文件") {
		t.Fatalf("提示语应带上上限：%v", err)
	}
}

func TestValidateTag(t *testing.T) {
	s := models.Settings{TagMaxLength: 3}
	if err := ValidateTag(s, "  "); err == nil {
		t.Fatal("空白标签应被拒绝")
	}
	if err := ValidateTag(s, "图片"); err != nil {
		t.Fatalf("合法标签不应报错：%v", err)
	}
	if err := ValidateTag(s, "四字标签"); err == nil {
		t.Fatal("超长标签应被拒绝")
	}
}

// TestRequestBytesGrowsWithFileCount 守住「整请求预算随文件数放大」这一行为：
// 固定成 32MiB 会让多图批量上传在整请求层面被误杀。
func TestRequestBytesGrowsWithFileCount(t *testing.T) {
	one := RequestBytes(models.Settings{MaxUploadFiles: 1})
	ten := RequestBytes(models.Settings{MaxUploadFiles: 10})
	if ten <= one {
		t.Fatalf("预算应随文件数增长：1 个=%d，10 个=%d", one, ten)
	}
	if ten < int64(10*(32<<20)) {
		t.Fatalf("10 个文件的预算过小：%d", ten)
	}
}
