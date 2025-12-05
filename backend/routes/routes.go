package routes

import (
	"net/http"
	"time"

	"oneimg/backend/config"
	"oneimg/backend/controllers"
	"oneimg/backend/middlewares"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

// 设置路由
func SetupRoutes() *gin.Engine {
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

	// 静态资源
	r.Static("/static", "./static/frontend")
	r.GET("/uploads/*path", controllers.ImageProxy)
	r.Static("/assets", "./frontend/dist/assets")
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

		// 需要认证的接口分组（应用AuthMiddleware）
		auth := api.Group("")
		auth.Use(middlewares.AuthMiddleware())
		{
			// 用户信息接口
			auth.GET("/user/status", controllers.CheckLoginStatus)

			// 统计数据
			auth.GET("/stats/dashboard", controllers.GetDashboardStats)
			auth.GET("/stats/images", controllers.GetImageStats)

			// 图片相关接口
			auth.POST("/upload", controllers.UploadImage)
			auth.POST("/upload/images", controllers.UploadImages)
			auth.DELETE("/images/:id", controllers.DeleteImage)
			auth.GET("/images", controllers.GetImageList)
			auth.GET("/images/:id", controllers.GetImageDetail)

			// 需要管理员权限
			auth.Use(middlewares.AdminOnlyMiddleware())
			{
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
		if len(c.Request.URL.Path) > 4 && c.Request.URL.Path[:4] == "/api" {
			c.JSON(http.StatusNotFound, gin.H{"code": 404, "msg": "API Not Found"})
			return
		}
		c.File("./frontend/dist/index.html")
	})

	return r
}
