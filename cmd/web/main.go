package main

import (
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	_ "github.com/Deali-Axy/ebook-generator/api-docs" // 导入生成的Swagger文档
	"github.com/Deali-Axy/ebook-generator/internal/services"
	"github.com/Deali-Axy/ebook-generator/internal/storage"
	"github.com/Deali-Axy/ebook-generator/internal/web/handlers"
	"github.com/Deali-Axy/ebook-generator/internal/web/middleware"
	webServices "github.com/Deali-Axy/ebook-generator/internal/web/services"
	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

// @title Ebook Generator API
// @version 1.0
// @description 电子书转换服务API
// @termsOfService http://swagger.io/terms/

// @contact.name API Support
// @contact.url http://www.swagger.io/support
// @contact.email support@swagger.io

// @license.name Apache 2.0
// @license.url http://www.apache.org/licenses/LICENSE-2.0.html

// @host localhost:8080
// @BasePath /api

// main 启动Web服务
func main() {
	// 设置Gin模式
	if os.Getenv("GIN_MODE") == "" {
		gin.SetMode(gin.DebugMode)
	}

	// 初始化服务管理器
	serviceManager, err := initServiceManager()
	if err != nil {
		log.Fatal("初始化服务管理器失败:", err)
	}

	// 启动所有服务
	if err := serviceManager.Start(); err != nil {
		log.Fatal("启动服务失败:", err)
	}

	// 设置优雅关闭
	defer func() {
		if err := serviceManager.Stop(); err != nil {
			log.Printf("停止服务时出错: %v", err)
		}
	}()

	// 初始化Web服务相关组件
	initWebServices(serviceManager)

	// 创建Gin引擎
	r := gin.Default()

	// 添加中间件
	r.Use(middleware.CORS())
	r.Use(middleware.Logger())
	r.Use(middleware.Recovery())

	// 设置文件上传大小限制 (50MB)
	r.MaxMultipartMemory = 50 << 20

	// API路由组
	api := r.Group("/api")
	{
		// 基础转换功能
		api.POST("/upload", handlers.UploadFile)
		api.POST("/convert", handlers.ConvertBook)
		api.GET("/status/:taskId", handlers.GetTaskStatus)
		api.GET("/download/:fileId", handlers.DownloadFile)
		api.DELETE("/cleanup/:taskId", handlers.CleanupTask)
		api.GET("/events/:taskId", handlers.GetTaskEvents)

		// 用户认证相关路由
		auth := api.Group("/auth")
		{
			auth.POST("/register", handlers.Register)
			auth.POST("/login", handlers.Login)
			auth.GET("/profile", handlers.AuthMiddleware(), handlers.GetProfile)
			auth.PUT("/profile", handlers.AuthMiddleware(), handlers.UpdateProfile)
			auth.POST("/logout", handlers.AuthMiddleware(), handlers.Logout)
			auth.POST("/refresh", handlers.AuthMiddleware(), handlers.RefreshToken)
		}

		// 转换历史相关路由（需要认证）
		history := api.Group("/history")
		history.Use(handlers.AuthMiddleware())
		{
			history.GET("", handlers.GetHistories)
			history.GET("/stats", handlers.GetHistoryStats)
			history.DELETE("/:id", handlers.DeleteHistory)
		}

		// 转换预设相关路由（需要认证）
		presets := api.Group("/presets")
		presets.Use(handlers.AuthMiddleware())
		{
			presets.POST("", handlers.CreatePreset)
			presets.GET("", handlers.GetPresets)
			presets.PUT("/:id", handlers.UpdatePreset)
			presets.DELETE("/:id", handlers.DeletePreset)
		}

		// 批量转换相关路由（需要认证）
		batch := api.Group("/batch")
		batch.Use(handlers.AuthMiddleware())
		{
			batch.POST("/convert", handlers.BatchConvert)
		}
	}

	// 集成Swagger文档
	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	// 静态文件服务
	r.Static("/static", "./web/static")
	r.StaticFile("/", "./web/static/index.html")
	r.StaticFile("/demo", "./web/static/index.html")

	// 健康检查接口
	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	// 启动服务
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("服务启动在端口: %s", port)
	log.Printf("Swagger文档地址: http://localhost:%s/swagger/index.html", port)

	if err := r.Run(":" + port); err != nil {
		log.Fatal("启动服务失败:", err)
	}
}

// initServiceManager 初始化服务管理器
// 创建并配置服务管理器，加载配置文件
func initServiceManager() (*services.ServiceManager, error) {
	// 检查配置文件是否存在
	configPath := "config/services.json"
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		// 如果不存在，使用示例配置文件
		configPath = "config/services.example.json"
		log.Printf("使用示例配置文件: %s", configPath)
	}

	// 创建服务管理器
	serviceManager, err := services.NewServiceManager(configPath)
	if err != nil {
		return nil, err
	}

	return serviceManager, nil
}

// initWebServices 初始化Web服务相关组件
// 使用服务管理器中的服务来初始化Web处理器
func initWebServices(serviceManager *services.ServiceManager) {
	// 获取工作目录
	workDir, err := os.Getwd()
	if err != nil {
		log.Fatal("获取工作目录失败:", err)
	}

	// 设置目录路径
	uploadDir := filepath.Join(workDir, "web", "uploads")
	outputDir := filepath.Join(workDir, "web", "outputs")

	// 创建存储服务（用于Web处理器）
	storageService := storage.NewStorageService(uploadDir, outputDir, 50<<20) // 50MB限制

	// 创建转换器服务（用于Web处理器）
	converterService := webServices.NewConverterService(outputDir)

	// 创建任务服务（用于Web处理器）
	taskService := webServices.NewTaskService(storageService, converterService)

	// 初始化基础处理器
	handlers.InitServices(taskService, storageService, converterService)

	// 如果服务管理器中有数据库服务，初始化需要数据库的处理器
	if serviceManager.DB != nil {
		// 创建认证服务
		authService := webServices.NewAuthService(serviceManager.DB, "ebook-generator-secret", 24*time.Hour) // 24小时过期
		handlers.InitAuthService(authService)

		// 初始化历史服务
		if serviceManager.HistorySvc != nil {
			handlers.InitHistoryService(serviceManager.HistorySvc)
		} else {
			// 如果服务管理器中没有历史服务，手动创建一个
			historyService := webServices.NewHistoryService(serviceManager.DB)
			handlers.InitHistoryService(historyService)
		}

		log.Println("数据库相关服务初始化完成")
	} else {
		log.Println("警告: 数据库未初始化，认证和历史功能将不可用")
	}

	log.Println("Web服务初始化完成")
	log.Printf("上传目录: %s", uploadDir)
	log.Printf("输出目录: %s", outputDir)

	// 设置信号处理，用于优雅关闭
	go func() {
		c := make(chan os.Signal, 1)
		signal.Notify(c, os.Interrupt, syscall.SIGTERM)
		<-c
		log.Println("收到关闭信号，正在优雅关闭服务...")
		os.Exit(0)
	}()
}
