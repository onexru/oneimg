package main

/**
 * 初春图床v3
 * 重构后端，标准化接口，支持更多存储方式
 */
import (
	"embed"
	"log"

	"oneimg/backend/app"
	"oneimg/backend/routes"
)

// 导入静态资源
//
//go:embed frontend/dist/**
var fs embed.FS

func main() {
	system := app.Init()
	log.Println("应用初始化完成")
	r := routes.SetupRoutes(fs)

	port := system.Config.Port

	log.Printf("Server starting on :%s", port)
	if err := r.Run(":" + port); err != nil {
		log.Fatal("Failed to start server:", err)
	}
}
