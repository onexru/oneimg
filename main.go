package main

import (
	"log"

	"oneimg/backend/app"
	"oneimg/backend/routes"
)

func main() {
	// 初始化应用
	system := app.Init()
	log.Println("应用初始化完成")

	// 设置路由
	r := routes.SetupRoutes()

	port := system.Config.Port

	log.Printf("Server starting on :%s", port)
	if err := r.Run(":" + port); err != nil {
		log.Fatal("Failed to start server:", err)
	}
}
