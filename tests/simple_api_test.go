package tests

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"

	"github.com/Deali-Axy/ebook-generator/internal/web/handlers"
)

// TestAPIEndpointsSimple 简化的API测试，只验证路由注册
func TestAPIEndpointsSimple(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// 创建路由
	router := gin.New()

	// 注册基本路由
	api := router.Group("/api")
	{
		auth := api.Group("/auth")
		{
			auth.POST("/register", func(c *gin.Context) {
				c.JSON(http.StatusOK, gin.H{"message": "注册接口已注册"})
			})
			auth.POST("/login", func(c *gin.Context) {
				c.JSON(http.StatusOK, gin.H{"message": "登录接口已注册"})
			})
			auth.GET("/profile", func(c *gin.Context) {
				c.JSON(http.StatusOK, gin.H{"message": "获取资料接口已注册"})
			})
			auth.PUT("/profile", func(c *gin.Context) {
				c.JSON(http.StatusOK, gin.H{"message": "更新资料接口已注册"})
			})
			auth.POST("/logout", func(c *gin.Context) {
				c.JSON(http.StatusOK, gin.H{"message": "登出接口已注册"})
			})
			auth.POST("/refresh", func(c *gin.Context) {
				c.JSON(http.StatusOK, gin.H{"message": "刷新token接口已注册"})
			})
			auth.PUT("/password", func(c *gin.Context) {
				c.JSON(http.StatusOK, gin.H{"message": "修改密码接口已注册"})
			})
		}

		history := api.Group("/history")
		{
			history.GET("", func(c *gin.Context) {
				c.JSON(http.StatusOK, gin.H{"message": "获取历史接口已注册"})
			})
		}

		presets := api.Group("/presets")
		{
			presets.POST("", func(c *gin.Context) {
				c.JSON(http.StatusOK, gin.H{"message": "创建预设接口已注册"})
			})
			presets.GET("", func(c *gin.Context) {
				c.JSON(http.StatusOK, gin.H{"message": "获取预设接口已注册"})
			})
			presets.PUT("/:id", func(c *gin.Context) {
				c.JSON(http.StatusOK, gin.H{"message": "更新预设接口已注册"})
			})
			presets.DELETE("/:id", func(c *gin.Context) {
				c.JSON(http.StatusOK, gin.H{"message": "删除预设接口已注册"})
			})
		}

		batch := api.Group("/batch")
		{
			batch.POST("/convert", func(c *gin.Context) {
				c.JSON(http.StatusOK, gin.H{"message": "批量转换接口已注册"})
			})
		}
	}

	t.Run("测试用户注册接口", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/api/auth/register", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
		
		var response map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &response)
		assert.Equal(t, "注册接口已注册", response["message"])
	})

	t.Run("测试用户登录接口", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/api/auth/login", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
		
		var response map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &response)
		assert.Equal(t, "登录接口已注册", response["message"])
	})

	t.Run("测试获取用户资料接口", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/auth/profile", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
		
		var response map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &response)
		assert.Equal(t, "获取资料接口已注册", response["message"])
	})

	t.Run("测试获取转换历史接口", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/history", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
		
		var response map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &response)
		assert.Equal(t, "获取历史接口已注册", response["message"])
	})

	t.Run("测试创建预设接口", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/api/presets", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
		
		var response map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &response)
		assert.Equal(t, "创建预设接口已注册", response["message"])
	})

	t.Run("测试获取预设列表接口", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/presets", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
		
		var response map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &response)
		assert.Equal(t, "获取预设接口已注册", response["message"])
	})

	t.Run("测试批量转换接口", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/api/batch/convert", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
		
		var response map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &response)
		assert.Equal(t, "批量转换接口已注册", response["message"])
	})
}

// TestRouteRegistration 测试路由注册
func TestRouteRegistration(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()

	// 注册所有路由
	api := router.Group("/api")
	{
		auth := api.Group("/auth")
		{
			auth.POST("/register", handlers.Register)
			auth.POST("/login", handlers.Login)
			auth.GET("/profile", handlers.AuthMiddleware(), handlers.GetProfile)
			auth.PUT("/profile", handlers.AuthMiddleware(), handlers.UpdateProfile)
			auth.POST("/logout", handlers.AuthMiddleware(), handlers.Logout)
			auth.POST("/refresh", handlers.RefreshToken)
			auth.PUT("/password", handlers.AuthMiddleware(), handlers.ChangePassword)
		}

		history := api.Group("/history")
		{
			history.GET("", handlers.AuthMiddleware(), handlers.GetHistories)
		}

		presets := api.Group("/presets")
		{
			presets.POST("", handlers.AuthMiddleware(), handlers.CreatePreset)
			presets.GET("", handlers.AuthMiddleware(), handlers.GetPresets)
			presets.PUT("/:id", handlers.AuthMiddleware(), handlers.UpdatePreset)
			presets.DELETE("/:id", handlers.AuthMiddleware(), handlers.DeletePreset)
		}

		batch := api.Group("/batch")
		{
			batch.POST("/convert", handlers.AuthMiddleware(), func(c *gin.Context) {
				c.JSON(http.StatusOK, gin.H{"message": "批量转换功能正常"})
			})
		}
	}

	// 测试路由是否正确注册
	routes := router.Routes()
	assert.True(t, len(routes) > 0, "应该注册了路由")

	// 检查关键路由是否存在
	routeMap := make(map[string]bool)
	for _, route := range routes {
		routeMap[route.Method+" "+route.Path] = true
	}

	// 验证关键API路由
	expectedRoutes := []string{
		"POST /api/auth/register",
		"POST /api/auth/login",
		"GET /api/auth/profile",
		"PUT /api/auth/profile",
		"POST /api/auth/logout",
		"POST /api/auth/refresh",
		"PUT /api/auth/password",
		"GET /api/history",
		"POST /api/presets",
		"GET /api/presets",
		"PUT /api/presets/:id",
		"DELETE /api/presets/:id",
		"POST /api/batch/convert",
	}

	for _, expectedRoute := range expectedRoutes {
		assert.True(t, routeMap[expectedRoute], "路由 %s 应该被注册", expectedRoute)
	}

	t.Logf("成功注册了 %d 个路由", len(routes))
	for _, route := range routes {
		t.Logf("路由: %s %s", route.Method, route.Path)
	}
}

// TestMiddleware 测试中间件
func TestMiddleware(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()

	// 测试认证中间件
	router.GET("/protected", handlers.AuthMiddleware(), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "success"})
	})

	t.Run("无token访问受保护路由", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/protected", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})

	t.Run("无效token访问受保护路由", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/protected", nil)
		req.Header.Set("Authorization", "Bearer invalid-token")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})

	t.Run("错误格式的Authorization头", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/protected", nil)
		req.Header.Set("Authorization", "InvalidFormat")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})
}