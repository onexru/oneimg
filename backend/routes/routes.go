package routes

import (
	"io/fs"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"oneimg/backend/config"
	"oneimg/backend/controllers"
	"oneimg/backend/middlewares"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

// isLocalhostURL 判断给定 URL 是否指向本机（localhost / 127.0.0.1 / ::1）。
// 仅用于 CORS 本机源放行决策：服务本身部署在本机时才允许本机任意端口源。
func isLocalhostURL(raw string) bool {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.Host == "" {
		return false
	}
	switch u.Hostname() {
	case "localhost", "127.0.0.1", "::1":
		return true
	}
	return false
}

// SetupRoutes 注册中间件、API 与 SPA 回退路由。
func SetupRoutes(frontendFS fs.FS) *gin.Engine {
	cfg := config.App

	gin.SetMode(gin.ReleaseMode)
	r := gin.New()

	r.Use(gin.LoggerWithConfig(gin.LoggerConfig{SkipPaths: []string{
		"/api/auth/oidc/callback",
		"/api/auth/cas/callback",
	}}))
	r.Use(middlewares.SecurityHeaders(config.App))
	r.HandleMethodNotAllowed = true
	r.NoMethod(middlewares.MethodNotAllowed)
	// Only explicit proxy networks may supply forwarded addresses. Empty disables
	// Gin's insecure default trust of every proxy/client-supplied X-Forwarded-For.
	trusted := []string{}
	for _, entry := range strings.Split(os.Getenv("TRUSTED_PROXIES"), ",") {
		if entry = strings.TrimSpace(entry); entry != "" {
			trusted = append(trusted, entry)
		}
	}
	if err := r.SetTrustedProxies(trusted); err != nil {
		log.Fatalf("无效 TRUSTED_PROXIES: %v", err)
	}
	r.Use(middlewares.NewAuthLimiter().Middleware(), middlewares.AuthBodyLimitMiddleware(64<<10), middlewares.SameOriginWrites(config.App))

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
			// 仅当服务本身以 localhost 形态部署（本地开发/默认配置）时才放行
			// 本机任意端口源；生产部署（APP_URL 为真实域名）不再接受
			// localhost 源，防止受害者本机其他端口的恶意页面借
			// SameSite=Lax 不区分端口的特性携带凭据读取已登录接口。
			if isLocalhostURL(appURL) {
				return strings.HasPrefix(origin, "http://localhost:") ||
					strings.HasPrefix(origin, "http://127.0.0.1:")
			}
			return false
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
	r.GET("/theme-init.js", func(c *gin.Context) {
		content, err := fs.ReadFile(distFS, "theme-init.js")
		if err != nil {
			c.AbortWithStatus(http.StatusNotFound)
			return
		}
		c.Data(http.StatusOK, "text/javascript; charset=utf-8", content)
	})

	// cap-pow 自托管验证组件（cap-widget JS + WASM）
	capFS, _ := fs.Sub(distFS, "cap")
	r.StaticFS("/cap", http.FS(capFS))

	api := r.Group("/api")
	{
		// 公开接口
		api.POST("/login", controllers.Login)
		api.POST("/register", controllers.Register)
		api.POST("/logout", middlewares.OptionalAuthMiddleware(), controllers.Logout)
		api.GET("/settings/login", controllers.GetLoginSettings)
		api.GET("/settings/seo", controllers.GetSEOSettings)
		api.POST("/verify/cappow/challenge", controllers.CapPowChallenge)
		api.POST("/verify/cappow/redeem", controllers.CapPowRedeem)
		api.GET("/images/random", controllers.GetRandomImages)
		api.GET("/auth/oidc/login", controllers.StartOIDCLogin)
		api.GET("/auth/oidc/callback", controllers.OIDCCallback)
		api.GET("/auth/cas/login", controllers.StartCASLogin)
		api.GET("/auth/cas/callback", controllers.CASCallback)

		// 需登录
		auth := api.Group("")
		auth.Use(middlewares.AuthMiddleware())
		{
			auth.GET("/user/status", controllers.CheckLoginStatus)
			auth.GET("/uploadConfig", controllers.GetUploadConfig)

			auth.GET("/stats/dashboard", controllers.GetDashboardStats)
			auth.GET("/stats/images", controllers.GetImageStats)

			auth.GET("/folders", controllers.GetFolders)
			auth.POST("/folders", controllers.CreateFolder)
			auth.PUT("/folders/:id", controllers.UpdateFolder)
			auth.DELETE("/folders/:id", controllers.DeleteFolder)
			auth.PUT("/images/folder", controllers.MoveImagesFolder)
			auth.GET("/tags", controllers.GetTags)
			auth.GET("/buckets/list", controllers.GetBucketsList)

			// 图片
			auth.POST("/upload", controllers.UploadImage)
			auth.POST("/upload/images", controllers.UploadImages)
			direct := auth.Group("/uploads/direct", controllers.DirectUploadGuard())
			direct.POST("", controllers.CreateDirectUpload)
			direct.GET("", controllers.ListDirectUploads)
			direct.GET("/:id", controllers.GetDirectUpload)
			direct.POST("/:id/sign", controllers.SignDirectUpload)
			direct.POST("/:id/complete", controllers.CompleteDirectUpload)
			direct.POST("/:id/fallback", controllers.FallbackDirectUpload)
			direct.POST("/:id/retry", controllers.RetryDirectUpload)
			direct.DELETE("/:id", controllers.CancelDirectUpload)
			auth.DELETE("/images/:id", controllers.DeleteImage)
			auth.GET("/images", controllers.GetImageList)
			auth.GET("/admin/images", middlewares.AdminOnlyMiddleware(), controllers.GetManagedImageList)
			auth.GET("/images/:id", controllers.GetImageDetail)
			auth.POST("/images/tag", controllers.AddImageTag)
			auth.DELETE("/images/tag", controllers.DeleteImageTag)
			auth.DELETE("/images/tags", controllers.DeleteImageTags)
			auth.POST("/images/tags", controllers.AddImageTags)
			auth.PUT("/images/access-source", controllers.BatchUpdateImageAccessSource)
			auth.PUT("/images/:id/access-source", controllers.UpdateImageAccessSource)
			auth.POST("/images/url", controllers.UploadImagesByURL)

			// 标签管理
			auth.POST("/tags", middlewares.RequirePermission("tag:create"), controllers.AddTag)
			auth.PUT("/tags/:id", middlewares.RequirePermission("tag:update"), controllers.UpdateTag)
			auth.DELETE("/tags/:id", middlewares.RequirePermission("tag:delete"), controllers.DeleteTag)

			// 存储管理
			// GET /api/buckets 与 POST /api/buckets/test 均挂权限门：
			// 前者返回存储拓扑（endpoint/本地路径），后者驱动服务器主动
			// 出站连接（S3/FTP/WebDAV/TG API），无权限时构成内网探测面。
			auth.GET("/buckets", middlewares.RequireAnyPermission("storage:create", "storage:update", "storage:delete"), controllers.GetBuckets)
			auth.POST("/buckets", middlewares.RequirePermission("storage:create"), controllers.AddBuckets)
			auth.POST("/buckets/test", middlewares.RequireAnyPermission("storage:create", "storage:update"), controllers.TestBucketConnection)
			auth.POST("/buckets/update/:id", middlewares.RequirePermission("storage:update"), controllers.UpdateBuckets)
			auth.PUT("/buckets/:id/enabled", middlewares.RequirePermission("storage:update"), controllers.UpdateBucketEnabled)
			auth.DELETE("/buckets/:id", middlewares.RequirePermission("storage:delete"), controllers.DeleteBuckets)

			// 账户
			auth.DELETE("/account/sessions/:id", controllers.RevokeAccountSession)
			auth.POST("/account/change", controllers.ChangeAccountInfo)
			auth.GET("/account/sessions", controllers.ListSessions)
			auth.GET("/account/login-history", controllers.ListAccountLoginHistory)
			auth.POST("/account/sessions/revoke", controllers.RevokeOwnSessions)
			auth.POST("/sessions/clear", middlewares.RequirePermission("setting:security"), controllers.ClearAllSessions)

			// 用户管理
			auth.GET("/users", middlewares.AdminOnlyMiddleware(), controllers.GetUsers)
			auth.GET("/admin/access-policy", middlewares.AdminOnlyMiddleware(), controllers.GetAccessPolicy)
			auth.GET("/admin/storage-assignments", middlewares.AdminOnlyMiddleware(), controllers.GetStorageAssignments)
			auth.PUT("/admin/storage-assignments/:role", middlewares.AdminOnlyMiddleware(), controllers.UpdateStorageAssignment)
			auth.POST("/users/Add", middlewares.RequirePermission("user:create"), controllers.CreateUser)
			auth.DELETE("/users/:id", middlewares.RequirePermission("user:delete"), controllers.DeleteUser)
			auth.POST("/users/updateRole", middlewares.RequirePermission("user:role:update"), controllers.UpdateUserRole)
			auth.POST("/users/resetPassword/:id", middlewares.RequirePermission("user:password:reset"), controllers.ResetPassword)
			auth.POST("/users/updatePermission/:id", middlewares.RequirePermission("user:permission:update"), controllers.UpdateUserPermission)

			// 系统设置
			auth.GET("/settings/get", middlewares.RequirePermission("setting:list"), controllers.GetSettings)
			auth.POST("/settings/get", middlewares.RequirePermission("setting:list"), controllers.GetSettings)
			auth.POST("/settings/regenerate", middlewares.RequirePermission("setting:api"), controllers.RegenerateAPIToken)
			auth.POST("/settings/renew_session", controllers.RenewSession)
			auth.POST("/settings/update", controllers.UpdateSettings)
			auth.GET("/settings/randomGraph", middlewares.RequirePermission("setting:api"), controllers.GetRandomGraph)
			auth.POST("/settings/randomGraph", middlewares.RequirePermission("setting:api"), controllers.SetRandomGraph)
		}
	}

	// SPA 回退：优先图片代理，否则返回 index.html
	r.NoRoute(func(c *gin.Context) {
		if strings.HasPrefix(c.Request.URL.Path, "/api") {
			c.JSON(http.StatusNotFound, gin.H{"code": 404, "msg": "API Not Found"})
			return
		}
		if controllers.ImageProxy(c) {
			return
		}
		// A missing image must not masquerade as an HTML SPA success response.
		if strings.HasPrefix(c.Request.URL.Path, "/uploads/") {
			c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": "图片不存在"})
			return
		}
		if c.Request.Method != http.MethodGet && c.Request.Method != http.MethodHead {
			c.Header("Allow", "GET, HEAD")
			middlewares.MethodNotAllowed(c)
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
