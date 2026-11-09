package main

import (
	"context"
	"embed"
	"errors"
	"log"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"oneimg/backend/app"
	"oneimg/backend/middlewares"
	"oneimg/backend/routes"
	"oneimg/backend/utils/watermark"
)

//go:embed frontend/dist/**
var fs embed.FS

//go:embed frontend/src/assets/fonts/**
var fontFs embed.FS

func main() {
	system := app.Init()
	r := routes.SetupRoutes(fs)
	watermark.Init(fontFs)
	// 预热每响应必经的 CSP：不让首批请求落到「读不到设置」的兜底分支，
	// 也不让第一个请求承担一次数据库往返。有界等待，数据库慢也不拖住启动。
	middlewares.WarmCSP()

	port := system.Config.Port
	log.Printf("应用初始化完成，监听 :%s", port)
	server := &http.Server{
		Addr:              ":" + port,
		Handler:           r,
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       90 * time.Second,
		MaxHeaderBytes:    1 << 20,
	}
	// No global write/read body timeout: large uploads and external storage
	// streams use request-scoped limits, not an arbitrary response deadline.
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			log.Printf("HTTP graceful shutdown failed: %v", err)
		}
	}()
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatal("服务启动失败:", err)
	}
}
