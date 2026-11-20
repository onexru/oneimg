package uploads

import (
	"mime/multipart"
	"testing"
)

func TestPickUploadFiles(t *testing.T) {
	img := func() *multipart.FileHeader { return &multipart.FileHeader{Filename: "a.png"} }

	cases := []struct {
		name string
		form map[string][]*multipart.FileHeader
		want int
	}{
		{"规范字段 images[]", map[string][]*multipart.FileHeader{"images[]": {img(), img()}}, 2},
		{"兼容单张 file", map[string][]*multipart.FileHeader{"file": {img()}}, 1},
		{"兼容批量 file[]", map[string][]*multipart.FileHeader{"file[]": {img(), img()}}, 2},
		{"兼容 images", map[string][]*multipart.FileHeader{"images": {img()}}, 1},
		{"无文件", map[string][]*multipart.FileHeader{}, 0},
		{"只有普通表单字段", map[string][]*multipart.FileHeader{"tags": {}}, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			form := &multipart.Form{File: tc.form}
			if got := len(PickUploadFiles(form)); got != tc.want {
				t.Fatalf("PickUploadFiles() 取到 %d 个文件，期望 %d", got, tc.want)
			}
		})
	}

	if got := PickUploadFiles(nil); got != nil {
		t.Fatalf("nil 表单应返回 nil，实际 %v", got)
	}
}

// TestPickUploadFilesPrefersCanonicalField 保证优先级：同时出现 images[] 与 file 时，
// 以规范字段为准，避免一次请求混入两份文件导致数量校验误判。
func TestPickUploadFilesPrefersCanonicalField(t *testing.T) {
	form := &multipart.Form{File: map[string][]*multipart.FileHeader{
		"images[]": {{Filename: "canonical.png"}},
		"file":     {{Filename: "legacy.png"}},
	}}
	files := PickUploadFiles(form)
	if len(files) != 1 || files[0].Filename != "canonical.png" {
		t.Fatalf("应优先取 images[]，实际：%+v", files)
	}
}
