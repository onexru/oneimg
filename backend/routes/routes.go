package routes

import (
	"embed"
	"io/fs"
	"net/http"
	"strings"
	"time"

	"oneimg/backend/config"
	"oneimg/backend/controllers"
	"oneimg/backend/middlewares"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

func SetupRoutes(frontendFS embed.FS) *gin.Engine {
	cfg := config.App

	gin.SetMode(gin.ReleaseMode)
	r := gin.New()

	r.Use(gin.LoggerWithConfig(gin.LoggerConfig{SkipPaths: []string{
		"/api/auth/oidc/callback",
		"/api/auth/cas/callback",
	}}))
	r.Use(gin.Recovery())
	r.Use(middlewares.ConfigMiddleware(cfg))
	r.Use(middlewares.SessionMiddleware(cfg))

	r.Use(cors.New(cors.Config{
		AllowOriginFunc: func(origin string) bool {
			if strings.TrimSpace(origin) == "" {
				return true
			}
			appURL := strings.TrimSpace(cfg.AppURL)
			if appURL != "" && strings.EqualFold(origin, appURL) {
				return true
			}
			return strings.HasPrefix(origin, "http://localhost:") || strings.HasPrefix(origin, "http://127.0.0.1:")
		},
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Accept", "Authorization", "X-Requested-With"},
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
	r.StaticFile("/favicon.ico", "./frontend/dist/favicon.ico")

	api := r.Group("/api")
	{
		// 公开接口
		api.POST("/login", controllers.Login)
		api.POST("/register", controllers.Register)
		api.POST("/logout", controllers.Logout)
		api.GET("/logout", controllers.Logout)
		api.GET("/settings/login", controllers.GetLoginSettings)
		api.GET("/settings/seo", controllers.GetSEOSettings)
		api.GET("/images/random", controllers.GetRandomImages)
		api.GET("/auth/oidc/login", controllers.StartOIDCLogin)
		api.GET("/auth/oidc/callback", controllers.OIDCCallback)
		api.GET("/auth/cas/login", controllers.StartCASLogin)
		api.GET("/auth/cas/callback", controllers.CASCallback)

		auth := api.Group("")
		auth.Use(middlewares.AuthMiddleware())
		{
			auth.GET("/user/status", controllers.CheckLoginStatus)
			auth.GET("/uploadConfig", controllers.GetUploadConfig)

			// 统计数据 (普通用户也能看自己的面板)
			auth.GET("/stats/dashboard", controllers.GetDashboardStats)
			auth.GET("/stats/images", controllers.GetImageStats)

			// 标签查看
			auth.GET("/tags", controllers.GetTags)
			// 存储桶列表
			auth.GET("/buckets/list", controllers.GetBucketsList)

			// 图片相关操作
			auth.POST("/upload", controllers.UploadImage)
			auth.POST("/upload/images", controllers.UploadImages)
			auth.DELETE("/images/:id", controllers.DeleteImage)
			auth.GET("/images", controllers.GetImageList)
			auth.GET("/images/:id", controllers.GetImageDetail)
			auth.POST("/images/tag", controllers.AddImageTag)
			auth.DELETE("/images/tag", controllers.DeleteImageTag)
			auth.DELETE("/images/tags", controllers.DeleteImageTags)
			auth.POST("/images/tags", controllers.AddImageTags)
			auth.PUT("/images/access-source", controllers.BatchUpdateImageAccessSource)
			auth.PUT("/images/:id/access-source", controllers.UpdateImageAccessSource)
			auth.POST("/images/url", controllers.UploadImagesByURL)

			// --- Tag 管理 ---
			auth.POST("/tags", middlewares.RequirePermission("tag:create"), controllers.AddTag)
			auth.PUT("/tags/:id", middlewares.RequirePermission("tag:update"), controllers.UpdateTag)
			auth.DELETE("/tags/:id", middlewares.RequirePermission("tag:delete"), controllers.DeleteTag)

			// --- 存储管理 ---
			auth.GET("/buckets", controllers.GetBuckets)
			auth.POST("/buckets", middlewares.RequirePermission("storage:create"), controllers.AddBuckets)
			auth.POST("/buckets/test", controllers.TestBucketConnection)
			auth.POST("/buckets/update/:id", middlewares.RequirePermission("storage:update"), controllers.UpdateBuckets)
			auth.PUT("/buckets/:id/enabled", middlewares.RequirePermission("storage:update"), controllers.UpdateBucketEnabled)
			auth.DELETE("/buckets/:id", middlewares.RequirePermission("storage:delete"), controllers.DeleteBuckets)

			// --- 账户管理 (修改自己的密码/信息通常不需要特殊权限，如果是修改全局则需) ---
			auth.POST("/account/change", controllers.ChangeAccountInfo)
			auth.POST("/sessions/clear", middlewares.RequirePermission("setting:security"), controllers.ClearAllSessions)

			// --- 用户管理 ---
			auth.GET("/users", controllers.GetUsers)
			auth.POST("/users/Add", middlewares.RequirePermission("user:create"), controllers.CreateUser)
			auth.DELETE("/users/:id", middlewares.RequirePermission("user:delete"), controllers.DeleteUser)
			auth.POST("/users/updateRole", middlewares.RequirePermission("user:role:update"), controllers.UpdateUserRole)
			auth.POST("/users/resetPassword/:id", middlewares.RequirePermission("user:password:reset"), controllers.ResetPassword)
			auth.POST("/users/updatePermission/:id", middlewares.RequirePermission("user:permission:update"), controllers.UpdateUserPermission)

			// --- 系统设置 ---
			auth.Any("/settings/get", controllers.GetSettings)
			auth.POST("/settings/update", controllers.UpdateSettings)
		}
	}

	// 前端 SPA 路由
	r.NoRoute(func(c *gin.Context) {
		if strings.HasPrefix(c.Request.URL.Path, "/api") {
			c.JSON(http.StatusNotFound, gin.H{"code": 404, "msg": "API Not Found"})
			return
		}
		if controllers.ImageProxy(c) {
			return
		}
		indexContent, err := fs.ReadFile(distFS, "index.html")
		if err != nil {
			c.String(http.StatusInternalServerError, "加载前端页面失败：%s", err)
			return
		}
		c.Header("Content-Type", "text/html; charset=utf-8")
		c.String(http.StatusOK, string(indexContent))
	})

	return r
}
