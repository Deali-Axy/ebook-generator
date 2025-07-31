package health

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"runtime"
	"sync"
	"time"
)

// HealthService 健康检查服务
type HealthService struct {
	checkers map[string]HealthChecker
	mutex    sync.RWMutex
	config   HealthConfig
	lastCheck time.Time
	lastResult *HealthReport
}

// HealthConfig 健康检查配置
type HealthConfig struct {
	Enabled         bool          `json:"enabled"`
	CheckInterval   time.Duration `json:"check_interval"`
	Timeout         time.Duration `json:"timeout"`
	CacheResults    bool          `json:"cache_results"`
	CacheDuration   time.Duration `json:"cache_duration"`
	FailureThreshold int          `json:"failure_threshold"`
	IncludeDetails  bool          `json:"include_details"`
}

// HealthChecker 健康检查器接口
type HealthChecker interface {
	Check(ctx context.Context) HealthResult
	Name() string
	Description() string
	Critical() bool
}

// HealthResult 健康检查结果
type HealthResult struct {
	Status    HealthStatus           `json:"status"`
	Message   string                 `json:"message,omitempty"`
	Details   map[string]interface{} `json:"details,omitempty"`
	Duration  time.Duration          `json:"duration"`
	Timestamp time.Time              `json:"timestamp"`
	Error     string                 `json:"error,omitempty"`
}

// HealthStatus 健康状态
type HealthStatus string

const (
	HealthStatusHealthy   HealthStatus = "healthy"
	HealthStatusUnhealthy HealthStatus = "unhealthy"
	HealthStatusDegraded  HealthStatus = "degraded"
	HealthStatusUnknown   HealthStatus = "unknown"
)

// HealthReport 健康报告
type HealthReport struct {
	Status      HealthStatus             `json:"status"`
	Timestamp   time.Time                `json:"timestamp"`
	Duration    time.Duration            `json:"duration"`
	Checks      map[string]HealthResult  `json:"checks"`
	Summary     HealthSummary            `json:"summary"`
	SystemInfo  SystemInfo               `json:"system_info,omitempty"`
	Version     string                   `json:"version,omitempty"`
	Environment string                   `json:"environment,omitempty"`
}

// HealthSummary 健康摘要
type HealthSummary struct {
	Total     int `json:"total"`
	Healthy   int `json:"healthy"`
	Unhealthy int `json:"unhealthy"`
	Degraded  int `json:"degraded"`
	Unknown   int `json:"unknown"`
}

// SystemInfo 系统信息
type SystemInfo struct {
	Hostname     string    `json:"hostname"`
	OS           string    `json:"os"`
	Architecture string    `json:"architecture"`
	GoVersion    string    `json:"go_version"`
	Goroutines   int       `json:"goroutines"`
	Memory       MemoryInfo `json:"memory"`
	Uptime       time.Duration `json:"uptime"`
	StartTime    time.Time `json:"start_time"`
}

// MemoryInfo 内存信息
type MemoryInfo struct {
	Allocated     uint64 `json:"allocated"`
	TotalAlloc    uint64 `json:"total_alloc"`
	Sys           uint64 `json:"sys"`
	NumGC         uint32 `json:"num_gc"`
	GCCPUFraction float64 `json:"gc_cpu_fraction"`
}

// DatabaseChecker 数据库健康检查器
type DatabaseChecker struct {
	db          *sql.DB
	name        string
	description string
	critical    bool
	timeout     time.Duration
}

// HTTPChecker HTTP服务健康检查器
type HTTPChecker struct {
	url         string
	name        string
	description string
	critical    bool
	timeout     time.Duration
	method      string
	headers     map[string]string
	expectedStatus int
}

// DiskSpaceChecker 磁盘空间检查器
type DiskSpaceChecker struct {
	path        string
	name        string
	description string
	critical    bool
	threshold   float64 // 使用率阈值（百分比）
}

// MemoryChecker 内存检查器
type MemoryChecker struct {
	name        string
	description string
	critical    bool
	threshold   float64 // 使用率阈值（百分比）
}

// CustomChecker 自定义检查器
type CustomChecker struct {
	name        string
	description string
	critical    bool
	checkFunc   func(ctx context.Context) HealthResult
}

var (
	startTime = time.Now()
)

// NewHealthService 创建健康检查服务
func NewHealthService(config HealthConfig) *HealthService {
	// 设置默认值
	if config.CheckInterval == 0 {
		config.CheckInterval = 30 * time.Second
	}
	if config.Timeout == 0 {
		config.Timeout = 10 * time.Second
	}
	if config.CacheDuration == 0 {
		config.CacheDuration = 5 * time.Second
	}
	if config.FailureThreshold == 0 {
		config.FailureThreshold = 3
	}

	return &HealthService{
		checkers: make(map[string]HealthChecker),
		config:   config,
	}
}

// RegisterChecker 注册健康检查器
func (hs *HealthService) RegisterChecker(checker HealthChecker) {
	hs.mutex.Lock()
	defer hs.mutex.Unlock()
	hs.checkers[checker.Name()] = checker
}

// UnregisterChecker 注销健康检查器
func (hs *HealthService) UnregisterChecker(name string) {
	hs.mutex.Lock()
	defer hs.mutex.Unlock()
	delete(hs.checkers, name)
}

// Check 执行健康检查
func (hs *HealthService) Check(ctx context.Context) *HealthReport {
	// 检查缓存
	if hs.config.CacheResults && hs.lastResult != nil {
		if time.Since(hs.lastCheck) < hs.config.CacheDuration {
			return hs.lastResult
		}
	}

	start := time.Now()
	report := &HealthReport{
		Timestamp: start,
		Checks:    make(map[string]HealthResult),
		Summary:   HealthSummary{},
	}

	// 添加系统信息
	if hs.config.IncludeDetails {
		report.SystemInfo = hs.getSystemInfo()
	}

	// 执行所有检查
	hs.mutex.RLock()
	checkers := make(map[string]HealthChecker)
	for name, checker := range hs.checkers {
		checkers[name] = checker
	}
	hs.mutex.RUnlock()

	// 并发执行检查
	var wg sync.WaitGroup
	var resultMutex sync.Mutex

	for name, checker := range checkers {
		wg.Add(1)
		go func(name string, checker HealthChecker) {
			defer wg.Done()

			// 创建带超时的上下文
			checkCtx, cancel := context.WithTimeout(ctx, hs.config.Timeout)
			defer cancel()

			// 执行检查
			result := checker.Check(checkCtx)

			// 添加到报告
			resultMutex.Lock()
			report.Checks[name] = result
			resultMutex.Unlock()
		}(name, checker)
	}

	wg.Wait()

	// 计算总体状态和摘要
	report.Duration = time.Since(start)
	report.Summary.Total = len(report.Checks)

	overallStatus := HealthStatusHealthy
	for _, result := range report.Checks {
		switch result.Status {
		case HealthStatusHealthy:
			report.Summary.Healthy++
		case HealthStatusUnhealthy:
			report.Summary.Unhealthy++
			overallStatus = HealthStatusUnhealthy
		case HealthStatusDegraded:
			report.Summary.Degraded++
			if overallStatus == HealthStatusHealthy {
				overallStatus = HealthStatusDegraded
			}
		case HealthStatusUnknown:
			report.Summary.Unknown++
			if overallStatus == HealthStatusHealthy {
				overallStatus = HealthStatusUnknown
			}
		}
	}

	report.Status = overallStatus

	// 更新缓存
	hs.lastCheck = time.Now()
	hs.lastResult = report

	return report
}

// getSystemInfo 获取系统信息
func (hs *HealthService) getSystemInfo() SystemInfo {
	hostname, _ := os.Hostname()

	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	return SystemInfo{
		Hostname:     hostname,
		OS:           runtime.GOOS,
		Architecture: runtime.GOARCH,
		GoVersion:    runtime.Version(),
		Goroutines:   runtime.NumGoroutine(),
		Memory: MemoryInfo{
			Allocated:     m.Alloc,
			TotalAlloc:    m.TotalAlloc,
			Sys:           m.Sys,
			NumGC:         m.NumGC,
			GCCPUFraction: m.GCCPUFraction,
		},
		Uptime:    time.Since(startTime),
		StartTime: startTime,
	}
}

// HTTPHandler 返回HTTP处理器
func (hs *HealthService) HTTPHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		report := hs.Check(r.Context())

		// 设置状态码
		switch report.Status {
		case HealthStatusHealthy:
			w.WriteHeader(http.StatusOK)
		case HealthStatusDegraded:
			w.WriteHeader(http.StatusOK) // 降级但仍可用
		case HealthStatusUnhealthy:
			w.WriteHeader(http.StatusServiceUnavailable)
		default:
			w.WriteHeader(http.StatusInternalServerError)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(report)
	}
}

// NewDatabaseChecker 创建数据库检查器
func NewDatabaseChecker(db *sql.DB, name, description string, critical bool, timeout time.Duration) *DatabaseChecker {
	return &DatabaseChecker{
		db:          db,
		name:        name,
		description: description,
		critical:    critical,
		timeout:     timeout,
	}
}

// Check 执行数据库检查
func (dc *DatabaseChecker) Check(ctx context.Context) HealthResult {
	start := time.Now()
	result := HealthResult{
		Timestamp: start,
		Details:   make(map[string]interface{}),
	}

	// 创建带超时的上下文
	checkCtx, cancel := context.WithTimeout(ctx, dc.timeout)
	defer cancel()

	// 执行ping
	err := dc.db.PingContext(checkCtx)
	result.Duration = time.Since(start)

	if err != nil {
		result.Status = HealthStatusUnhealthy
		result.Error = err.Error()
		result.Message = "Database connection failed"
	} else {
		result.Status = HealthStatusHealthy
		result.Message = "Database connection successful"

		// 获取连接统计
		stats := dc.db.Stats()
		result.Details["open_connections"] = stats.OpenConnections
		result.Details["in_use"] = stats.InUse
		result.Details["idle"] = stats.Idle
		result.Details["max_open_connections"] = stats.MaxOpenConnections
	}

	return result
}

// Name 返回检查器名称
func (dc *DatabaseChecker) Name() string {
	return dc.name
}

// Description 返回检查器描述
func (dc *DatabaseChecker) Description() string {
	return dc.description
}

// Critical 返回是否为关键检查
func (dc *DatabaseChecker) Critical() bool {
	return dc.critical
}

// NewHTTPChecker 创建HTTP检查器
func NewHTTPChecker(url, name, description string, critical bool, timeout time.Duration) *HTTPChecker {
	return &HTTPChecker{
		url:            url,
		name:           name,
		description:    description,
		critical:       critical,
		timeout:        timeout,
		method:         "GET",
		headers:        make(map[string]string),
		expectedStatus: 200,
	}
}

// Check 执行HTTP检查
func (hc *HTTPChecker) Check(ctx context.Context) HealthResult {
	start := time.Now()
	result := HealthResult{
		Timestamp: start,
		Details:   make(map[string]interface{}),
	}

	// 创建HTTP客户端
	client := &http.Client{
		Timeout: hc.timeout,
	}

	// 创建请求
	req, err := http.NewRequestWithContext(ctx, hc.method, hc.url, nil)
	if err != nil {
		result.Status = HealthStatusUnhealthy
		result.Error = err.Error()
		result.Duration = time.Since(start)
		return result
	}

	// 添加头部
	for k, v := range hc.headers {
		req.Header.Set(k, v)
	}

	// 执行请求
	resp, err := client.Do(req)
	result.Duration = time.Since(start)

	if err != nil {
		result.Status = HealthStatusUnhealthy
		result.Error = err.Error()
		result.Message = fmt.Sprintf("HTTP request to %s failed", hc.url)
	} else {
		defer resp.Body.Close()
		result.Details["status_code"] = resp.StatusCode
		result.Details["response_time_ms"] = result.Duration.Milliseconds()

		if resp.StatusCode == hc.expectedStatus {
			result.Status = HealthStatusHealthy
			result.Message = fmt.Sprintf("HTTP request to %s successful", hc.url)
		} else {
			result.Status = HealthStatusUnhealthy
			result.Message = fmt.Sprintf("HTTP request to %s returned unexpected status: %d", hc.url, resp.StatusCode)
		}
	}

	return result
}

// Name 返回检查器名称
func (hc *HTTPChecker) Name() string {
	return hc.name
}

// Description 返回检查器描述
func (hc *HTTPChecker) Description() string {
	return hc.description
}

// Critical 返回是否为关键检查
func (hc *HTTPChecker) Critical() bool {
	return hc.critical
}

// NewMemoryChecker 创建内存检查器
func NewMemoryChecker(name, description string, critical bool, threshold float64) *MemoryChecker {
	return &MemoryChecker{
		name:        name,
		description: description,
		critical:    critical,
		threshold:   threshold,
	}
}

// Check 执行内存检查
func (mc *MemoryChecker) Check(ctx context.Context) HealthResult {
	start := time.Now()
	result := HealthResult{
		Timestamp: start,
		Details:   make(map[string]interface{}),
	}

	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	// 计算内存使用率
	usagePercent := float64(m.HeapInuse) / float64(m.HeapSys) * 100

	result.Duration = time.Since(start)
	result.Details["heap_inuse"] = m.HeapInuse
	result.Details["heap_sys"] = m.HeapSys
	result.Details["usage_percent"] = usagePercent
	result.Details["threshold_percent"] = mc.threshold

	if usagePercent > mc.threshold {
		result.Status = HealthStatusUnhealthy
		result.Message = fmt.Sprintf("Memory usage %.2f%% exceeds threshold %.2f%%", usagePercent, mc.threshold)
	} else if usagePercent > mc.threshold*0.8 {
		result.Status = HealthStatusDegraded
		result.Message = fmt.Sprintf("Memory usage %.2f%% is approaching threshold %.2f%%", usagePercent, mc.threshold)
	} else {
		result.Status = HealthStatusHealthy
		result.Message = fmt.Sprintf("Memory usage %.2f%% is within acceptable range", usagePercent)
	}

	return result
}

// Name 返回检查器名称
func (mc *MemoryChecker) Name() string {
	return mc.name
}

// Description 返回检查器描述
func (mc *MemoryChecker) Description() string {
	return mc.description
}

// Critical 返回是否为关键检查
func (mc *MemoryChecker) Critical() bool {
	return mc.critical
}

// NewCustomChecker 创建自定义检查器
func NewCustomChecker(name, description string, critical bool, checkFunc func(ctx context.Context) HealthResult) *CustomChecker {
	return &CustomChecker{
		name:        name,
		description: description,
		critical:    critical,
		checkFunc:   checkFunc,
	}
}

// Check 执行自定义检查
func (cc *CustomChecker) Check(ctx context.Context) HealthResult {
	return cc.checkFunc(ctx)
}

// Name 返回检查器名称
func (cc *CustomChecker) Name() string {
	return cc.name
}

// Description 返回检查器描述
func (cc *CustomChecker) Description() string {
	return cc.description
}

// Critical 返回是否为关键检查
func (cc *CustomChecker) Critical() bool {
	return cc.critical
}