package routes

import (
	"embed"
	"io/fs"
	"log"
	"net/http"
	"time"

	"oneimg/backend/config"
	"oneimg/backend/controllers"
	"oneimg/backend/middlewares"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

// 设置路由
func SetupRoutes(frontendFS embed.FS) *gin.Engine {
	cfg := config.App

	gin.SetMode(gin.ReleaseMode)
	r := gin.New()

	// 基础中间件
	r.Use(gin.Logger())
	r.Use(gin.Recovery())
	r.Use(middlewares.ConfigMiddleware(cfg))
	r.Use(middlewares.SessionMiddleware(cfg))

	// 跨域配置
	r.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"*"},
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"*"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))

	distFS, err := fs.Sub(frontendFS, "frontend/dist")
	if err != nil {
		panic("加载前端文件失败：" + err.Error())
	}
	assetsFS, _ := fs.Sub(distFS, "assets")
	r.StaticFS("/assets", http.FS(assetsFS))

	// 静态资源
	r.GET("/uploads/*path", controllers.ImageProxy)
	r.StaticFile("/favicon.ico", "./frontend/dist/favicon.ico")

	// API路由分组
	api := r.Group("/api")
	{
		// 公开接口（无需认证）
		api.POST("/login", controllers.Login)
		api.POST("/logout", controllers.Logout)
		api.GET("/logout", controllers.Logout)
		// 返回登录设置
		api.GET("/settings/login", controllers.GetLoginSettings)
		// 返回SEO设置
		api.GET("/settings/seo", controllers.GetSEOSettings)
		// 随机图片
		api.GET("/images/random", controllers.GetRandomImages)

		// 需要认证的接口分组（应用AuthMiddleware）
		auth := api.Group("")
		auth.Use(middlewares.AuthMiddleware())
		{
			// 用户信息接口
			auth.GET("/user/status", controllers.CheckLoginStatus)

			// 获取上传配置
			auth.GET("/uploadConfig", controllers.GetUploadConfig)

			// 标签管理接口
			auth.GET("/tags", controllers.GetTags)
			auth.DELETE("/tags/:id", controllers.DeleteTag)

			// 存储桶列表
			auth.GET("/buckets/list", controllers.GetBucketsList)

			// 统计数据
			auth.GET("/stats/dashboard", controllers.GetDashboardStats)
			auth.GET("/stats/images", controllers.GetImageStats)

			// 图片相关接口
			auth.POST("/upload", controllers.UploadImage)
			auth.POST("/upload/images", controllers.UploadImages)
			auth.DELETE("/images/:id", controllers.DeleteImage)
			auth.GET("/images", controllers.GetImageList)
			auth.GET("/images/:id", controllers.GetImageDetail)
			auth.POST("/images/tag", controllers.AddImageTag)
			auth.DELETE("/images/tag", controllers.DeleteImageTag)
			auth.DELETE("/images/tags", controllers.DeleteImageTags) // 批量删除图片标签
			auth.POST("/images/tags", controllers.AddImageTags)

			// 需要管理员权限
			auth.Use(middlewares.AdminOnlyMiddleware())
			{
				// 新增标签
				auth.POST("/tags", controllers.AddTag)

				// 存储管理接口
				auth.GET("/buckets", controllers.GetBuckets)
				auth.POST("/buckets", controllers.AddBuckets)
				auth.POST("/buckets/update/:id", controllers.UpdateBuckets)
				auth.DELETE("/buckets/:id", controllers.DeleteBuckets)

				// 账户管理接口
				auth.POST("/account/change", controllers.ChangeAccountInfo)
				auth.POST("/sessions/clear", controllers.ClearAllSessions)

				// 系统设置接口
				auth.Any("/settings/get", controllers.GetSettings)
				auth.POST("/settings/update", controllers.UpdateSettings)
			}
		}
	}

	// 前端SPA路由支持
	r.NoRoute(func(c *gin.Context) {
		// API路径返回404
		if len(c.Request.URL.Path) > 4 && c.Request.URL.Path[:4] == "/api" {
			c.JSON(http.StatusNotFound, gin.H{"code": 404, "msg": "API Not Found"})
			return
		}

		if files, err := fs.ReadDir(distFS, "."); err == nil {
			var fileNames []string
			for _, f := range files {
				fileNames = append(fileNames, f.Name())
			}
			log.Printf("读取distFS文件列表成功：%v", fileNames)
		} else {
			log.Printf("读取distFS文件列表失败：%s", err)
		}
		indexContent, err := fs.ReadFile(distFS, "index.html")
		if err != nil {
			c.String(http.StatusInternalServerError, "加载前端页面失败：%s", err)
			return
		}

		// 返回HTML内容
		c.Header("Content-Type", "text/html; charset=utf-8")
		c.String(http.StatusOK, string(indexContent))
	})

	return r
}
