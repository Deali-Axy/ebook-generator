package monitoring

import (
	"encoding/json"
	"fmt"
	"runtime"
	"sync"
	"time"
)

// MetricsService 性能监控服务
type MetricsService struct {
	metrics map[string]*Metric
	mutex   sync.RWMutex
	config  MetricsConfig
}

// MetricsConfig 监控配置
type MetricsConfig struct {
	Enabled         bool          `json:"enabled"`
	CollectInterval time.Duration `json:"collect_interval"`
	RetentionPeriod time.Duration `json:"retention_period"`
	MaxDataPoints   int           `json:"max_data_points"`
}

// Metric 指标
type Metric struct {
	Name        string      `json:"name"`
	Type        MetricType  `json:"type"`
	Value       float64     `json:"value"`
	Unit        string      `json:"unit"`
	Description string      `json:"description"`
	Timestamp   time.Time   `json:"timestamp"`
	History     []DataPoint `json:"history"`
	Labels      map[string]string `json:"labels,omitempty"`
}

// MetricType 指标类型
type MetricType string

const (
	MetricTypeCounter   MetricType = "counter"
	MetricTypeGauge     MetricType = "gauge"
	MetricTypeHistogram MetricType = "histogram"
	MetricTypeSummary   MetricType = "summary"
)

// DataPoint 数据点
type DataPoint struct {
	Value     float64   `json:"value"`
	Timestamp time.Time `json:"timestamp"`
}

// SystemMetrics 系统指标
type SystemMetrics struct {
	CPUUsage         float64 `json:"cpu_usage"`
	MemoryUsage      float64 `json:"memory_usage"`
	MemoryTotal      uint64  `json:"memory_total"`
	MemoryUsed       uint64  `json:"memory_used"`
	Goroutines       int     `json:"goroutines"`
	GCPauseTotal     float64 `json:"gc_pause_total"`
	GCCount          uint32  `json:"gc_count"`
	HeapSize         uint64  `json:"heap_size"`
	HeapInUse        uint64  `json:"heap_in_use"`
	StackInUse       uint64  `json:"stack_in_use"`
	OpenFileHandles  int     `json:"open_file_handles"`
	NetworkConnections int   `json:"network_connections"`
}

// ApplicationMetrics 应用指标
type ApplicationMetrics struct {
	TotalRequests       int64   `json:"total_requests"`
	ActiveRequests      int64   `json:"active_requests"`
	RequestsPerSecond   float64 `json:"requests_per_second"`
	AverageResponseTime float64 `json:"average_response_time"`
	ErrorRate           float64 `json:"error_rate"`
	ActiveConnections   int64   `json:"active_connections"`
	TotalConversions    int64   `json:"total_conversions"`
	ActiveConversions   int64   `json:"active_conversions"`
	ConversionQueue     int64   `json:"conversion_queue"`
	SuccessfulConversions int64 `json:"successful_conversions"`
	FailedConversions   int64   `json:"failed_conversions"`
	AverageConversionTime float64 `json:"average_conversion_time"`
	CacheHitRate        float64 `json:"cache_hit_rate"`
	CacheSize           int64   `json:"cache_size"`
}

// PerformanceReport 性能报告
type PerformanceReport struct {
	Timestamp           time.Time          `json:"timestamp"`
	SystemMetrics       SystemMetrics      `json:"system_metrics"`
	ApplicationMetrics  ApplicationMetrics `json:"application_metrics"`
	CustomMetrics       map[string]float64 `json:"custom_metrics"`
	Alerts              []Alert            `json:"alerts"`
	Recommendations     []string           `json:"recommendations"`
}

// Alert 告警
type Alert struct {
	Level       AlertLevel `json:"level"`
	Metric      string     `json:"metric"`
	Value       float64    `json:"value"`
	Threshold   float64    `json:"threshold"`
	Message     string     `json:"message"`
	Timestamp   time.Time  `json:"timestamp"`
	Resolved    bool       `json:"resolved"`
}

// AlertLevel 告警级别
type AlertLevel string

const (
	AlertLevelInfo     AlertLevel = "info"
	AlertLevelWarning  AlertLevel = "warning"
	AlertLevelCritical AlertLevel = "critical"
)

// NewMetricsService 创建监控服务
func NewMetricsService(config MetricsConfig) *MetricsService {
	// 设置默认值
	if config.CollectInterval == 0 {
		config.CollectInterval = 30 * time.Second
	}
	if config.RetentionPeriod == 0 {
		config.RetentionPeriod = 24 * time.Hour
	}
	if config.MaxDataPoints == 0 {
		config.MaxDataPoints = 1000
	}

	ms := &MetricsService{
		metrics: make(map[string]*Metric),
		config:  config,
	}

	// 初始化基础指标
	ms.initializeMetrics()

	return ms
}

// Start 启动监控服务
func (ms *MetricsService) Start() {
	if !ms.config.Enabled {
		return
	}

	// 启动指标收集
	go ms.collectMetrics()

	// 启动数据清理
	go ms.cleanupOldData()
}

// initializeMetrics 初始化指标
func (ms *MetricsService) initializeMetrics() {
	// 系统指标
	ms.RegisterMetric("cpu_usage", MetricTypeGauge, "percent", "CPU使用率")
	ms.RegisterMetric("memory_usage", MetricTypeGauge, "percent", "内存使用率")
	ms.RegisterMetric("goroutines", MetricTypeGauge, "count", "Goroutine数量")
	ms.RegisterMetric("gc_pause_total", MetricTypeCounter, "ms", "GC暂停总时间")
	ms.RegisterMetric("heap_size", MetricTypeGauge, "bytes", "堆内存大小")

	// 应用指标
	ms.RegisterMetric("total_requests", MetricTypeCounter, "count", "总请求数")
	ms.RegisterMetric("active_requests", MetricTypeGauge, "count", "活跃请求数")
	ms.RegisterMetric("requests_per_second", MetricTypeGauge, "rps", "每秒请求数")
	ms.RegisterMetric("average_response_time", MetricTypeGauge, "ms", "平均响应时间")
	ms.RegisterMetric("error_rate", MetricTypeGauge, "percent", "错误率")
	ms.RegisterMetric("total_conversions", MetricTypeCounter, "count", "总转换数")
	ms.RegisterMetric("active_conversions", MetricTypeGauge, "count", "活跃转换数")
	ms.RegisterMetric("conversion_queue", MetricTypeGauge, "count", "转换队列长度")
	ms.RegisterMetric("cache_hit_rate", MetricTypeGauge, "percent", "缓存命中率")
}

// RegisterMetric 注册指标
func (ms *MetricsService) RegisterMetric(name string, metricType MetricType, unit, description string) {
	ms.mutex.Lock()
	defer ms.mutex.Unlock()

	ms.metrics[name] = &Metric{
		Name:        name,
		Type:        metricType,
		Unit:        unit,
		Description: description,
		Timestamp:   time.Now(),
		History:     make([]DataPoint, 0),
		Labels:      make(map[string]string),
	}
}

// SetMetric 设置指标值
func (ms *MetricsService) SetMetric(name string, value float64, labels ...map[string]string) {
	ms.mutex.Lock()
	defer ms.mutex.Unlock()

	metric, exists := ms.metrics[name]
	if !exists {
		return
	}

	now := time.Now()
	metric.Value = value
	metric.Timestamp = now

	// 添加标签
	if len(labels) > 0 {
		for k, v := range labels[0] {
			metric.Labels[k] = v
		}
	}

	// 添加到历史记录
	metric.History = append(metric.History, DataPoint{
		Value:     value,
		Timestamp: now,
	})

	// 限制历史记录数量
	if len(metric.History) > ms.config.MaxDataPoints {
		metric.History = metric.History[1:]
	}
}

// IncrementMetric 增加指标值
func (ms *MetricsService) IncrementMetric(name string, delta float64) {
	ms.mutex.Lock()
	defer ms.mutex.Unlock()

	metric, exists := ms.metrics[name]
	if !exists {
		return
	}

	metric.Value += delta
	metric.Timestamp = time.Now()

	// 添加到历史记录
	metric.History = append(metric.History, DataPoint{
		Value:     metric.Value,
		Timestamp: metric.Timestamp,
	})

	// 限制历史记录数量
	if len(metric.History) > ms.config.MaxDataPoints {
		metric.History = metric.History[1:]
	}
}

// GetMetric 获取指标
func (ms *MetricsService) GetMetric(name string) (*Metric, bool) {
	ms.mutex.RLock()
	defer ms.mutex.RUnlock()

	metric, exists := ms.metrics[name]
	if !exists {
		return nil, false
	}

	// 返回副本以避免并发问题
	copy := *metric
	return &copy, true
}

// GetAllMetrics 获取所有指标
func (ms *MetricsService) GetAllMetrics() map[string]*Metric {
	ms.mutex.RLock()
	defer ms.mutex.RUnlock()

	result := make(map[string]*Metric)
	for name, metric := range ms.metrics {
		copy := *metric
		result[name] = &copy
	}
	return result
}

// collectMetrics 收集指标
func (ms *MetricsService) collectMetrics() {
	ticker := time.NewTicker(ms.config.CollectInterval)
	defer ticker.Stop()

	for range ticker.C {
		ms.collectSystemMetrics()
	}
}

// collectSystemMetrics 收集系统指标
func (ms *MetricsService) collectSystemMetrics() {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	// CPU使用率（简化实现，实际应该使用更精确的方法）
	ms.SetMetric("goroutines", float64(runtime.NumGoroutine()))

	// 内存指标
	ms.SetMetric("heap_size", float64(m.HeapSys))
	ms.SetMetric("heap_in_use", float64(m.HeapInuse))
	ms.SetMetric("stack_in_use", float64(m.StackInuse))

	// GC指标
	ms.SetMetric("gc_pause_total", float64(m.PauseTotalNs)/1e6) // 转换为毫秒
	ms.SetMetric("gc_count", float64(m.NumGC))

	// 内存使用率
	memoryUsage := float64(m.HeapInuse) / float64(m.HeapSys) * 100
	ms.SetMetric("memory_usage", memoryUsage)
}

// cleanupOldData 清理过期数据
func (ms *MetricsService) cleanupOldData() {
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	for range ticker.C {
		ms.mutex.Lock()
		cutoff := time.Now().Add(-ms.config.RetentionPeriod)

		for _, metric := range ms.metrics {
			// 清理过期的历史数据
			var newHistory []DataPoint
			for _, point := range metric.History {
				if point.Timestamp.After(cutoff) {
					newHistory = append(newHistory, point)
				}
			}
			metric.History = newHistory
		}
		ms.mutex.Unlock()
	}
}

// GenerateReport 生成性能报告
func (ms *MetricsService) GenerateReport() *PerformanceReport {
	report := &PerformanceReport{
		Timestamp:       time.Now(),
		CustomMetrics:   make(map[string]float64),
		Alerts:          make([]Alert, 0),
		Recommendations: make([]string, 0),
	}

	// 收集系统指标
	report.SystemMetrics = ms.getSystemMetrics()

	// 收集应用指标
	report.ApplicationMetrics = ms.getApplicationMetrics()

	// 收集自定义指标
	for name, metric := range ms.GetAllMetrics() {
		report.CustomMetrics[name] = metric.Value
	}

	// 生成告警
	report.Alerts = ms.generateAlerts()

	// 生成建议
	report.Recommendations = ms.generateRecommendations(report)

	return report
}

// getSystemMetrics 获取系统指标
func (ms *MetricsService) getSystemMetrics() SystemMetrics {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	return SystemMetrics{
		MemoryTotal:    m.Sys,
		MemoryUsed:     m.HeapInuse,
		MemoryUsage:    float64(m.HeapInuse) / float64(m.Sys) * 100,
		Goroutines:     runtime.NumGoroutine(),
		GCPauseTotal:   float64(m.PauseTotalNs) / 1e6,
		GCCount:        m.NumGC,
		HeapSize:       m.HeapSys,
		HeapInUse:      m.HeapInuse,
		StackInUse:     m.StackInuse,
	}
}

// getApplicationMetrics 获取应用指标
func (ms *MetricsService) getApplicationMetrics() ApplicationMetrics {
	metrics := ApplicationMetrics{}

	if metric, exists := ms.GetMetric("total_requests"); exists {
		metrics.TotalRequests = int64(metric.Value)
	}
	if metric, exists := ms.GetMetric("active_requests"); exists {
		metrics.ActiveRequests = int64(metric.Value)
	}
	if metric, exists := ms.GetMetric("requests_per_second"); exists {
		metrics.RequestsPerSecond = metric.Value
	}
	if metric, exists := ms.GetMetric("average_response_time"); exists {
		metrics.AverageResponseTime = metric.Value
	}
	if metric, exists := ms.GetMetric("error_rate"); exists {
		metrics.ErrorRate = metric.Value
	}
	if metric, exists := ms.GetMetric("total_conversions"); exists {
		metrics.TotalConversions = int64(metric.Value)
	}
	if metric, exists := ms.GetMetric("active_conversions"); exists {
		metrics.ActiveConversions = int64(metric.Value)
	}
	if metric, exists := ms.GetMetric("cache_hit_rate"); exists {
		metrics.CacheHitRate = metric.Value
	}

	return metrics
}

// generateAlerts 生成告警
func (ms *MetricsService) generateAlerts() []Alert {
	var alerts []Alert
	now := time.Now()

	// CPU使用率告警
	if metric, exists := ms.GetMetric("cpu_usage"); exists {
		if metric.Value > 80 {
			alerts = append(alerts, Alert{
				Level:     AlertLevelCritical,
				Metric:    "cpu_usage",
				Value:     metric.Value,
				Threshold: 80,
				Message:   fmt.Sprintf("CPU使用率过高: %.2f%%", metric.Value),
				Timestamp: now,
			})
		} else if metric.Value > 60 {
			alerts = append(alerts, Alert{
				Level:     AlertLevelWarning,
				Metric:    "cpu_usage",
				Value:     metric.Value,
				Threshold: 60,
				Message:   fmt.Sprintf("CPU使用率较高: %.2f%%", metric.Value),
				Timestamp: now,
			})
		}
	}

	// 内存使用率告警
	if metric, exists := ms.GetMetric("memory_usage"); exists {
		if metric.Value > 85 {
			alerts = append(alerts, Alert{
				Level:     AlertLevelCritical,
				Metric:    "memory_usage",
				Value:     metric.Value,
				Threshold: 85,
				Message:   fmt.Sprintf("内存使用率过高: %.2f%%", metric.Value),
				Timestamp: now,
			})
		}
	}

	// 错误率告警
	if metric, exists := ms.GetMetric("error_rate"); exists {
		if metric.Value > 5 {
			alerts = append(alerts, Alert{
				Level:     AlertLevelCritical,
				Metric:    "error_rate",
				Value:     metric.Value,
				Threshold: 5,
				Message:   fmt.Sprintf("错误率过高: %.2f%%", metric.Value),
				Timestamp: now,
			})
		}
	}

	return alerts
}

// generateRecommendations 生成建议
func (ms *MetricsService) generateRecommendations(report *PerformanceReport) []string {
	var recommendations []string

	// 基于内存使用率的建议
	if report.SystemMetrics.MemoryUsage > 80 {
		recommendations = append(recommendations, "内存使用率过高，建议增加内存或优化内存使用")
	}

	// 基于Goroutine数量的建议
	if report.SystemMetrics.Goroutines > 1000 {
		recommendations = append(recommendations, "Goroutine数量过多，可能存在goroutine泄漏")
	}

	// 基于错误率的建议
	if report.ApplicationMetrics.ErrorRate > 1 {
		recommendations = append(recommendations, "错误率较高，建议检查应用逻辑和错误处理")
	}

	// 基于响应时间的建议
	if report.ApplicationMetrics.AverageResponseTime > 1000 {
		recommendations = append(recommendations, "平均响应时间过长，建议优化性能")
	}

	// 基于缓存命中率的建议
	if report.ApplicationMetrics.CacheHitRate < 80 {
		recommendations = append(recommendations, "缓存命中率较低，建议优化缓存策略")
	}

	return recommendations
}

// ExportMetrics 导出指标（Prometheus格式）
func (ms *MetricsService) ExportMetrics() string {
	var result string
	for name, metric := range ms.GetAllMetrics() {
		result += fmt.Sprintf("# HELP %s %s\n", name, metric.Description)
		result += fmt.Sprintf("# TYPE %s %s\n", name, metric.Type)
		result += fmt.Sprintf("%s %.2f %d\n", name, metric.Value, metric.Timestamp.Unix())
	}
	return result
}

// ExportMetricsJSON 导出指标（JSON格式）
func (ms *MetricsService) ExportMetricsJSON() ([]byte, error) {
	return json.MarshalIndent(ms.GetAllMetrics(), "", "  ")
}