package uploads

// 上传请求的文件字段名解析。
//
// 规范字段是 images[]：前端（frontend/src/utils/homeUploadActions.js）与
// 批量接口都用它，单张与批量共用同一套解析逻辑。
//
// 另外兼容 file / file[] / files[] —— 早期接口文档里写过 file（单张）与
// file[]（批量），照文档对接的客户端会一直收到 400「请选择要上传的图片」，
// 而这个提示并不会告诉他们「字段名错了」。这里做一次回退读取，
// 让两种写法都能用，同时把错误提示写清楚。

import (
	"mime/multipart"
)

// UploadFileFields 是按优先级尝试的字段名，第一个为规范字段。
var UploadFileFields = []string{"images[]", "images", "file[]", "files[]", "file"}

// PickUploadFiles 从已解析的 multipart 表单里按优先级取出文件列表。
func PickUploadFiles(form *multipart.Form) []*multipart.FileHeader {
	if form == nil {
		return nil
	}
	for _, name := range UploadFileFields {
		if files := form.File[name]; len(files) > 0 {
			return files
		}
	}
	return nil
}
