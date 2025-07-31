package loadbalancer

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"hash/fnv"
	"math/rand"
	"net/http"
	"net/http/httputil"
	"net/url"
	"sort"
	"sync"
	"sync/atomic"
	"time"
)

// LoadBalancer 负载均衡器
type LoadBalancer struct {
	backends    []*Backend
	algorithm   Algorithm
	healthCheck *HealthChecker
	config      LoadBalancerConfig
	mutex       sync.RWMutex
	stats       *LoadBalancerStats
	statsMutex  sync.RWMutex
	ctx         context.Context
	cancel      context.CancelFunc
	consistentHash *ConsistentHash
}

// LoadBalancerConfig 负载均衡配置
type LoadBalancerConfig struct {
	Algorithm           Algorithm     `json:"algorithm"`
	HealthCheckInterval time.Duration `json:"health_check_interval"`
	HealthCheckTimeout  time.Duration `json:"health_check_timeout"`
	HealthCheckPath     string        `json:"health_check_path"`
	MaxRetries          int           `json:"max_retries"`
	RetryDelay          time.Duration `json:"retry_delay"`
	SessionSticky       bool          `json:"session_sticky"`
	SessionTimeout      time.Duration `json:"session_timeout"`
	CircuitBreaker      CircuitBreakerConfig `json:"circuit_breaker"`
	Metrics             bool          `json:"metrics"`
	Logging             bool          `json:"logging"`
}

// CircuitBreakerConfig 熔断器配置
type CircuitBreakerConfig struct {
	Enabled           bool          `json:"enabled"`
	FailureThreshold  int           `json:"failure_threshold"`
	SuccessThreshold  int           `json:"success_threshold"`
	Timeout           time.Duration `json:"timeout"`
	HalfOpenRequests  int           `json:"half_open_requests"`
}

// Backend 后端服务器
type Backend struct {
	ID          string                 `json:"id"`
	URL         *url.URL               `json:"url"`
	Weight      int                    `json:"weight"`
	Healthy     bool                   `json:"healthy"`
	LastCheck   time.Time              `json:"last_check"`
	Connections int64                  `json:"connections"`
	ResponseTime time.Duration         `json:"response_time"`
	ErrorCount  int64                  `json:"error_count"`
	SuccessCount int64                 `json:"success_count"`
	Metadata    map[string]interface{} `json:"metadata"`
	Proxy       *httputil.ReverseProxy `json:"-"`
	CircuitBreaker *CircuitBreaker     `json:"-"`
	mutex       sync.RWMutex           `json:"-"`
}

// Algorithm 负载均衡算法
type Algorithm string

const (
	AlgorithmRoundRobin     Algorithm = "round_robin"
	AlgorithmWeightedRR     Algorithm = "weighted_round_robin"
	AlgorithmLeastConn      Algorithm = "least_connections"
	AlgorithmWeightedLC     Algorithm = "weighted_least_connections"
	AlgorithmIPHash         Algorithm = "ip_hash"
	AlgorithmConsistentHash Algorithm = "consistent_hash"
	AlgorithmRandom         Algorithm = "random"
	AlgorithmWeightedRandom Algorithm = "weighted_random"
	AlgorithmLeastTime      Algorithm = "least_time"
)

// HealthChecker 健康检查器
type HealthChecker struct {
	config LoadBalancerConfig
	client *http.Client
}

// LoadBalancerStats 负载均衡统计
type LoadBalancerStats struct {
	TotalRequests    int64 `json:"total_requests"`
	SuccessRequests  int64 `json:"success_requests"`
	FailedRequests   int64 `json:"failed_requests"`
	AverageLatency   time.Duration `json:"average_latency"`
	ActiveConnections int64 `json:"active_connections"`
	Throughput       float64 `json:"throughput"`
	LastReset        time.Time `json:"last_reset"`
}

// CircuitBreaker 熔断器
type CircuitBreaker struct {
	config       CircuitBreakerConfig
	state        CircuitState
	failureCount int64
	successCount int64
	lastFailure  time.Time
	mutex        sync.RWMutex
}

// CircuitState 熔断器状态
type CircuitState string

const (
	CircuitStateClosed   CircuitState = "closed"
	CircuitStateOpen     CircuitState = "open"
	CircuitStateHalfOpen CircuitState = "half_open"
)

// ConsistentHash 一致性哈希
type ConsistentHash struct {
	hashRing map[uint32]*Backend
	sortedHashes []uint32
	virtualNodes int
	mutex        sync.RWMutex
}

// SessionStore 会话存储
type SessionStore struct {
	sessions map[string]*Session
	mutex    sync.RWMutex
	ttl      time.Duration
}

// Session 会话
type Session struct {
	BackendID string
	CreatedAt time.Time
	LastAccess time.Time
}

// NewLoadBalancer 创建负载均衡器
func NewLoadBalancer(config LoadBalancerConfig) (*LoadBalancer, error) {
	// 设置默认值
	if config.HealthCheckInterval == 0 {
		config.HealthCheckInterval = 30 * time.Second
	}
	if config.HealthCheckTimeout == 0 {
		config.HealthCheckTimeout = 5 * time.Second
	}
	if config.HealthCheckPath == "" {
		config.HealthCheckPath = "/health"
	}
	if config.MaxRetries == 0 {
		config.MaxRetries = 3
	}
	if config.RetryDelay == 0 {
		config.RetryDelay = 1 * time.Second
	}
	if config.SessionTimeout == 0 {
		config.SessionTimeout = 30 * time.Minute
	}
	if config.Algorithm == "" {
		config.Algorithm = AlgorithmRoundRobin
	}

	// 设置熔断器默认值
	if config.CircuitBreaker.FailureThreshold == 0 {
		config.CircuitBreaker.FailureThreshold = 5
	}
	if config.CircuitBreaker.SuccessThreshold == 0 {
		config.CircuitBreaker.SuccessThreshold = 3
	}
	if config.CircuitBreaker.Timeout == 0 {
		config.CircuitBreaker.Timeout = 60 * time.Second
	}
	if config.CircuitBreaker.HalfOpenRequests == 0 {
		config.CircuitBreaker.HalfOpenRequests = 3
	}

	ctx, cancel := context.WithCancel(context.Background())

	lb := &LoadBalancer{
		backends:  make([]*Backend, 0),
		algorithm: config.Algorithm,
		config:    config,
		stats:     &LoadBalancerStats{LastReset: time.Now()},
		ctx:       ctx,
		cancel:    cancel,
	}

	// 创建健康检查器
	lb.healthCheck = &HealthChecker{
		config: config,
		client: &http.Client{
			Timeout: config.HealthCheckTimeout,
		},
	}

	// 创建一致性哈希（如果需要）
	if config.Algorithm == AlgorithmConsistentHash {
		lb.consistentHash = NewConsistentHash(150) // 150个虚拟节点
	}

	// 启动健康检查
	go lb.startHealthCheck()

	return lb, nil
}

// AddBackend 添加后端服务器
func (lb *LoadBalancer) AddBackend(backendURL string, weight int, metadata map[string]interface{}) error {
	parsedURL, err := url.Parse(backendURL)
	if err != nil {
		return fmt.Errorf("invalid backend URL: %w", err)
	}

	backendID := lb.generateBackendID(backendURL)

	backend := &Backend{
		ID:       backendID,
		URL:      parsedURL,
		Weight:   weight,
		Healthy:  true,
		Metadata: metadata,
		Proxy:    httputil.NewSingleHostReverseProxy(parsedURL),
	}

	// 创建熔断器
	if lb.config.CircuitBreaker.Enabled {
		backend.CircuitBreaker = NewCircuitBreaker(lb.config.CircuitBreaker)
	}

	lb.mutex.Lock()
	lb.backends = append(lb.backends, backend)
	lb.mutex.Unlock()

	// 添加到一致性哈希
	if lb.consistentHash != nil {
		lb.consistentHash.AddBackend(backend)
	}

	return nil
}

// RemoveBackend 移除后端服务器
func (lb *LoadBalancer) RemoveBackend(backendID string) error {
	lb.mutex.Lock()
	defer lb.mutex.Unlock()

	for i, backend := range lb.backends {
		if backend.ID == backendID {
			// 从一致性哈希中移除
			if lb.consistentHash != nil {
				lb.consistentHash.RemoveBackend(backend)
			}

			// 从切片中移除
			lb.backends = append(lb.backends[:i], lb.backends[i+1:]...)
			return nil
		}
	}

	return fmt.Errorf("backend not found: %s", backendID)
}

// GetBackend 根据算法选择后端服务器
func (lb *LoadBalancer) GetBackend(req *http.Request) (*Backend, error) {
	lb.mutex.RLock()
	defer lb.mutex.RUnlock()

	// 过滤健康的后端
	healthyBackends := make([]*Backend, 0)
	for _, backend := range lb.backends {
		backend.mutex.RLock()
		if backend.Healthy && (backend.CircuitBreaker == nil || backend.CircuitBreaker.CanRequest()) {
			healthyBackends = append(healthyBackends, backend)
		}
		backend.mutex.RUnlock()
	}

	if len(healthyBackends) == 0 {
		return nil, fmt.Errorf("no healthy backends available")
	}

	// 根据算法选择后端
	switch lb.algorithm {
	case AlgorithmRoundRobin:
		return lb.roundRobin(healthyBackends), nil
	case AlgorithmWeightedRR:
		return lb.weightedRoundRobin(healthyBackends), nil
	case AlgorithmLeastConn:
		return lb.leastConnections(healthyBackends), nil
	case AlgorithmWeightedLC:
		return lb.weightedLeastConnections(healthyBackends), nil
	case AlgorithmIPHash:
		return lb.ipHash(healthyBackends, req), nil
	case AlgorithmConsistentHash:
		return lb.consistentHashSelect(req), nil
	case AlgorithmRandom:
		return lb.random(healthyBackends), nil
	case AlgorithmWeightedRandom:
		return lb.weightedRandom(healthyBackends), nil
	case AlgorithmLeastTime:
		return lb.leastTime(healthyBackends), nil
	default:
		return lb.roundRobin(healthyBackends), nil
	}
}

// ServeHTTP 处理HTTP请求
func (lb *LoadBalancer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	start := time.Now()

	// 更新统计
	atomic.AddInt64(&lb.stats.TotalRequests, 1)
	atomic.AddInt64(&lb.stats.ActiveConnections, 1)
	defer atomic.AddInt64(&lb.stats.ActiveConnections, -1)

	// 选择后端
	backend, err := lb.GetBackend(r)
	if err != nil {
		http.Error(w, "Service Unavailable", http.StatusServiceUnavailable)
		atomic.AddInt64(&lb.stats.FailedRequests, 1)
		return
	}

	// 增加连接数
	atomic.AddInt64(&backend.Connections, 1)
	defer atomic.AddInt64(&backend.Connections, -1)

	// 代理请求
	backend.Proxy.ServeHTTP(w, r)

	// 更新统计
	duration := time.Since(start)
	atomic.AddInt64(&lb.stats.SuccessRequests, 1)
	atomic.AddInt64(&backend.SuccessCount, 1)

	// 更新响应时间
	backend.mutex.Lock()
	backend.ResponseTime = duration
	backend.mutex.Unlock()

	// 更新熔断器
	if backend.CircuitBreaker != nil {
		backend.CircuitBreaker.RecordSuccess()
	}
}

// 轮询算法
func (lb *LoadBalancer) roundRobin(backends []*Backend) *Backend {
	if len(backends) == 0 {
		return nil
	}

	// 简单的轮询实现
	index := int(atomic.LoadInt64(&lb.stats.TotalRequests)) % len(backends)
	return backends[index]
}

// 加权轮询算法
func (lb *LoadBalancer) weightedRoundRobin(backends []*Backend) *Backend {
	if len(backends) == 0 {
		return nil
	}

	// 计算总权重
	totalWeight := 0
	for _, backend := range backends {
		totalWeight += backend.Weight
	}

	if totalWeight == 0 {
		return backends[0]
	}

	// 根据权重选择
	random := rand.Intn(totalWeight)
	currentWeight := 0
	for _, backend := range backends {
		currentWeight += backend.Weight
		if random < currentWeight {
			return backend
		}
	}

	return backends[0]
}

// 最少连接算法
func (lb *LoadBalancer) leastConnections(backends []*Backend) *Backend {
	if len(backends) == 0 {
		return nil
	}

	minConnections := atomic.LoadInt64(&backends[0].Connections)
	selected := backends[0]

	for _, backend := range backends[1:] {
		connections := atomic.LoadInt64(&backend.Connections)
		if connections < minConnections {
			minConnections = connections
			selected = backend
		}
	}

	return selected
}

// 加权最少连接算法
func (lb *LoadBalancer) weightedLeastConnections(backends []*Backend) *Backend {
	if len(backends) == 0 {
		return nil
	}

	minRatio := float64(atomic.LoadInt64(&backends[0].Connections)) / float64(backends[0].Weight)
	selected := backends[0]

	for _, backend := range backends[1:] {
		if backend.Weight == 0 {
			continue
		}
		ratio := float64(atomic.LoadInt64(&backend.Connections)) / float64(backend.Weight)
		if ratio < minRatio {
			minRatio = ratio
			selected = backend
		}
	}

	return selected
}

// IP哈希算法
func (lb *LoadBalancer) ipHash(backends []*Backend, req *http.Request) *Backend {
	if len(backends) == 0 {
		return nil
	}

	clientIP := lb.getClientIP(req)
	hash := fnv.New32a()
	hash.Write([]byte(clientIP))
	index := int(hash.Sum32()) % len(backends)

	return backends[index]
}

// 一致性哈希算法
func (lb *LoadBalancer) consistentHashSelect(req *http.Request) *Backend {
	if lb.consistentHash == nil {
		return nil
	}

	clientIP := lb.getClientIP(req)
	return lb.consistentHash.GetBackend(clientIP)
}

// 随机算法
func (lb *LoadBalancer) random(backends []*Backend) *Backend {
	if len(backends) == 0 {
		return nil
	}

	index := rand.Intn(len(backends))
	return backends[index]
}

// 加权随机算法
func (lb *LoadBalancer) weightedRandom(backends []*Backend) *Backend {
	return lb.weightedRoundRobin(backends) // 复用加权轮询的逻辑
}

// 最短响应时间算法
func (lb *LoadBalancer) leastTime(backends []*Backend) *Backend {
	if len(backends) == 0 {
		return nil
	}

	backends[0].mutex.RLock()
	minTime := backends[0].ResponseTime
	backends[0].mutex.RUnlock()
	selected := backends[0]

	for _, backend := range backends[1:] {
		backend.mutex.RLock()
		responseTime := backend.ResponseTime
		backend.mutex.RUnlock()

		if responseTime < minTime {
			minTime = responseTime
			selected = backend
		}
	}

	return selected
}

// getClientIP 获取客户端IP
func (lb *LoadBalancer) getClientIP(req *http.Request) string {
	// 检查X-Forwarded-For头部
	if xff := req.Header.Get("X-Forwarded-For"); xff != "" {
		return xff
	}

	// 检查X-Real-IP头部
	if xri := req.Header.Get("X-Real-IP"); xri != "" {
		return xri
	}

	// 使用RemoteAddr
	return req.RemoteAddr
}

// startHealthCheck 启动健康检查
func (lb *LoadBalancer) startHealthCheck() {
	ticker := time.NewTicker(lb.config.HealthCheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-lb.ctx.Done():
			return
		case <-ticker.C:
			lb.performHealthCheck()
		}
	}
}

// performHealthCheck 执行健康检查
func (lb *LoadBalancer) performHealthCheck() {
	lb.mutex.RLock()
	backends := make([]*Backend, len(lb.backends))
	copy(backends, lb.backends)
	lb.mutex.RUnlock()

	for _, backend := range backends {
		go lb.checkBackendHealth(backend)
	}
}

// checkBackendHealth 检查后端健康状态
func (lb *LoadBalancer) checkBackendHealth(backend *Backend) {
	healthURL := backend.URL.String() + lb.config.HealthCheckPath
	resp, err := lb.healthCheck.client.Get(healthURL)

	backend.mutex.Lock()
	defer backend.mutex.Unlock()

	backend.LastCheck = time.Now()

	if err != nil || resp.StatusCode != http.StatusOK {
		backend.Healthy = false
		atomic.AddInt64(&backend.ErrorCount, 1)
		if backend.CircuitBreaker != nil {
			backend.CircuitBreaker.RecordFailure()
		}
	} else {
		backend.Healthy = true
		atomic.AddInt64(&backend.SuccessCount, 1)
		if backend.CircuitBreaker != nil {
			backend.CircuitBreaker.RecordSuccess()
		}
	}

	if resp != nil {
		resp.Body.Close()
	}
}

// GetBackends 获取所有后端服务器
func (lb *LoadBalancer) GetBackends() []*Backend {
	lb.mutex.RLock()
	defer lb.mutex.RUnlock()

	backends := make([]*Backend, len(lb.backends))
	copy(backends, lb.backends)
	return backends
}

// GetStats 获取统计信息
func (lb *LoadBalancer) GetStats() *LoadBalancerStats {
	lb.statsMutex.RLock()
	defer lb.statsMutex.RUnlock()

	stats := *lb.stats
	return &stats
}

// generateBackendID 生成后端ID
func (lb *LoadBalancer) generateBackendID(url string) string {
	hash := md5.Sum([]byte(url + fmt.Sprintf("%d", time.Now().UnixNano())))
	return hex.EncodeToString(hash[:])
}

// Stop 停止负载均衡器
func (lb *LoadBalancer) Stop() {
	lb.cancel()
}

// NewConsistentHash 创建一致性哈希
func NewConsistentHash(virtualNodes int) *ConsistentHash {
	return &ConsistentHash{
		hashRing:     make(map[uint32]*Backend),
		sortedHashes: make([]uint32, 0),
		virtualNodes: virtualNodes,
	}
}

// AddBackend 添加后端到一致性哈希
func (ch *ConsistentHash) AddBackend(backend *Backend) {
	ch.mutex.Lock()
	defer ch.mutex.Unlock()

	for i := 0; i < ch.virtualNodes; i++ {
		hash := ch.hash(fmt.Sprintf("%s:%d", backend.ID, i))
		ch.hashRing[hash] = backend
		ch.sortedHashes = append(ch.sortedHashes, hash)
	}

	sort.Slice(ch.sortedHashes, func(i, j int) bool {
		return ch.sortedHashes[i] < ch.sortedHashes[j]
	})
}

// RemoveBackend 从一致性哈希中移除后端
func (ch *ConsistentHash) RemoveBackend(backend *Backend) {
	ch.mutex.Lock()
	defer ch.mutex.Unlock()

	for i := 0; i < ch.virtualNodes; i++ {
		hash := ch.hash(fmt.Sprintf("%s:%d", backend.ID, i))
		delete(ch.hashRing, hash)

		// 从排序数组中移除
		for j, h := range ch.sortedHashes {
			if h == hash {
				ch.sortedHashes = append(ch.sortedHashes[:j], ch.sortedHashes[j+1:]...)
				break
			}
		}
	}
}

// GetBackend 根据键获取后端
func (ch *ConsistentHash) GetBackend(key string) *Backend {
	ch.mutex.RLock()
	defer ch.mutex.RUnlock()

	if len(ch.sortedHashes) == 0 {
		return nil
	}

	hash := ch.hash(key)

	// 二分查找
	idx := sort.Search(len(ch.sortedHashes), func(i int) bool {
		return ch.sortedHashes[i] >= hash
	})

	// 如果没找到，使用第一个
	if idx == len(ch.sortedHashes) {
		idx = 0
	}

	return ch.hashRing[ch.sortedHashes[idx]]
}

// hash 计算哈希值
func (ch *ConsistentHash) hash(key string) uint32 {
	h := fnv.New32a()
	h.Write([]byte(key))
	return h.Sum32()
}

// NewCircuitBreaker 创建熔断器
func NewCircuitBreaker(config CircuitBreakerConfig) *CircuitBreaker {
	return &CircuitBreaker{
		config: config,
		state:  CircuitStateClosed,
	}
}

// CanRequest 检查是否可以发送请求
func (cb *CircuitBreaker) CanRequest() bool {
	cb.mutex.RLock()
	defer cb.mutex.RUnlock()

	switch cb.state {
	case CircuitStateClosed:
		return true
	case CircuitStateOpen:
		// 检查是否可以进入半开状态
		return time.Since(cb.lastFailure) > cb.config.Timeout
	case CircuitStateHalfOpen:
		// 半开状态下限制请求数量
		return cb.successCount < int64(cb.config.HalfOpenRequests)
	default:
		return false
	}
}

// RecordSuccess 记录成功
func (cb *CircuitBreaker) RecordSuccess() {
	cb.mutex.Lock()
	defer cb.mutex.Unlock()

	cb.successCount++

	switch cb.state {
	case CircuitStateHalfOpen:
		if cb.successCount >= int64(cb.config.SuccessThreshold) {
			cb.state = CircuitStateClosed
			cb.failureCount = 0
			cb.successCount = 0
		}
	case CircuitStateClosed:
		cb.failureCount = 0
	}
}

// RecordFailure 记录失败
func (cb *CircuitBreaker) RecordFailure() {
	cb.mutex.Lock()
	defer cb.mutex.Unlock()

	cb.failureCount++
	cb.lastFailure = time.Now()

	switch cb.state {
	case CircuitStateClosed:
		if cb.failureCount >= int64(cb.config.FailureThreshold) {
			cb.state = CircuitStateOpen
			cb.successCount = 0
		}
	case CircuitStateHalfOpen:
		cb.state = CircuitStateOpen
		cb.successCount = 0
	}
}

// GetState 获取熔断器状态
func (cb *CircuitBreaker) GetState() CircuitState {
	cb.mutex.RLock()
	defer cb.mutex.RUnlock()
	return cb.state
}