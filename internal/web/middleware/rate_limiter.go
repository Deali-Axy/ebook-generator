package middleware

import (
	"fmt"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// RateLimiterConfig 限流配置
type RateLimiterConfig struct {
	RequestsPerMinute int           // 每分钟请求数
	BurstSize         int           // 突发请求数
	WindowSize        time.Duration // 时间窗口大小
	KeyFunc           func(*gin.Context) string // 获取限流key的函数
}

// ClientInfo 客户端信息
type ClientInfo struct {
	Requests  int       // 请求计数
	LastReset time.Time // 上次重置时间
	Tokens    int       // 令牌数量（令牌桶算法）
	LastRefill time.Time // 上次补充令牌时间
}

// AdvancedRateLimiter 高级限流器
type AdvancedRateLimiter struct {
	config  RateLimiterConfig
	clients map[string]*ClientInfo
	mutex   sync.RWMutex
}

// NewAdvancedRateLimiter 创建高级限流器
func NewAdvancedRateLimiter(config RateLimiterConfig) *AdvancedRateLimiter {
	if config.KeyFunc == nil {
		config.KeyFunc = func(c *gin.Context) string {
			return c.ClientIP()
		}
	}
	if config.WindowSize == 0 {
		config.WindowSize = time.Minute
	}
	if config.BurstSize == 0 {
		config.BurstSize = config.RequestsPerMinute
	}

	rl := &AdvancedRateLimiter{
		config:  config,
		clients: make(map[string]*ClientInfo),
	}

	// 启动清理goroutine
	go rl.cleanup()

	return rl
}

// Allow 检查是否允许请求
func (rl *AdvancedRateLimiter) Allow(key string) bool {
	rl.mutex.Lock()
	defer rl.mutex.Unlock()

	now := time.Now()
	client, exists := rl.clients[key]

	if !exists {
		// 新客户端
		rl.clients[key] = &ClientInfo{
			Requests:   1,
			LastReset:  now,
			Tokens:     rl.config.BurstSize - 1,
			LastRefill: now,
		}
		return true
	}

	// 使用令牌桶算法
	return rl.tokenBucketAllow(client, now)
}

// tokenBucketAllow 令牌桶算法
func (rl *AdvancedRateLimiter) tokenBucketAllow(client *ClientInfo, now time.Time) bool {
	// 计算需要补充的令牌数
	elapsed := now.Sub(client.LastRefill)
	tokensToAdd := int(elapsed.Seconds()) * rl.config.RequestsPerMinute / 60

	if tokensToAdd > 0 {
		client.Tokens += tokensToAdd
		if client.Tokens > rl.config.BurstSize {
			client.Tokens = rl.config.BurstSize
		}
		client.LastRefill = now
	}

	// 检查是否有可用令牌
	if client.Tokens > 0 {
		client.Tokens--
		client.Requests++
		return true
	}

	return false
}

// cleanup 清理过期的客户端信息
func (rl *AdvancedRateLimiter) cleanup() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		rl.mutex.Lock()
		now := time.Now()
		for key, client := range rl.clients {
			if now.Sub(client.LastReset) > 10*time.Minute {
				delete(rl.clients, key)
			}
		}
		rl.mutex.Unlock()
	}
}

// Middleware 返回Gin中间件
func (rl *AdvancedRateLimiter) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		key := rl.config.KeyFunc(c)
		if !rl.Allow(key) {
			c.Header("X-RateLimit-Limit", strconv.Itoa(rl.config.RequestsPerMinute))
			c.Header("X-RateLimit-Remaining", "0")
			c.Header("X-RateLimit-Reset", strconv.FormatInt(time.Now().Add(rl.config.WindowSize).Unix(), 10))
			
			c.JSON(http.StatusTooManyRequests, gin.H{
				"code":    429,
				"message": "请求过于频繁，请稍后再试",
				"error":   "Rate limit exceeded",
			})
			c.Abort()
			return
		}

		// 设置限流相关的响应头
		rl.mutex.RLock()
		client := rl.clients[key]
		remaining := client.Tokens
		rl.mutex.RUnlock()

		c.Header("X-RateLimit-Limit", strconv.Itoa(rl.config.RequestsPerMinute))
		c.Header("X-RateLimit-Remaining", strconv.Itoa(remaining))
		c.Header("X-RateLimit-Reset", strconv.FormatInt(time.Now().Add(rl.config.WindowSize).Unix(), 10))

		c.Next()
	}
}

// IPBasedRateLimiter 基于IP的限流中间件
func IPBasedRateLimiter(requestsPerMinute, burstSize int) gin.HandlerFunc {
	config := RateLimiterConfig{
		RequestsPerMinute: requestsPerMinute,
		BurstSize:         burstSize,
		WindowSize:        time.Minute,
		KeyFunc: func(c *gin.Context) string {
			return c.ClientIP()
		},
	}
	return NewAdvancedRateLimiter(config).Middleware()
}

// UserBasedRateLimiter 基于用户的限流中间件
func UserBasedRateLimiter(requestsPerMinute, burstSize int) gin.HandlerFunc {
	config := RateLimiterConfig{
		RequestsPerMinute: requestsPerMinute,
		BurstSize:         burstSize,
		WindowSize:        time.Minute,
		KeyFunc: func(c *gin.Context) string {
			// 优先使用用户ID，如果没有则使用IP
			if userID, exists := c.Get("user_id"); exists {
				return fmt.Sprintf("user:%v", userID)
			}
			return fmt.Sprintf("ip:%s", c.ClientIP())
		},
	}
	return NewAdvancedRateLimiter(config).Middleware()
}

// APIKeyBasedRateLimiter 基于API Key的限流中间件
func APIKeyBasedRateLimiter(requestsPerMinute, burstSize int) gin.HandlerFunc {
	config := RateLimiterConfig{
		RequestsPerMinute: requestsPerMinute,
		BurstSize:         burstSize,
		WindowSize:        time.Minute,
		KeyFunc: func(c *gin.Context) string {
			apiKey := c.GetHeader("X-API-Key")
			if apiKey != "" {
				return fmt.Sprintf("apikey:%s", apiKey)
			}
			return fmt.Sprintf("ip:%s", c.ClientIP())
		},
	}
	return NewAdvancedRateLimiter(config).Middleware()
}