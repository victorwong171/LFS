package app

import (
	"embed"
	"log"
	"net"
	"net/http"
	"strings"

	"lfs/config"
	"lfs/internal/handlers"
	"lfs/internal/interfaces"
	"lfs/internal/services"
	"lfs/internal/static"
	"lfs/internal/storage"
	"lfs/pkg/compression"

	"github.com/gin-gonic/gin"
	"golang.org/x/net/http2"
)

// App represents the core application structure.
// It uses dependency injection to assemble all components including services, handlers, and HTTP server.
type App struct {
	config         config.Config
	fileService    interfaces.FileService
	chatService    interfaces.ChatService
	metricsService interfaces.MetricsService
	staticService  interfaces.StaticFileService
	sysService     interfaces.SystemControlService
	fileHandlers   *handlers.FileHandlers
	chatHandlers   *handlers.ChatHandlers
	sysHandlers    *handlers.SystemHandlers
	router         *gin.Engine
	server         *http.Server
}

// NewApp creates and initializes a new application instance.
// cfg is the application configuration, staticFiles is the embedded static file system.
// Returns a configured App instance with all dependencies initialized via dependency injection.
func NewApp(cfg config.Config, staticFiles embed.FS) *App {
	// Initialize compressor
	compressor := compression.NewGzipCompressor()

	// Initialize static file service (subPath is "web/static" because embed path is "web/static/*")
	staticService := static.NewService(staticFiles, "web/static", compressor)

	// Initialize MD5 cache and calculator based on pluggable config
	var md5Cache interfaces.MD5Cache
	var md5Calculator interfaces.MD5Calculator

	if cfg.EnableMD5 {
		md5Cache = storage.NewMD5CacheAdapter()
		md5Calculator = storage.NewMD5CalculatorAdapter(cfg.StoragePath, md5Cache, true)
	} else {
		md5Cache = storage.NewNullMD5Cache()
		md5Calculator = storage.NewNullMD5Calculator()
	}

	// Initialize storage adapter
	storageAdapter := storage.NewStorageAdapter(cfg.StoragePath, md5Cache, cfg.EnableMD5)

	// Initialize service layer
	fileService := services.NewFileService(storageAdapter, md5Calculator, cfg.StoragePath)
	chatService := services.NewChatService()
	metricsService := services.NewMetricsService()

	// Initialize handlers
	fileHandlers := handlers.NewFileHandlers(fileService)
	chatHandlers := handlers.NewChatHandlers(chatService)

	// === 增量装配开始 ===
	hostController := storage.NewLocalHostController()
	sysService := services.NewSystemControlService(hostController)
	fileService.AddTransferActivityListener(sysService) // 注册监听器
	sysHandlers := handlers.NewSystemHandlers(sysService)
	// === 增量装配结束 ===

	// Create Gin engine
	router := gin.New()

	// Apply middleware
	setupMiddleware(router, staticService, compressor)

	// Register routes
	fileHandlers.Register(router)
	chatHandlers.Register(router)
	sysHandlers.Register(router) // 注册新路由
	setupStaticRoutes(router, staticService)
	setupMetricsRoute(router, metricsService)

	// Create HTTP server
	server := &http.Server{
		Addr:    ":8080",
		Handler: router,
	}

	// Configure HTTP/2
	http2.ConfigureServer(server, &http2.Server{
		MaxConcurrentStreams: 1000,
		MaxReadFrameSize:     1048576, // 1MB
	})

	return &App{
		config:         cfg,
		fileService:    fileService,
		chatService:    chatService,
		metricsService: metricsService,
		staticService:  staticService,
		sysService:     sysService,
		fileHandlers:   fileHandlers,
		chatHandlers:   chatHandlers,
		sysHandlers:    sysHandlers,
		router:         router,
		server:         server,
	}
}

// Run starts the HTTP server and begins listening for requests.
// It outputs the server listening address and access information before starting.
func (a *App) Run() error {
	addr := a.server.Addr
	if addr == "" {
		addr = ":http"
	}

	// Parse address
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		// If address format is incorrect, try to parse port
		if strings.HasPrefix(addr, ":") {
			port = addr[1:]
			host = ""
		} else {
			port = addr
			host = ""
		}
	}

	// Output listening information
	if host == "" || host == "0.0.0.0" {
		log.Printf("LFS Server listening on 0.0.0.0:%s (all interfaces)", port)
		log.Printf("Access the server at: http://localhost:%s", port)

		// Get local IP addresses
		if localIPs := getLocalIPs(); len(localIPs) > 0 {
			log.Print("Or access via local network:")
			for _, ip := range localIPs {
				log.Printf("  http://%s:%s", ip, port)
			}
		}
	} else {
		log.Printf("LFS Server listening on %s:%s", host, port)
		log.Printf("Access the server at: http://%s:%s", host, port)
	}

	log.Println("Static files embedded and cached successfully")
	log.Println("HTTP/2 and Gzip compression enabled")
	return a.server.ListenAndServe()
}

// getLocalIPs returns a list of all non-loopback IPv4 addresses.
// Used to display the server's network access addresses at startup.
func getLocalIPs() []string {
	var ips []string
	interfaces, err := net.Interfaces()
	if err != nil {
		return ips
	}

	for _, iface := range interfaces {
		// Skip loopback interfaces and disabled interfaces
		if iface.Flags&net.FlagLoopback != 0 || iface.Flags&net.FlagUp == 0 {
			continue
		}

		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}

		for _, addr := range addrs {
			var ip net.IP
			switch v := addr.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}

			// Only add IPv4 addresses, exclude loopback addresses
			if ip != nil && ip.To4() != nil && !ip.IsLoopback() {
				ips = append(ips, ip.String())
			}
		}
	}

	return ips
}

// setupMiddleware configures HTTP middleware including logging, recovery, CORS, and gzip compression.
func setupMiddleware(r *gin.Engine, staticService interfaces.StaticFileService, compressor interfaces.Compressor) {
	r.Use(gin.Logger())
	r.Use(gin.Recovery())
	r.Use(corsMiddleware())
	r.Use(gzipMiddleware(compressor, staticService))
}

// setupStaticRoutes configures static file routes.
func setupStaticRoutes(r *gin.Engine, staticService interfaces.StaticFileService) {
	r.GET("/static/*filepath", staticFileHandler(staticService))
	r.GET("/", homeHandler(staticService))
}

// setupMetricsRoute configures the metrics route.
func setupMetricsRoute(r *gin.Engine, metricsService interfaces.MetricsService) {
	r.GET("/metrics", metricsHandler(metricsService))
}

// corsMiddleware returns a CORS (Cross-Origin Resource Sharing) middleware.
func corsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		origin := c.Request.Header.Get("Origin")
		if origin != "" {
			c.Header("Access-Control-Allow-Origin", origin)
		} else {
			c.Header("Access-Control-Allow-Origin", "*")
		}

		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Origin, Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, Range")
		c.Header("Access-Control-Allow-Credentials", "true")
		c.Header("Access-Control-Max-Age", "86400")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	}
}

// gzipMiddleware returns a gzip compression middleware.
// Static file and home handlers already manage their own compression cleanly.
func gzipMiddleware(compressor interfaces.Compressor, staticService interfaces.StaticFileService) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()
	}
}

// staticFileHandler returns a handler for static file requests.
// Supports ETag cache validation and gzip compression.
func staticFileHandler(service interfaces.StaticFileService) gin.HandlerFunc {
	return func(c *gin.Context) {
		path := c.Param("filepath")
		path = path[1:] // Remove leading slash

		if path == "" {
			path = "index.html"
		}

		if !service.FileExists(path) {
			c.Status(http.StatusNotFound)
			return
		}

		etag := service.GetETag(path)
		if c.GetHeader("If-None-Match") == etag {
			c.Status(http.StatusNotModified)
			return
		}

		acceptEncoding := c.GetHeader("Accept-Encoding")
		var data []byte
		var contentType string
		var err error

		if acceptEncoding != "" {
			data, contentType, err = service.GetFileGzip(path)
			if err == nil && len(data) > 0 {
				c.Header("Content-Encoding", "gzip")
			}
		} else {
			data, contentType, err = service.GetFile(path)
		}

		if err != nil {
			c.Status(http.StatusNotFound)
			return
		}

		c.Header("Content-Type", contentType)
		c.Header("ETag", etag)
		c.Header("Cache-Control", "public, max-age=31536000")
		c.Data(http.StatusOK, "", data)
	}
}

// homeHandler returns a handler for home page requests.
func homeHandler(service interfaces.StaticFileService) gin.HandlerFunc {
	return func(c *gin.Context) {
		acceptEncoding := c.GetHeader("Accept-Encoding")
		var data []byte
		var contentType string
		var err error

		// Check if browser supports gzip, and fetch pre-compressed gzip content
		if acceptEncoding != "" && strings.Contains(acceptEncoding, "gzip") {
			data, contentType, err = service.GetFileGzip("index.html")
			if err == nil && len(data) > 0 {
				c.Header("Content-Encoding", "gzip")
				c.Header("Vary", "Accept-Encoding")
			}
		}

		if len(data) == 0 {
			data, contentType, err = service.GetFile("index.html")
		}

		if err != nil {
			c.Status(http.StatusNotFound)
			return
		}

		c.Header("Content-Type", contentType)
		c.Header("ETag", service.GetETag("index.html"))
		c.Header("Cache-Control", "public, max-age=3600")
		c.Data(http.StatusOK, "", data)
	}
}

// metricsHandler returns a handler for metrics requests.
func metricsHandler(service interfaces.MetricsService) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.JSON(http.StatusOK, service.GetMetrics())
	}
}
