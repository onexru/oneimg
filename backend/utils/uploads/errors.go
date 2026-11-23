package uploads

import "errors"

// ErrStorageWrite 标记「文件已经处理好，但写进存储驱动失败」。
// 控制器据此把「存储没写进去」和「图片本身有问题」分开报给用户：
// 前者该查存储配置或重试，后者该换图或缩图。
var ErrStorageWrite = errors.New("storage write failed")
