package middleware

import (
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"runtime/debug"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// CORS 跨域中间件
func CORS() gin.HandlerFunc {
	return func(c *gin.Context) {
		method := c.Request.Method
		origin := c.Request.Header.Get("Origin")

		// 设置CORS头
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Origin, X-Requested-With, Content-Type, Accept, Authorization, Cache-Control, X-File-Name")
		c.Header("Access-Control-Expose-Headers", "Content-Length, Access-Control-Allow-Origin, Access-Control-Allow-Headers, Content-Type")
		c.Header("Access-Control-Allow-Credentials", "true")

		// 处理预检请求
		if method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		log.Printf("CORS: Origin=%s, Method=%s", origin, method)
		c.Next()
	}
}

// Logger 日志中间件
func Logger() gin.HandlerFunc {
	return gin.LoggerWithFormatter(func(param gin.LogFormatterParams) string {
		return fmt.Sprintf("%s - [%s] \"%s %s %s %d %s \"%s\" %s\"\n",
			param.ClientIP,
			param.TimeStamp.Format(time.RFC1123),
			param.Method,
			param.Path,
			param.Request.Proto,
			param.StatusCode,
			param.Latency,
			param.Request.UserAgent(),
			param.ErrorMessage,
		)
	})
}

// Recovery 错误恢复中间件
func Recovery() gin.HandlerFunc {
	return gin.CustomRecovery(func(c *gin.Context, recovered interface{}) {
		log.Printf("Panic recovered: %v\n%s", recovered, debug.Stack())

		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "服务器内部错误",
			"error":   "Internal server error",
		})
		c.Abort()
	})
}

// RequestID 请求ID中间件
func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		requestID := c.GetHeader("X-Request-ID")
		if requestID == "" {
			requestID = generateRequestID()
		}
		c.Set("RequestID", requestID)
		c.Header("X-Request-ID", requestID)
		c.Next()
	}
}

// RateLimiter 简单的速率限制中间件
func RateLimiter(maxRequests int, duration time.Duration) gin.HandlerFunc {
	type client struct {
		count     int
		lastReset time.Time
	}

	clients := make(map[string]*client)
	var mu sync.RWMutex

	return func(c *gin.Context) {
		clientIP := c.ClientIP()
		now := time.Now()

		mu.Lock()
		cli, exists := clients[clientIP]
		if !exists {
			cli = &client{
				count:     1,
				lastReset: now,
			}
			clients[clientIP] = cli
		} else {
			// 检查是否需要重置计数器
			if now.Sub(cli.lastReset) > duration {
				cli.count = 1
				cli.lastReset = now
			} else {
				cli.count++
			}
		}
		mu.Unlock()

		if cli.count > maxRequests {
			c.JSON(http.StatusTooManyRequests, gin.H{
				"code":    429,
				"message": "请求过于频繁，请稍后再试",
				"error":   "Too many requests",
			})
			c.Abort()
			return
		}

		c.Next()
	}
}

// JWTAuth JWT认证中间件
func JWTAuth(authService interface{}) gin.HandlerFunc {
	return func(c *gin.Context) {
		// 从请求头获取token
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, gin.H{
				"code":    401,
				"message": "缺少认证token",
				"error":   "Authorization header required",
			})
			c.Abort()
			return
		}

		// 检查Bearer前缀
		tokenString := ""
		if len(authHeader) > 7 && authHeader[:7] == "Bearer " {
			tokenString = authHeader[7:]
		} else {
			c.JSON(http.StatusUnauthorized, gin.H{
				"code":    401,
				"message": "无效的认证格式",
				"error":   "Invalid authorization format",
			})
			c.Abort()
			return
		}

		// 这里需要类型断言，实际使用时需要传入正确的AuthService
		// 由于Go的类型系统限制，这里使用interface{}，在实际使用时需要进行类型转换
		// 验证token的逻辑需要在具体的handler中实现
		c.Set("token", tokenString)
		c.Next()
	}
}

// RequireRole 角色权限中间件
func RequireRole(roles ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		// 从上下文获取用户角色
		userRole, exists := c.Get("user_role")
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{
				"code":    401,
				"message": "用户未认证",
				"error":   "User not authenticated",
			})
			c.Abort()
			return
		}

		// 检查角色权限
		role := userRole.(string)
		for _, requiredRole := range roles {
			if role == requiredRole {
				c.Next()
				return
			}
		}

		c.JSON(http.StatusForbidden, gin.H{
			"code":    403,
			"message": "权限不足",
			"error":   "Insufficient permissions",
		})
		c.Abort()
	}
}

// generateRequestID 生成请求ID
func generateRequestID() string {
	return fmt.Sprintf("%d-%d", time.Now().UnixNano(), rand.Intn(10000))
}
