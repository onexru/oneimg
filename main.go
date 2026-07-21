package main

import (
	"embed"
	"log"

	"oneimg/backend/app"
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

	port := system.Config.Port
	log.Printf("应用初始化完成，监听 :%s", port)
	if err := r.Run(":" + port); err != nil {
		log.Fatal("服务启动失败:", err)
	}
}
