package main

import (
	"log"
	"os"
	"path/filepath"

	_ "github.com/Deali-Axy/ebook-generator/docs" // 导入生成的Swagger文档
	"github.com/Deali-Axy/ebook-generator/internal/storage"
	"github.com/Deali-Axy/ebook-generator/internal/web/handlers"
	"github.com/Deali-Axy/ebook-generator/internal/web/middleware"
	"github.com/Deali-Axy/ebook-generator/internal/web/services"
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

	// 初始化服务
	initServices()

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
		api.POST("/upload", handlers.UploadFile)
		api.POST("/convert", handlers.ConvertBook)
		api.GET("/status/:taskId", handlers.GetTaskStatus)
		api.GET("/download/:fileId", handlers.DownloadFile)
		api.DELETE("/cleanup/:taskId", handlers.CleanupTask)
		api.GET("/events/:taskId", handlers.GetTaskEvents)
	}

	// Swagger文档
	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	// 健康检查
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

// initServices 初始化服务
func initServices() {
	// 获取工作目录
	workDir, err := os.Getwd()
	if err != nil {
		log.Fatal("获取工作目录失败:", err)
	}

	// 设置目录路径
	uploadDir := filepath.Join(workDir, "web", "uploads")
	outputDir := filepath.Join(workDir, "web", "outputs")

	// 创建存储服务
	storageService := storage.NewStorageService(uploadDir, outputDir, 50<<20) // 50MB限制

	// 创建转换器服务
	converterService := services.NewConverterService(outputDir)

	// 创建任务服务
	taskService := services.NewTaskService(storageService, converterService)

	// 初始化处理器
	handlers.InitServices(taskService, storageService, converterService)

	log.Println("服务初始化完成")
	log.Printf("上传目录: %s", uploadDir)
	log.Printf("输出目录: %s", outputDir)
}