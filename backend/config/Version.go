package config

// Version 是当前运行的版本号。
//
// 默认值在源码里写死一个可读的 "dev"，正式发布时由构建参数覆盖：
//
//	go build -ldflags "-X oneimg/backend/config.Version=v3.5.8"
//
// 这样不必为了改版本号去动源码，也能保证二进制里记录的版本与实际发布的
// tag 一致（镜像构建时由 Dockerfile 的 ARG 传入）。
var Version = "dev"
