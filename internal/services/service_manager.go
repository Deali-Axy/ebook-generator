package services

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/ebook-generator/internal/cache"
	"github.com/ebook-generator/internal/cleanup"
	"github.com/ebook-generator/internal/config"
	"github.com/ebook-generator/internal/database"
	"github.com/ebook-generator/internal/download"
	"github.com/ebook-generator/internal/health"
	"github.com/ebook-generator/internal/loadbalancer"
	"github.com/ebook-generator/internal/logging"
	"github.com/ebook-generator/internal/monitoring"
	"github.com/ebook-generator/internal/upload"
	"github.com/ebook-generator/internal/validation"
	"github.com/ebook-generator/internal/web/middleware"
	"github.com/ebook-generator/internal/web/services"
	gorm "gorm.io/gorm"
)

// ServiceManager 服务管理器
type ServiceManager struct {
	// 核心服务
	DB           *gorm.DB
	ConfigMgr    *config.ConfigManager
	Logger       *logging.LoggerService
	HealthSvc    *health.HealthService
	MetricsSvc   *monitoring.MetricsService

	// 业务服务
	CacheSvc     *cache.CacheService
	CleanupSvc   *cleanup.CleanupService
	UploadSvc    *upload.StreamUploadService
	DownloadMgr  *download.DownloadManager
	Validator    *validation.FileValidator
	HistorySvc   *services.HistoryService

	// 中间件
	RateLimiter  *middleware.RateLimiter
	LoadBalancer *loadbalancer.LoadBalancer

	// 配置
	config       *ServiceConfig
	mutex        sync.RWMutex
	ctx          context.Context
	cancel       context.CancelFunc
	started      bool
}

// ServiceConfig 服务配置
type ServiceConfig struct {
	// 数据库配置
	Database DatabaseConfig `json:"database"`

	// 日志配置
	Logging logging.LogConfig `json:"logging"`

	// 健康检查配置
	Health health.HealthConfig `json:"health"`

	// 监控配置
	Metrics monitoring.MetricsConfig `json:"metrics"`

	// 缓存配置
	Cache cache.CacheConfig `json:"cache"`

	// 清理配置
	Cleanup cleanup.CleanupConfig `json:"cleanup"`

	// 上传配置
	Upload upload.UploadConfig `json:"upload"`

	// 下载配置
	Download download.DownloadConfig `json:"download"`

	// 验证配置
	Validation validation.ValidatorConfig `json:"validation"`

	// 限流配置
	RateLimit middleware.RateLimiterConfig `json:"rate_limit"`

	// 负载均衡配置
	LoadBalancer loadbalancer.LoadBalancerConfig `json:"load_balancer"`

	// 配置管理配置
	ConfigManager config.ConfigOptions `json:"config_manager"`
}

// DatabaseConfig 数据库配置
type DatabaseConfig struct {
	DSN             string `json:"dsn"`
	MaxOpenConns    int    `json:"max_open_conns"`
	MaxIdleConns    int    `json:"max_idle_conns"`
	ConnMaxLifetime string `json:"conn_max_lifetime"`
	AutoMigrate     bool   `json:"auto_migrate"`
}

// NewServiceManager 创建服务管理器
func NewServiceManager(configPath string) (*ServiceManager, error) {
	ctx, cancel := context.WithCancel(context.Background())

	sm := &ServiceManager{
		ctx:    ctx,
		cancel: cancel,
	}

	// 加载配置
	if err := sm.loadConfig(configPath); err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	// 初始化服务
	if err := sm.initializeServices(); err != nil {
		return nil, fmt.Errorf("failed to initialize services: %w", err)
	}

	return sm, nil
}

// loadConfig 加载配置
func (sm *ServiceManager) loadConfig(configPath string) error {
	// 创建配置管理器
	configMgr, err := config.NewConfigManager(config.ConfigOptions{
		ConfigFile: configPath,
		WatchChanges: true,
		Format: config.ConfigFormatJSON,
	})
	if err != nil {
		return fmt.Errorf("failed to create config manager: %w", err)
	}

	sm.ConfigMgr = configMgr

	// 加载服务配置
	sm.config = &ServiceConfig{}
	if err := sm.ConfigMgr.Get("services", sm.config); err != nil {
		// 使用默认配置
		sm.config = sm.getDefaultConfig()
	}

	// 监听配置变化
	sm.ConfigMgr.AddChangeHook("services", sm.onConfigChange)

	return nil
}

// getDefaultConfig 获取默认配置
func (sm *ServiceManager) getDefaultConfig() *ServiceConfig {
	return &ServiceConfig{
		Database: DatabaseConfig{
			DSN:             "file:ebook_generator.db?cache=shared&mode=rwc",
			MaxOpenConns:    25,
			MaxIdleConns:    5,
			ConnMaxLifetime: "5m",
			AutoMigrate:     true,
		},
		Logging: logging.LogConfig{
			Level:      logging.LogLevelInfo,
			Format:     logging.LogFormatJSON,
			Output:     []string{"console", "file"},
			FilePath:   "logs/app.log",
			MaxSize:    100,
			MaxBackups: 10,
			MaxAge:     30,
			Compress:   true,
		},
		Health: health.HealthConfig{
			CheckInterval: 30 * time.Second,
			Timeout:       5 * time.Second,
			Endpoint:      "/health",
		},
		Metrics: monitoring.MetricsConfig{
			Enabled:         true,
			CollectInterval: 15 * time.Second,
			RetentionPeriod: 24 * time.Hour,
			Endpoint:        "/metrics",
		},
		Cache: cache.CacheConfig{
			MaxSize:         1000,
			TTL:             1 * time.Hour,
			CleanupInterval: 10 * time.Minute,
			CacheDir:        "cache",
		},
		Cleanup: cleanup.CleanupConfig{
			Enabled:         true,
			Interval:        1 * time.Hour,
			MaxAge:          24 * time.Hour,
			MaxSize:         10 * 1024 * 1024 * 1024, // 10GB
			Directories:     []string{"uploads", "downloads", "temp"},
		},
		Upload: upload.UploadConfig{
			ChunkSize:       1024 * 1024, // 1MB
			MaxFileSize:     100 * 1024 * 1024, // 100MB
			MaxConcurrency:  3,
			SessionTimeout:  30 * time.Minute,
			CleanupInterval: 1 * time.Hour,
			TempDir:         "temp/uploads",
			AllowedTypes:    []string{".txt", ".md", ".html", ".epub", ".mobi", ".azw3"},
			ChecksumType:    "md5",
		},
		Download: download.DownloadConfig{
			MaxConcurrent:   3,
			ChunkSize:       1024 * 1024, // 1MB
			MaxRetries:      3,
			RetryDelay:      5 * time.Second,
			Timeout:         30 * time.Second,
			DownloadDir:     "downloads",
			TempDir:         "temp/downloads",
			MaxFileSize:     1024 * 1024 * 1024, // 1GB
			CleanupInterval: 1 * time.Hour,
			KeepCompleted:   24 * time.Hour,
		},
		Validation: validation.ValidatorConfig{
			MaxFileSize:     100 * 1024 * 1024, // 100MB
			AllowedTypes:    []string{".txt", ".md", ".html", ".epub", ".mobi", ".azw3"},
			CheckContent:    true,
			CheckEncoding:   true,
			StrictMode:      false,
		},
		RateLimit: middleware.RateLimiterConfig{
			Enabled:        true,
			GlobalRate:     100,
			GlobalBurst:    200,
			PerIPRate:      10,
			PerIPBurst:     20,
			PerUserRate:    50,
			PerUserBurst:   100,
			WindowSize:     time.Minute,
			CleanupInterval: 5 * time.Minute,
		},
		LoadBalancer: loadbalancer.LoadBalancerConfig{
			Algorithm:           loadbalancer.AlgorithmRoundRobin,
			HealthCheckInterval: 30 * time.Second,
			HealthCheckTimeout:  5 * time.Second,
			HealthCheckPath:     "/health",
			MaxRetries:          3,
			RetryDelay:          1 * time.Second,
			SessionSticky:       false,
			SessionTimeout:      30 * time.Minute,
			CircuitBreaker: loadbalancer.CircuitBreakerConfig{
				Enabled:           true,
				FailureThreshold:  5,
				SuccessThreshold:  3,
				Timeout:           60 * time.Second,
				HalfOpenRequests:  3,
			},
			Metrics: true,
			Logging: true,
		},
		ConfigManager: config.ConfigOptions{
			WatchChanges: true,
			Format:       config.ConfigFormatJSON,
		},
	}
}

// initializeServices 初始化服务
func (sm *ServiceManager) initializeServices() error {
	// 初始化日志服务
	if err := sm.initLogger(); err != nil {
		return fmt.Errorf("failed to init logger: %w", err)
	}

	// 初始化数据库
	if err := sm.initDatabase(); err != nil {
		return fmt.Errorf("failed to init database: %w", err)
	}

	// 初始化监控服务
	if err := sm.initMetrics(); err != nil {
		return fmt.Errorf("failed to init metrics: %w", err)
	}

	// 初始化健康检查服务
	if err := sm.initHealth(); err != nil {
		return fmt.Errorf("failed to init health: %w", err)
	}

	// 初始化缓存服务
	if err := sm.initCache(); err != nil {
		return fmt.Errorf("failed to init cache: %w", err)
	}

	// 初始化清理服务
	if err := sm.initCleanup(); err != nil {
		return fmt.Errorf("failed to init cleanup: %w", err)
	}

	// 初始化上传服务
	if err := sm.initUpload(); err != nil {
		return fmt.Errorf("failed to init upload: %w", err)
	}

	// 初始化下载服务
	if err := sm.initDownload(); err != nil {
		return fmt.Errorf("failed to init download: %w", err)
	}

	// 初始化验证服务
	if err := sm.initValidator(); err != nil {
		return fmt.Errorf("failed to init validator: %w", err)
	}

	// 初始化历史服务
	if err := sm.initHistory(); err != nil {
		return fmt.Errorf("failed to init history: %w", err)
	}

	// 初始化限流中间件
	if err := sm.initRateLimit(); err != nil {
		return fmt.Errorf("failed to init rate limit: %w", err)
	}

	// 初始化负载均衡器
	if err := sm.initLoadBalancer(); err != nil {
		return fmt.Errorf("failed to init load balancer: %w", err)
	}

	return nil
}

// initLogger 初始化日志服务
func (sm *ServiceManager) initLogger() error {
	logger, err := logging.NewLoggerService(sm.config.Logging)
	if err != nil {
		return err
	}
	sm.Logger = logger
	return nil
}

// initDatabase 初始化数据库
func (sm *ServiceManager) initDatabase() error {
	db, err := database.InitDatabase(sm.config.Database.DSN)
	if err != nil {
		return err
	}

	// 配置连接池
	sqlDB, err := db.DB()
	if err != nil {
		return err
	}

	sqlDB.SetMaxOpenConns(sm.config.Database.MaxOpenConns)
	sqlDB.SetMaxIdleConns(sm.config.Database.MaxIdleConns)

	if sm.config.Database.ConnMaxLifetime != "" {
		lifetime, err := time.ParseDuration(sm.config.Database.ConnMaxLifetime)
		if err == nil {
			sqlDB.SetConnMaxLifetime(lifetime)
		}
	}

	// 自动迁移
	if sm.config.Database.AutoMigrate {
		if err := database.RunMigrations(db); err != nil {
			return fmt.Errorf("failed to run migrations: %w", err)
		}
	}

	sm.DB = db
	return nil
}

// initMetrics 初始化监控服务
func (sm *ServiceManager) initMetrics() error {
	metrics, err := monitoring.NewMetricsService(sm.config.Metrics)
	if err != nil {
		return err
	}
	sm.MetricsSvc = metrics
	return nil
}

// initHealth 初始化健康检查服务
func (sm *ServiceManager) initHealth() error {
	healthSvc, err := health.NewHealthService(sm.config.Health)
	if err != nil {
		return err
	}

	// 注册数据库健康检查
	healthSvc.RegisterChecker("database", health.NewDatabaseChecker(sm.DB))

	sm.HealthSvc = healthSvc
	return nil
}

// initCache 初始化缓存服务
func (sm *ServiceManager) initCache() error {
	cacheSvc, err := cache.NewCacheService(sm.config.Cache)
	if err != nil {
		return err
	}
	sm.CacheSvc = cacheSvc
	return nil
}

// initCleanup 初始化清理服务
func (sm *ServiceManager) initCleanup() error {
	cleanupSvc, err := cleanup.NewCleanupService(sm.config.Cleanup)
	if err != nil {
		return err
	}
	sm.CleanupSvc = cleanupSvc
	return nil
}

// initUpload 初始化上传服务
func (sm *ServiceManager) initUpload() error {
	uploadSvc, err := upload.NewStreamUploadService(sm.config.Upload)
	if err != nil {
		return err
	}
	sm.UploadSvc = uploadSvc
	return nil
}

// initDownload 初始化下载服务
func (sm *ServiceManager) initDownload() error {
	downloadMgr, err := download.NewDownloadManager(sm.config.Download)
	if err != nil {
		return err
	}
	sm.DownloadMgr = downloadMgr
	return nil
}

// initValidator 初始化验证服务
func (sm *ServiceManager) initValidator() error {
	validator, err := validation.NewFileValidator(sm.config.Validation)
	if err != nil {
		return err
	}
	sm.Validator = validator
	return nil
}

// initHistory 初始化历史服务
func (sm *ServiceManager) initHistory() error {
	historySvc := services.NewHistoryService(sm.DB)
	sm.HistorySvc = historySvc
	return nil
}

// initRateLimit 初始化限流中间件
func (sm *ServiceManager) initRateLimit() error {
	rateLimiter, err := middleware.NewRateLimiter(sm.config.RateLimit)
	if err != nil {
		return err
	}
	sm.RateLimiter = rateLimiter
	return nil
}

// initLoadBalancer 初始化负载均衡器
func (sm *ServiceManager) initLoadBalancer() error {
	loadBalancer, err := loadbalancer.NewLoadBalancer(sm.config.LoadBalancer)
	if err != nil {
		return err
	}
	sm.LoadBalancer = loadBalancer
	return nil
}

// Start 启动所有服务
func (sm *ServiceManager) Start() error {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()

	if sm.started {
		return fmt.Errorf("services already started")
	}

	// 启动日志服务
	if sm.Logger != nil {
		if err := sm.Logger.Start(); err != nil {
			return fmt.Errorf("failed to start logger: %w", err)
		}
		sm.Logger.Info("Logger service started")
	}

	// 启动监控服务
	if sm.MetricsSvc != nil {
		if err := sm.MetricsSvc.Start(); err != nil {
			return fmt.Errorf("failed to start metrics: %w", err)
		}
		sm.Logger.Info("Metrics service started")
	}

	// 启动健康检查服务
	if sm.HealthSvc != nil {
		if err := sm.HealthSvc.Start(); err != nil {
			return fmt.Errorf("failed to start health: %w", err)
		}
		sm.Logger.Info("Health service started")
	}

	// 启动缓存服务
	if sm.CacheSvc != nil {
		if err := sm.CacheSvc.Start(); err != nil {
			return fmt.Errorf("failed to start cache: %w", err)
		}
		sm.Logger.Info("Cache service started")
	}

	// 启动清理服务
	if sm.CleanupSvc != nil {
		if err := sm.CleanupSvc.Start(); err != nil {
			return fmt.Errorf("failed to start cleanup: %w", err)
		}
		sm.Logger.Info("Cleanup service started")
	}

	// 启动上传服务
	if sm.UploadSvc != nil {
		if err := sm.UploadSvc.Start(); err != nil {
			return fmt.Errorf("failed to start upload: %w", err)
		}
		sm.Logger.Info("Upload service started")
	}

	sm.started = true
	sm.Logger.Info("All services started successfully")

	return nil
}

// Stop 停止所有服务
func (sm *ServiceManager) Stop() error {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()

	if !sm.started {
		return nil
	}

	sm.Logger.Info("Stopping all services...")

	// 停止服务（逆序）
	if sm.UploadSvc != nil {
		sm.UploadSvc.Stop()
		sm.Logger.Info("Upload service stopped")
	}

	if sm.CleanupSvc != nil {
		sm.CleanupSvc.Stop()
		sm.Logger.Info("Cleanup service stopped")
	}

	if sm.CacheSvc != nil {
		sm.CacheSvc.Stop()
		sm.Logger.Info("Cache service stopped")
	}

	if sm.HealthSvc != nil {
		sm.HealthSvc.Stop()
		sm.Logger.Info("Health service stopped")
	}

	if sm.MetricsSvc != nil {
		sm.MetricsSvc.Stop()
		sm.Logger.Info("Metrics service stopped")
	}

	if sm.DownloadMgr != nil {
		sm.DownloadMgr.Stop()
		sm.Logger.Info("Download manager stopped")
	}

	if sm.LoadBalancer != nil {
		sm.LoadBalancer.Stop()
		sm.Logger.Info("Load balancer stopped")
	}

	// 关闭数据库连接
	if sm.DB != nil {
		if sqlDB, err := sm.DB.DB(); err == nil {
			sqlDB.Close()
		}
		sm.Logger.Info("Database connection closed")
	}

	// 最后停止日志服务
	if sm.Logger != nil {
		sm.Logger.Info("All services stopped successfully")
		sm.Logger.Stop()
	}

	sm.cancel()
	sm.started = false

	return nil
}

// onConfigChange 配置变更回调
func (sm *ServiceManager) onConfigChange(key string, oldValue, newValue interface{}) {
	if sm.Logger != nil {
		sm.Logger.Info("Configuration changed", map[string]interface{}{
			"key": key,
		})
	}

	// 重新加载配置
	newConfig := &ServiceConfig{}
	if err := sm.ConfigMgr.Get("services", newConfig); err != nil {
		if sm.Logger != nil {
			sm.Logger.Error("Failed to reload configuration", map[string]interface{}{
				"error": err.Error(),
			})
		}
		return
	}

	sm.mutex.Lock()
	sm.config = newConfig
	sm.mutex.Unlock()

	// 通知相关服务重新加载配置
	sm.reloadServices()
}

// reloadServices 重新加载服务配置
func (sm *ServiceManager) reloadServices() {
	// 重新加载日志配置
	if sm.Logger != nil {
		sm.Logger.UpdateConfig(sm.config.Logging)
	}

	// 重新加载监控配置
	if sm.MetricsSvc != nil {
		sm.MetricsSvc.UpdateConfig(sm.config.Metrics)
	}

	// 重新加载缓存配置
	if sm.CacheSvc != nil {
		sm.CacheSvc.UpdateConfig(sm.config.Cache)
	}

	// 重新加载清理配置
	if sm.CleanupSvc != nil {
		sm.CleanupSvc.UpdateConfig(sm.config.Cleanup)
	}

	// 重新加载限流配置
	if sm.RateLimiter != nil {
		sm.RateLimiter.UpdateConfig(sm.config.RateLimit)
	}
}

// GetConfig 获取配置
func (sm *ServiceManager) GetConfig() *ServiceConfig {
	sm.mutex.RLock()
	defer sm.mutex.RUnlock()
	return sm.config
}

// IsStarted 检查服务是否已启动
func (sm *ServiceManager) IsStarted() bool {
	sm.mutex.RLock()
	defer sm.mutex.RUnlock()
	return sm.started
}

// GetServiceStatus 获取服务状态
func (sm *ServiceManager) GetServiceStatus() map[string]interface{} {
	status := make(map[string]interface{})

	status["started"] = sm.IsStarted()
	status["services"] = map[string]interface{}{
		"database":     sm.DB != nil,
		"logger":       sm.Logger != nil,
		"health":       sm.HealthSvc != nil,
		"metrics":      sm.MetricsSvc != nil,
		"cache":        sm.CacheSvc != nil,
		"cleanup":      sm.CleanupSvc != nil,
		"upload":       sm.UploadSvc != nil,
		"download":     sm.DownloadMgr != nil,
		"validator":    sm.Validator != nil,
		"history":      sm.HistorySvc != nil,
		"rate_limiter": sm.RateLimiter != nil,
		"load_balancer": sm.LoadBalancer != nil,
	}

	// 获取健康状态
	if sm.HealthSvc != nil {
		healthReport := sm.HealthSvc.CheckHealth()
		status["health_report"] = healthReport
	}

	// 获取统计信息
	if sm.MetricsSvc != nil {
		metrics := sm.MetricsSvc.GetMetrics()
		status["metrics"] = metrics
	}

	return status
}

// Restart 重启服务管理器
func (sm *ServiceManager) Restart() error {
	if err := sm.Stop(); err != nil {
		return fmt.Errorf("failed to stop services: %w", err)
	}

	// 等待一段时间确保资源释放
	time.Sleep(2 * time.Second)

	if err := sm.Start(); err != nil {
		return fmt.Errorf("failed to start services: %w", err)
	}

	return nil
}

// HealthCheck 健康检查
func (sm *ServiceManager) HealthCheck() error {
	if !sm.IsStarted() {
		return fmt.Errorf("services not started")
	}

	// 检查数据库连接
	if sm.DB != nil {
		if sqlDB, err := sm.DB.DB(); err == nil {
			if err := sqlDB.Ping(); err != nil {
				return fmt.Errorf("database ping failed: %w", err)
			}
		}
	}

	// 检查其他关键服务
	if sm.HealthSvc != nil {
		report := sm.HealthSvc.CheckHealth()
		if report.Status != health.HealthStatusHealthy {
			return fmt.Errorf("health check failed: %s", report.Message)
		}
	}

	return nil
}