# 电子书生成器 - 增强功能文档

本文档详细介绍了电子书生成器的所有增强功能，包括安全性、性能优化、用户体验和监控运维等方面的改进。

## 🔒 安全性增强

### 1. 用户认证系统

#### 数据库迁移
- **文件**: `internal/database/migrations.go`
- **功能**: 自动创建用户表结构，包含默认管理员账户
- **特性**:
  - 用户注册、登录、权限管理
  - 密码加密存储（bcrypt）
  - JWT token 认证
  - 角色权限控制

```go
// 使用示例
db, err := database.InitDatabase("your-database-dsn")
if err != nil {
    log.Fatal(err)
}

// 自动运行迁移
if err := database.RunMigrations(db); err != nil {
    log.Fatal(err)
}
```

### 2. 更严格的限流

#### 多策略限流中间件
- **文件**: `internal/web/middleware/rate_limiter.go`
- **功能**: 支持多种限流策略的高级限流器
- **特性**:
  - 基于IP的限流
  - 基于用户ID的限流
  - 基于API Key的限流
  - 令牌桶算法实现
  - 分布式限流支持（Redis）
  - 白名单/黑名单功能

```go
// 配置示例
config := middleware.RateLimiterConfig{
    Enabled:        true,
    GlobalRate:     100,
    GlobalBurst:    200,
    PerIPRate:      10,
    PerIPBurst:     20,
    PerUserRate:    50,
    PerUserBurst:   100,
    WindowSize:     time.Minute,
    CleanupInterval: 5 * time.Minute,
}

rateLimiter, err := middleware.NewRateLimiter(config)
```

### 3. 文件内容验证

#### 高级文件验证器
- **文件**: `internal/validation/file_validator.go`
- **功能**: 深度文件内容验证，不仅检查扩展名
- **特性**:
  - MIME类型检测
  - 文件头部验证
  - UTF-8编码验证
  - 二进制内容检查
  - 特定格式验证（EPUB、MOBI、AZW3等）
  - 病毒扫描接口

```go
// 使用示例
validator, err := validation.NewFileValidator(validation.ValidatorConfig{
    MaxFileSize:   100 * 1024 * 1024, // 100MB
    AllowedTypes:  []string{".txt", ".md", ".epub"},
    CheckContent:  true,
    CheckEncoding: true,
    StrictMode:    false,
})

result := validator.ValidateFile("path/to/file.txt")
if !result.Valid {
    log.Printf("Validation failed: %v", result.Errors)
}
```

## ⚡ 性能优化

### 1. 文件流式上传

#### 分块上传服务
- **文件**: `internal/upload/stream_upload_service.go`
- **功能**: 支持大文件分块上传和断点续传
- **特性**:
  - 分块上传
  - 断点续传
  - 并发上传
  - 校验和验证
  - 上传进度跟踪
  - 会话管理

```go
// 使用示例
uploadSvc, err := upload.NewStreamUploadService(upload.UploadConfig{
    ChunkSize:       1024 * 1024, // 1MB
    MaxFileSize:     100 * 1024 * 1024, // 100MB
    MaxConcurrency:  3,
    SessionTimeout:  30 * time.Minute,
    TempDir:         "temp/uploads",
    AllowedTypes:    []string{".txt", ".md", ".epub"},
    ChecksumType:    "md5",
})

// 初始化上传
session, err := uploadSvc.InitiateUpload(upload.InitiateRequest{
    FileName: "large-file.txt",
    FileSize: 50 * 1024 * 1024,
    ChunkCount: 50,
})
```

### 2. 定期清理机制

#### 自动清理服务
- **文件**: `internal/cleanup/cleanup_service.go`
- **功能**: 自动清理过期文件和管理磁盘空间
- **特性**:
  - 定时清理任务
  - 磁盘空间监控
  - 文件压缩
  - 清理统计
  - 紧急清理机制
  - 可配置清理策略

```go
// 使用示例
cleanupSvc, err := cleanup.NewCleanupService(cleanup.CleanupConfig{
    Enabled:         true,
    Interval:        1 * time.Hour,
    MaxAge:          24 * time.Hour,
    MaxSize:         10 * 1024 * 1024 * 1024, // 10GB
    Directories:     []string{"uploads", "downloads", "temp"},
    PreserveRecent:  10,
    EnableCompression: true,
})

cleanupSvc.Start()
```

### 3. 缓存机制

#### 转换结果缓存
- **文件**: `internal/cache/cache_service.go`
- **功能**: 智能缓存转换结果，提高响应速度
- **特性**:
  - LRU驱逐策略
  - 压缩存储
  - 缓存统计
  - 持久化索引
  - 自动清理
  - 缓存预热

```go
// 使用示例
cacheSvc, err := cache.NewCacheService(cache.CacheConfig{
    MaxSize:         1000,
    TTL:             1 * time.Hour,
    CleanupInterval: 10 * time.Minute,
    CacheDir:        "cache",
    EnableCompression: true,
    EvictionPolicy:  "lru",
})

// 存储缓存
cacheSvc.Set("conversion_123", conversionResult, metadata)

// 获取缓存
result, found := cacheSvc.Get("conversion_123")
```

### 4. 负载均衡

#### 多实例部署支持
- **文件**: `internal/loadbalancer/load_balancer.go`
- **功能**: 支持多种负载均衡算法和健康检查
- **特性**:
  - 多种负载均衡算法（轮询、加权轮询、最少连接、一致性哈希等）
  - 健康检查
  - 熔断器
  - 会话粘性
  - 统计监控

```go
// 使用示例
lb, err := loadbalancer.NewLoadBalancer(loadbalancer.LoadBalancerConfig{
    Algorithm:           loadbalancer.AlgorithmRoundRobin,
    HealthCheckInterval: 30 * time.Second,
    HealthCheckTimeout:  5 * time.Second,
    CircuitBreaker: loadbalancer.CircuitBreakerConfig{
        Enabled:          true,
        FailureThreshold: 5,
        SuccessThreshold: 3,
        Timeout:          60 * time.Second,
    },
})

// 添加后端服务器
lb.AddBackend("http://localhost:8081", 1, nil)
lb.AddBackend("http://localhost:8082", 1, nil)
```

## 🎨 用户体验

### 1. 转换历史记录

#### 历史记录管理
- **文件**: `internal/web/models/history.go`, `internal/web/services/history_service.go`
- **功能**: 完整的转换历史记录功能
- **特性**:
  - 转换历史跟踪
  - 统计分析
  - 下载记录
  - 历史搜索
  - 数据导出

```go
// 使用示例
historySvc := services.NewHistoryService(db)

// 创建历史记录
history := &models.ConversionHistory{
    UserID:     userID,
    TaskID:     taskID,
    FileName:   "document.txt",
    FileSize:   1024,
    InputFormat: "txt",
    OutputFormat: "epub",
    Status:     "pending",
}

if err := historySvc.CreateHistory(history); err != nil {
    log.Printf("Failed to create history: %v", err)
}
```

### 2. 批量转换

#### 批量处理支持
- **文件**: `internal/web/handlers/history_handlers.go`
- **功能**: 支持多文件同时转换
- **特性**:
  - 批量文件上传
  - 并发转换处理
  - 进度跟踪
  - 批量下载
  - 错误处理

### 3. 转换参数预设

#### 预设管理
- **功能**: 保存和管理常用转换配置
- **特性**:
  - 预设创建和编辑
  - 预设分享
  - 默认预设
  - 预设导入导出

### 4. 下载管理

#### 下载队列和断点续传
- **文件**: `internal/download/download_manager.go`
- **功能**: 完整的下载管理系统
- **特性**:
  - 下载队列
  - 断点续传
  - 分块下载
  - 下载统计
  - 重试机制
  - 速度限制

```go
// 使用示例
downloadMgr, err := download.NewDownloadManager(download.DownloadConfig{
    MaxConcurrent:   3,
    ChunkSize:       1024 * 1024, // 1MB
    MaxRetries:      3,
    RetryDelay:      5 * time.Second,
    Timeout:         30 * time.Second,
    DownloadDir:     "downloads",
    MaxFileSize:     1024 * 1024 * 1024, // 1GB
})

// 添加下载任务
response, err := downloadMgr.AddDownload(download.DownloadRequest{
    URL:      "https://example.com/file.zip",
    FileName: "download.zip",
})
```

## 📊 监控和运维

### 1. 性能监控

#### 详细性能指标收集
- **文件**: `internal/monitoring/metrics_service.go`
- **功能**: 全面的性能监控和指标收集
- **特性**:
  - 系统指标（CPU、内存、磁盘）
  - 应用指标（请求数、响应时间、错误率）
  - 自定义指标
  - Prometheus集成
  - 告警机制
  - 性能报告

```go
// 使用示例
metricsSvc, err := monitoring.NewMetricsService(monitoring.MetricsConfig{
    Enabled:         true,
    CollectInterval: 15 * time.Second,
    RetentionPeriod: 24 * time.Hour,
    EnablePrometheus: true,
    EnableAlerts:    true,
})

// 注册自定义指标
metricsSvc.RegisterMetric("conversion_count", monitoring.MetricTypeCounter)
metricsSvc.IncrementCounter("conversion_count", 1)
```

### 2. 日志系统

#### 结构化日志记录和分析
- **文件**: `internal/logging/logger_service.go`
- **功能**: 完善的日志记录和分析系统
- **特性**:
  - 结构化日志
  - 多级别日志
  - 日志轮转
  - 日志分析
  - 日志搜索
  - 日志导出

```go
// 使用示例
logger, err := logging.NewLoggerService(logging.LogConfig{
    Level:      logging.LogLevelInfo,
    Format:     logging.LogFormatJSON,
    Output:     []string{"console", "file"},
    FilePath:   "logs/app.log",
    MaxSize:    100,
    MaxBackups: 10,
    MaxAge:     30,
    Compress:   true,
})

logger.Info("Application started", map[string]interface{}{
    "version": "1.0.0",
    "port":    8080,
})
```

### 3. 健康检查

#### 详细健康状态监控
- **文件**: `internal/health/health_service.go`
- **功能**: 全面的健康检查和监控
- **特性**:
  - 多组件健康检查
  - 自定义检查器
  - 健康报告
  - 依赖检查
  - 健康历史

```go
// 使用示例
healthSvc, err := health.NewHealthService(health.HealthConfig{
    CheckInterval: 30 * time.Second,
    Timeout:       5 * time.Second,
    Endpoint:      "/health",
})

// 注册数据库健康检查
healthSvc.RegisterChecker("database", health.NewDatabaseChecker(db))

// 注册自定义健康检查
healthSvc.RegisterChecker("custom", &CustomHealthChecker{})
```

### 4. 配置管理

#### 动态配置更新机制
- **文件**: `internal/config/config_manager.go`
- **功能**: 支持动态配置更新和热重载
- **特性**:
  - 配置热重载
  - 配置验证
  - 配置版本管理
  - 配置备份
  - 配置监听

```go
// 使用示例
configMgr, err := config.NewConfigManager(config.ConfigOptions{
    ConfigFile:   "config/app.json",
    WatchChanges: true,
    Format:       config.ConfigFormatJSON,
})

// 监听配置变化
configMgr.AddChangeHook("database", func(key string, oldValue, newValue interface{}) {
    log.Printf("Database config changed: %s", key)
    // 重新初始化数据库连接
})
```

## 🚀 服务集成

### 服务管理器

#### 统一服务管理
- **文件**: `internal/services/service_manager.go`
- **功能**: 统一管理所有服务的生命周期
- **特性**:
  - 服务依赖管理
  - 优雅启动和关闭
  - 服务健康监控
  - 配置热重载
  - 服务状态查询

```go
// 使用示例
serviceManager, err := services.NewServiceManager("config/services.json")
if err != nil {
    log.Fatal(err)
}

// 启动所有服务
if err := serviceManager.Start(); err != nil {
    log.Fatal(err)
}

// 优雅关闭
defer serviceManager.Stop()
```

## 📋 配置示例

### 完整配置文件
- **文件**: `config/services.example.json`
- **功能**: 提供完整的配置示例
- **包含**:
  - 所有服务的配置选项
  - 开发和生产环境配置
  - 安全配置
  - 性能调优参数

## 🔧 部署和使用

### 1. 环境准备

```bash
# 创建必要目录
mkdir -p logs cache temp/uploads temp/downloads uploads downloads storage

# 复制配置文件
cp config/services.example.json config/services.json

# 编辑配置文件
vim config/services.json
```

### 2. 启动应用

```go
package main

import (
    "log"
    "os"
    "os/signal"
    "syscall"
    
    "github.com/ebook-generator/internal/services"
)

func main() {
    // 创建服务管理器
    serviceManager, err := services.NewServiceManager("config/services.json")
    if err != nil {
        log.Fatal("Failed to create service manager:", err)
    }

    // 启动所有服务
    if err := serviceManager.Start(); err != nil {
        log.Fatal("Failed to start services:", err)
    }

    log.Println("Application started successfully")

    // 等待信号
    sigChan := make(chan os.Signal, 1)
    signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
    <-sigChan

    log.Println("Shutting down...")

    // 优雅关闭
    if err := serviceManager.Stop(); err != nil {
        log.Printf("Error during shutdown: %v", err)
    }

    log.Println("Application stopped")
}
```

### 3. API 端点

#### 健康检查
```
GET /health
```

#### 监控指标
```
GET /metrics
```

#### 转换历史
```
GET /api/v1/history
POST /api/v1/history
DELETE /api/v1/history/{id}
```

#### 转换预设
```
GET /api/v1/presets
POST /api/v1/presets
PUT /api/v1/presets/{id}
DELETE /api/v1/presets/{id}
```

#### 批量转换
```
POST /api/v1/convert/batch
```

#### 下载管理
```
POST /api/v1/downloads
GET /api/v1/downloads/{id}
DELETE /api/v1/downloads/{id}
```

## 🔍 监控和调试

### 1. 日志查看

```bash
# 查看应用日志
tail -f logs/app.log

# 查看错误日志
grep "ERROR" logs/app.log

# 查看特定用户的操作
grep "user_id:123" logs/app.log
```

### 2. 性能监控

```bash
# 查看系统指标
curl http://localhost:8080/metrics

# 查看健康状态
curl http://localhost:8080/health

# 查看服务状态
curl http://localhost:8080/api/v1/status
```

### 3. 缓存管理

```bash
# 查看缓存统计
curl http://localhost:8080/api/v1/cache/stats

# 清空缓存
curl -X DELETE http://localhost:8080/api/v1/cache

# 预热缓存
curl -X POST http://localhost:8080/api/v1/cache/warmup
```

## 🛠️ 故障排除

### 常见问题

1. **服务启动失败**
   - 检查配置文件格式
   - 确认端口未被占用
   - 检查文件权限

2. **数据库连接失败**
   - 检查数据库配置
   - 确认数据库服务运行
   - 检查网络连接

3. **文件上传失败**
   - 检查磁盘空间
   - 确认目录权限
   - 检查文件大小限制

4. **转换任务失败**
   - 检查转换工具安装
   - 确认文件格式支持
   - 查看详细错误日志

### 性能调优

1. **数据库优化**
   - 调整连接池大小
   - 添加适当索引
   - 定期清理历史数据

2. **缓存优化**
   - 调整缓存大小
   - 优化TTL设置
   - 启用压缩

3. **并发优化**
   - 调整工作线程数
   - 优化队列大小
   - 配置限流参数

## 📈 扩展和定制

### 1. 添加自定义验证器

```go
type CustomValidator struct{}

func (cv *CustomValidator) Validate(filePath string) validation.ValidationResult {
    // 自定义验证逻辑
    return validation.ValidationResult{
        Valid: true,
        Errors: nil,
    }
}

// 注册自定义验证器
validator.RegisterCustomValidator("custom", &CustomValidator{})
```

### 2. 添加自定义健康检查

```go
type CustomHealthChecker struct{}

func (chc *CustomHealthChecker) Check() health.HealthResult {
    // 自定义健康检查逻辑
    return health.HealthResult{
        Status:  health.HealthStatusHealthy,
        Message: "Custom service is healthy",
    }
}

// 注册自定义健康检查
healthSvc.RegisterChecker("custom_service", &CustomHealthChecker{})
```

### 3. 添加自定义指标

```go
// 注册自定义指标
metricsSvc.RegisterMetric("custom_metric", monitoring.MetricTypeGauge)

// 更新指标值
metricsSvc.SetGauge("custom_metric", 42.0)
```

## 🔐 安全最佳实践

1. **配置安全**
   - 使用强密码和密钥
   - 定期轮换密钥
   - 限制配置文件权限

2. **网络安全**
   - 启用HTTPS
   - 配置防火墙
   - 使用VPN或专网

3. **数据安全**
   - 加密敏感数据
   - 定期备份
   - 实施访问控制

4. **运行时安全**
   - 最小权限原则
   - 定期更新依赖
   - 监控异常行为

## 📚 更多资源

- [API 文档](./API.md)
- [部署指南](./DEPLOYMENT.md)
- [开发指南](./DEVELOPMENT.md)
- [故障排除](./TROUBLESHOOTING.md)

---

通过这些增强功能，电子书生成器现在具备了企业级应用所需的安全性、性能、可用性和可维护性。所有功能都经过精心设计，支持高并发、大规模部署和复杂的业务场景。