package routes

import (
	"io/fs"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	"oneimg/backend/config"
	"oneimg/backend/middlewares"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	authctrl "oneimg/backend/controllers/auth"
	usersctrl "oneimg/backend/controllers/users"
	imagesctrl "oneimg/backend/controllers/images"
	storagectrl "oneimg/backend/controllers/storage"
	systemctrl "oneimg/backend/controllers/system"
)

// noDirListingFS 包一层文件系统：把「目录请求」变成不存在。
//
// gin 的 StaticFS 最终用 http.FileServer，而 http.FileServer 在目录下没有
// index.html 时会渲染文件列表。构建产物目录（/assets、/cap）本来只应被
// 按确切文件名引用，列表页只会暴露产物名，没有正面作用。
// 返回 fs.ErrNotExist 让 http.FileServer 走标准 404，行为与「文件不存在」一致。
type noDirListingFS struct{ fs http.FileSystem }

func (n noDirListingFS) Open(name string) (http.File, error) {
	f, err := n.fs.Open(name)
	if err != nil {
		return nil, err
	}
	info, err := f.Stat()
	if err != nil {
		f.Close()
		return nil, err
	}
	if info.IsDir() {
		f.Close()
		return nil, fs.ErrNotExist
	}
	return f, nil
}

// isLocalhostURL 判断给定 URL 是否指向本机（localhost / 127.0.0.1 / ::1）。// 仅用于 CORS 本机源放行决策：服务本身部署在本机时才允许本机任意端口源。
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
	// 只采信显式配置的反代网络。空列表关掉 Gin 默认的「信任所有代理」——
	// 那等于让任何人用 X-Forwarded-For 伪造客户端 IP、绕开所有按 IP 的限流。
	//
	// 解析与校验在 config.parseTrustedProxies：格式错误的条目直接拒绝启动，
	// 而不是静默丢弃。静默丢弃的后果很隐蔽——应用照常起来，但所有访客共用
	// 反代的 IP，30 次/分钟的全站预算被共享，一个访客刷满全站登录 429。
	if err := r.SetTrustedProxies(config.App.TrustedProxies); err != nil {
		log.Fatalf("无效 TRUSTED_PROXIES: %v", err)
	}
	// 用共享实例：控制器在登录成功/失败时需要回写同一个限流器的账号预算。
	r.Use(middlewares.DefaultAuthLimiter().Middleware(), middlewares.AuthBodyLimitMiddleware(64<<10), middlewares.SameOriginWrites(config.App))
	// 演示模式闸门：DEMO_MODE=1 时全站只读（禁止上传与一切写操作），
	// 必须挂在所有业务路由之前——普通部署（DEMO_MODE 空）零影响。
	r.Use(middlewares.DemoModeMiddleware())

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
	// http.FileServer 在目录没有 index.html 时会返回文件列表，暴露构建产物名。
	// 这些目录本来只该按确切文件名被引用，目录请求一律 404。
	r.StaticFS("/assets", noDirListingFS{http.FS(assetsFS)})
	// 用 embed FS 而不是磁盘相对路径：StaticFile 依赖进程工作目录，
	// 容器里 ./frontend/dist/favicon.ico 根本不存在（图标实际在 assets 下），
	// 结果是每次请求都 404。这里改成从 dist 读取，不存在就明确 404。
	r.GET("/favicon.ico", func(c *gin.Context) {
		content, err := fs.ReadFile(distFS, "favicon.ico")
		if err != nil {
			c.Status(http.StatusNotFound)
			return
		}
		c.Data(http.StatusOK, "image/x-icon", content)
	})
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
	r.StaticFS("/cap", noDirListingFS{http.FS(capFS)})

	api := r.Group("/api")
	{
		// 公开接口
		api.POST("/login", authctrl.Login)
		api.POST("/register", authctrl.Register)
		api.POST("/logout", middlewares.OptionalAuthMiddleware(), authctrl.Logout)
		api.GET("/settings/login", systemctrl.GetLoginSettings)
		api.GET("/settings/seo", systemctrl.GetSEOSettings)
		api.POST("/verify/cappow/challenge", authctrl.CapPowChallenge)
		api.POST("/verify/cappow/redeem", authctrl.CapPowRedeem)
		api.GET("/images/random", imagesctrl.GetRandomImages)
		api.GET("/auth/oidc/login", authctrl.StartOIDCLogin)
		api.GET("/auth/oidc/callback", authctrl.OIDCCallback)
		api.GET("/auth/cas/login", authctrl.StartCASLogin)
		api.GET("/auth/cas/callback", authctrl.CASCallback)

		// 需登录
		auth := api.Group("")
		auth.Use(middlewares.AuthMiddleware())
		{
			auth.GET("/user/status", usersctrl.CheckLoginStatus)
			auth.GET("/uploadConfig", imagesctrl.GetUploadConfig)

			// 系统版本与更新检查：查询 GitHub 放在服务端做，既能缓存避开
			// 匿名限流，也不把访客 IP 暴露给 GitHub。?check=1 才真的联网。
			auth.GET("/system/version", systemctrl.GetSystemVersion)

			auth.GET("/stats/dashboard", imagesctrl.GetDashboardStats)
			auth.GET("/stats/images", imagesctrl.GetImageStats)

			auth.GET("/folders", imagesctrl.GetFolders)
			auth.POST("/folders", imagesctrl.CreateFolder)
			auth.PUT("/folders/:id", imagesctrl.UpdateFolder)
			auth.DELETE("/folders/:id", imagesctrl.DeleteFolder)
			auth.PUT("/images/folder", imagesctrl.MoveImagesFolder)
			auth.GET("/tags", imagesctrl.GetTags)
			auth.GET("/buckets/list", storagectrl.GetBucketsList)

			// 图片
			auth.POST("/upload", imagesctrl.UploadImage)
			auth.POST("/upload/images", imagesctrl.UploadImages)
			direct := auth.Group("/uploads/direct", storagectrl.DirectUploadGuard())
			direct.POST("", storagectrl.CreateDirectUpload)
			direct.GET("", storagectrl.ListDirectUploads)
			direct.GET("/:id", storagectrl.GetDirectUpload)
			direct.POST("/:id/sign", storagectrl.SignDirectUpload)
			direct.POST("/:id/complete", storagectrl.CompleteDirectUpload)
			direct.POST("/:id/fallback", storagectrl.FallbackDirectUpload)
			direct.POST("/:id/retry", storagectrl.RetryDirectUpload)
			direct.DELETE("/:id", storagectrl.CancelDirectUpload)
			auth.DELETE("/images/:id", imagesctrl.DeleteImage)
			auth.GET("/images", imagesctrl.GetImageList)
			auth.GET("/admin/images", middlewares.AdminOnlyMiddleware(), imagesctrl.GetManagedImageList)
			auth.GET("/images/:id", imagesctrl.GetImageDetail)
			auth.POST("/images/tag", imagesctrl.AddImageTag)
			auth.DELETE("/images/tag", imagesctrl.DeleteImageTag)
			auth.DELETE("/images/tags", imagesctrl.DeleteImageTags)
			auth.POST("/images/tags", imagesctrl.AddImageTags)
			auth.PUT("/images/access-source", imagesctrl.BatchUpdateImageAccessSource)
			auth.PUT("/images/:id/access-source", imagesctrl.UpdateImageAccessSource)
			auth.POST("/images/url", imagesctrl.UploadImagesByURL)

			// 标签管理
			auth.POST("/tags", middlewares.RequirePermission("tag:create"), imagesctrl.AddTag)
			auth.PUT("/tags/:id", middlewares.RequirePermission("tag:update"), imagesctrl.UpdateTag)
			auth.DELETE("/tags/:id", middlewares.RequirePermission("tag:delete"), imagesctrl.DeleteTag)

			// 存储管理
			// GET /api/buckets 与 POST /api/buckets/test 均挂权限门：
			// 前者返回存储拓扑（endpoint/本地路径），后者驱动服务器主动
			// 出站连接（S3/FTP/WebDAV/TG API），无权限时构成内网探测面。
			auth.GET("/buckets", middlewares.RequireAnyPermission("storage:create", "storage:update", "storage:delete"), storagectrl.GetBuckets)
			auth.POST("/buckets", middlewares.RequirePermission("storage:create"), storagectrl.AddBuckets)
			auth.POST("/buckets/test", middlewares.RequireAnyPermission("storage:create", "storage:update"), storagectrl.TestBucketConnection)
			auth.POST("/buckets/update/:id", middlewares.RequirePermission("storage:update"), storagectrl.UpdateBuckets)
			auth.PUT("/buckets/:id/enabled", middlewares.RequirePermission("storage:update"), storagectrl.UpdateBucketEnabled)
			auth.DELETE("/buckets/:id", middlewares.RequirePermission("storage:delete"), storagectrl.DeleteBuckets)

			// 账户
			auth.DELETE("/account/sessions/:id", authctrl.RevokeAccountSession)
			auth.POST("/account/change", authctrl.ChangeAccountInfo)
			auth.GET("/account/sessions", authctrl.ListSessions)
			auth.GET("/account/login-history", authctrl.ListAccountLoginHistory)
			auth.POST("/account/sessions/revoke", authctrl.RevokeOwnSessions)
			auth.POST("/sessions/clear", middlewares.RequirePermission("setting:security"), authctrl.ClearAllSessions)

			// 用户管理
			auth.GET("/users", middlewares.AdminOnlyMiddleware(), usersctrl.GetUsers)
			auth.GET("/admin/access-policy", middlewares.AdminOnlyMiddleware(), usersctrl.GetAccessPolicy)
			auth.GET("/admin/storage-assignments", middlewares.AdminOnlyMiddleware(), usersctrl.GetStorageAssignments)
			auth.PUT("/admin/storage-assignments/:role", middlewares.AdminOnlyMiddleware(), usersctrl.UpdateStorageAssignment)
			auth.POST("/users/Add", middlewares.RequirePermission("user:create"), usersctrl.CreateUser)
			auth.DELETE("/users/:id", middlewares.RequirePermission("user:delete"), usersctrl.DeleteUser)
			auth.POST("/users/updateRole", middlewares.RequirePermission("user:role:update"), usersctrl.UpdateUserRole)
			auth.POST("/users/resetPassword/:id", middlewares.RequirePermission("user:password:reset"), usersctrl.ResetPassword)
			auth.POST("/users/updatePermission/:id", middlewares.RequirePermission("user:permission:update"), usersctrl.UpdateUserPermission)
			// 单个账号上传配额。不新增权限码：配额属于「系统级上传策略」，
			// 与 max_file_size 同类，由 setting:* 授权面管控；
			// 精确的授权判断在控制器内（超管 或 持任一 setting:* 权限码）。
			auth.POST("/users/quota/:id", middlewares.RequireSettingPermission(), usersctrl.UpdateUserQuota)

			// 系统设置
			auth.GET("/settings/get", middlewares.RequirePermission("setting:list"), systemctrl.GetSettings)
			auth.POST("/settings/get", middlewares.RequirePermission("setting:list"), systemctrl.GetSettings)
			auth.POST("/settings/regenerate", middlewares.RequirePermission("setting:api"), authctrl.RegenerateAPIToken)
			auth.POST("/settings/renew_session", authctrl.RenewSession)
			auth.POST("/settings/update", systemctrl.UpdateSettings)
			auth.GET("/settings/randomGraph", middlewares.RequirePermission("setting:api"), imagesctrl.GetRandomGraph)
			auth.POST("/settings/randomGraph", middlewares.RequirePermission("setting:api"), imagesctrl.SetRandomGraph)
		}
	}

	// SPA 回退：优先图片代理，否则返回 index.html
	r.NoRoute(func(c *gin.Context) {
		if strings.HasPrefix(c.Request.URL.Path, "/api") {
			c.JSON(http.StatusNotFound, gin.H{"code": 404, "msg": "API Not Found"})
			return
		}
		if imagesctrl.ImageProxy(c) {
			return
		}
		// A missing image must not masquerade as an HTML SPA success response.
		if strings.HasPrefix(c.Request.URL.Path, "/uploads/") {
			c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": "图片不存在"})
			return
		}
		// 静态前缀同样不该回落到 SPA 页面：/assets/、/cap/ 下的目录请求已被
		// noDirListingFS 判为不存在，这里直接给 404，避免「请求一个不存在的
		// 资源却返回一整个 index.html」这种既误导又浪费带宽的响应。
		if strings.HasPrefix(c.Request.URL.Path, "/assets/") || strings.HasPrefix(c.Request.URL.Path, "/cap/") {
			c.Status(http.StatusNotFound)
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
