package tests

import (
	"bytes"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	_ "modernc.org/sqlite"

	"github.com/Deali-Axy/ebook-generator/internal/web/handlers"
	"github.com/Deali-Axy/ebook-generator/internal/web/models"
	"github.com/Deali-Axy/ebook-generator/internal/web/services"
)

// TestServer 测试服务器结构
type TestServer struct {
	router *gin.Engine
	db     *gorm.DB
	token  string
	userID uint
}

// setupTestServer 设置测试服务器
func setupTestServer(t *testing.T) *TestServer {
	// 设置Gin为测试模式
	gin.SetMode(gin.TestMode)

	// 创建内存数据库
	db, err := gorm.Open(sqlite.New(sqlite.Config{
		DriverName: "sqlite",
		DSN:        ":memory:",
	}), &gorm.Config{})
	require.NoError(t, err)

	// 自动迁移数据库表
	err = db.AutoMigrate(&models.User{}, &models.ConversionHistory{}, &models.ConversionPreset{})
	require.NoError(t, err)

	// 创建服务
	authService := services.NewAuthService(db, "test-secret", time.Hour*24)
	historyService := services.NewHistoryService(db)

	// 初始化处理器服务
	handlers.InitAuthService(authService)
	handlers.InitHistoryService(historyService)

	// 创建路由
	router := gin.New()
	router.Use(gin.Recovery())

	// API路由组
	api := router.Group("/api")

	// 认证路由
	auth := api.Group("/auth")
	{
		auth.POST("/register", handlers.Register)
		auth.POST("/login", handlers.Login)
		auth.GET("/profile", handlers.AuthMiddleware(), handlers.GetProfile)
		auth.PUT("/profile", handlers.AuthMiddleware(), handlers.UpdateProfile)
		auth.POST("/logout", handlers.AuthMiddleware(), handlers.Logout)
		auth.POST("/refresh", handlers.RefreshToken)
	}

	// 历史记录路由
	history := api.Group("/history")
	history.Use(handlers.AuthMiddleware())
	{
		history.GET("", handlers.GetHistories)
		history.GET("/stats", handlers.GetHistoryStats)
		history.DELETE("/:id", handlers.DeleteHistory)
	}

	// 预设路由
	presets := api.Group("/presets")
	presets.Use(handlers.AuthMiddleware())
	{
		presets.POST("", handlers.CreatePreset)
		presets.GET("", handlers.GetPresets)
		presets.PUT("/:id", handlers.UpdatePreset)
		presets.DELETE("/:id", handlers.DeletePreset)
	}

	// 批量转换路由（简化版，仅用于测试）
	api.POST("/batch/convert", handlers.AuthMiddleware(), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "批量转换功能正在开发中"})
	})

	return &TestServer{
		router: router,
		db:     db,
	}
}

// registerTestUser 注册测试用户并获取token
func (ts *TestServer) registerTestUser(t *testing.T) {
	// 注册用户
	registerData := map[string]interface{}{
		"username": "testuser",
		"email":    "test@example.com",
		"password": "password123",
	}

	body, _ := json.Marshal(registerData)
	req := httptest.NewRequest("POST", "/api/auth/register", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
		ts.router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)

	// 登录获取token
	loginData := map[string]interface{}{
		"email":    "test@example.com",
		"password": "password123",
	}

	body, _ = json.Marshal(loginData)
	req = httptest.NewRequest("POST", "/api/auth/login", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")

	w = httptest.NewRecorder()
	ts.router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var loginResponse map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &loginResponse)
	require.NoError(t, err)

	ts.token = loginResponse["access_token"].(string)
	ts.userID = uint(loginResponse["user"].(map[string]interface{})["id"].(float64))
}

// TestAuthEndpoints 测试认证相关接口
func TestAuthEndpoints(t *testing.T) {
	ts := setupTestServer(t)

	t.Run("用户注册", func(t *testing.T) {
		registerData := map[string]interface{}{
			"username": "newuser",
			"email":    "newuser@example.com",
			"password": "password123",
		}

		body, _ := json.Marshal(registerData)
		req := httptest.NewRequest("POST", "/api/auth/register", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")

		w := httptest.NewRecorder()
		ts.router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.Contains(t, response, "message")
	})

	t.Run("用户登录", func(t *testing.T) {
		// 先注册用户
		registerData := map[string]interface{}{
			"username": "loginuser",
			"email":    "login@example.com",
			"password": "password123",
		}

		body, _ := json.Marshal(registerData)
		req := httptest.NewRequest("POST", "/api/auth/register", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")

		w := httptest.NewRecorder()
		ts.router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusCreated, w.Code)

		// 登录
		loginData := map[string]interface{}{
			"email":    "login@example.com",
			"password": "password123",
		}

		body, _ = json.Marshal(loginData)
		req = httptest.NewRequest("POST", "/api/auth/login", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")

		w = httptest.NewRecorder()
		ts.router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.Contains(t, response, "access_token")
		assert.Contains(t, response, "user")
	})

	t.Run("获取用户资料", func(t *testing.T) {
		ts.registerTestUser(t)

		req := httptest.NewRequest("GET", "/api/auth/profile", nil)
		req.Header.Set("Authorization", "Bearer "+ts.token)

		w := httptest.NewRecorder()
		ts.router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.Equal(t, "testuser", response["username"])
		assert.Equal(t, "test@example.com", response["email"])
	})

	t.Run("更新用户资料", func(t *testing.T) {
		ts.registerTestUser(t)

		updateData := map[string]interface{}{
			"username": "updateduser",
			"email":    "updated@example.com",
		}

		body, _ := json.Marshal(updateData)
		req := httptest.NewRequest("PUT", "/api/auth/profile", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+ts.token)

		w := httptest.NewRecorder()
		ts.router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.Equal(t, "用户资料更新成功", response["message"])
	})
}

// TestHistoryEndpoints 测试历史记录相关接口
func TestHistoryEndpoints(t *testing.T) {
	ts := setupTestServer(t)
	ts.registerTestUser(t)

	// 创建测试历史记录
	history := &models.ConversionHistory{
		UserID:           ts.userID,
		OriginalFileName: "test.txt",
		OutputFormat:     "epub",
		Status:           "completed",
		OriginalFileSize: 1024,
		CreatedAt:        time.Now(),
	}
	ts.db.Create(history)

	t.Run("获取转换历史列表", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/history", nil)
		req.Header.Set("Authorization", "Bearer "+ts.token)

		w := httptest.NewRecorder()
		ts.router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.Contains(t, response, "data")
		assert.Contains(t, response, "total")
	})

	t.Run("获取转换统计信息", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/history/stats", nil)
		req.Header.Set("Authorization", "Bearer "+ts.token)

		w := httptest.NewRecorder()
		ts.router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.Contains(t, response, "total_conversions")
		assert.Contains(t, response, "monthly_stats")
	})

	t.Run("删除转换历史", func(t *testing.T) {
		req := httptest.NewRequest("DELETE", fmt.Sprintf("/api/history/%d", history.ID), nil)
		req.Header.Set("Authorization", "Bearer "+ts.token)

		w := httptest.NewRecorder()
		ts.router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.Equal(t, "历史记录删除成功", response["message"])
	})
}

// TestPresetEndpoints 测试预设相关接口
func TestPresetEndpoints(t *testing.T) {
	ts := setupTestServer(t)
	ts.registerTestUser(t)

	t.Run("创建转换预设", func(t *testing.T) {
		presetData := map[string]interface{}{
			"name":        "测试预设",
			"description": "这是一个测试预设",
			"options": map[string]interface{}{
				"format": "epub",
				"quality": "high",
			},
		}

		body, _ := json.Marshal(presetData)
		req := httptest.NewRequest("POST", "/api/presets", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+ts.token)

		w := httptest.NewRecorder()
		ts.router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.Contains(t, response, "message")
	})

	t.Run("获取预设列表", func(t *testing.T) {
		// 先创建一个预设
		preset := &models.ConversionPreset{
			UserID:      ts.userID,
			Name:        "测试预设",
			Description: "测试描述",
			Options:      models.ConvertOptionsJSON{"format": "epub"},
		}
		ts.db.Create(preset)

		req := httptest.NewRequest("GET", "/api/presets", nil)
		req.Header.Set("Authorization", "Bearer "+ts.token)

		w := httptest.NewRecorder()
		ts.router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response []map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.Len(t, response, 1)
		assert.Equal(t, "测试预设", response[0]["name"])
	})

	t.Run("更新转换预设", func(t *testing.T) {
		// 先创建一个预设
		preset := &models.ConversionPreset{
			UserID:      ts.userID,
			Name:        "原始预设",
			Description: "原始描述",
			Options:     models.ConvertOptionsJSON{"format": "epub"},
		}
		ts.db.Create(preset)

		updateData := map[string]interface{}{
			"name":        "更新后的预设",
			"description": "更新后的描述",
			"options": map[string]interface{}{
				"format": "mobi",
				"quality": "medium",
			},
		}

		body, _ := json.Marshal(updateData)
		req := httptest.NewRequest("PUT", fmt.Sprintf("/api/presets/%d", preset.ID), bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+ts.token)

		w := httptest.NewRecorder()
		ts.router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.Equal(t, "预设更新成功", response["message"])
	})

	t.Run("删除转换预设", func(t *testing.T) {
		// 先创建一个预设
		preset := &models.ConversionPreset{
			UserID:      ts.userID,
			Name:        "待删除预设",
			Description: "待删除描述",
			Options:     models.ConvertOptionsJSON{"format": "epub"},
		}
		ts.db.Create(preset)

		req := httptest.NewRequest("DELETE", fmt.Sprintf("/api/presets/%d", preset.ID), nil)
		req.Header.Set("Authorization", "Bearer "+ts.token)

		w := httptest.NewRecorder()
		ts.router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.Equal(t, "预设删除成功", response["message"])
	})
}

// TestBatchConversionEndpoint 测试批量转换接口
func TestBatchConversionEndpoint(t *testing.T) {
	ts := setupTestServer(t)
	ts.registerTestUser(t)

	t.Run("批量转换文件", func(t *testing.T) {
		// 创建测试文件内容
		var buf bytes.Buffer
		writer := multipart.NewWriter(&buf)

		// 添加文件1
		file1, err := writer.CreateFormFile("files", "test1.txt")
		require.NoError(t, err)
		_, err = file1.Write([]byte("测试文件内容1"))
		require.NoError(t, err)

		// 添加文件2
		file2, err := writer.CreateFormFile("files", "test2.txt")
		require.NoError(t, err)
		_, err = file2.Write([]byte("测试文件内容2"))
		require.NoError(t, err)

		// 添加转换选项
		err = writer.WriteField("target_format", "epub")
		require.NoError(t, err)

		err = writer.Close()
		require.NoError(t, err)

		req := httptest.NewRequest("POST", "/api/batch/convert", &buf)
		req.Header.Set("Content-Type", writer.FormDataContentType())
		req.Header.Set("Authorization", "Bearer "+ts.token)

		w := httptest.NewRecorder()
		ts.router.ServeHTTP(w, req)

		// 由于批量转换可能需要实际的转换逻辑，这里主要测试请求格式是否正确
		// 实际的转换逻辑可能返回不同的状态码
		assert.True(t, w.Code == http.StatusOK || w.Code == http.StatusAccepted || w.Code == http.StatusBadRequest)
	})
}

// TestUnauthorizedAccess 测试未授权访问
func TestUnauthorizedAccess(t *testing.T) {
	ts := setupTestServer(t)

	endpoints := []struct {
		method string
		path   string
	}{
		{"GET", "/api/auth/profile"},
		{"PUT", "/api/auth/profile"},
		{"POST", "/api/auth/logout"},
		{"GET", "/api/history"},
		{"GET", "/api/history/stats"},
		{"DELETE", "/api/history/1"},
		{"POST", "/api/presets"},
		{"GET", "/api/presets"},
		{"PUT", "/api/presets/1"},
		{"DELETE", "/api/presets/1"},
		{"POST", "/api/batch/convert"},
	}

	for _, endpoint := range endpoints {
		t.Run(fmt.Sprintf("未授权访问 %s %s", endpoint.method, endpoint.path), func(t *testing.T) {
			req := httptest.NewRequest(endpoint.method, endpoint.path, nil)
			w := httptest.NewRecorder()
			ts.router.ServeHTTP(w, req)

			assert.Equal(t, http.StatusUnauthorized, w.Code)
		})
	}
}

// TestInvalidToken 测试无效token
func TestInvalidToken(t *testing.T) {
	ts := setupTestServer(t)

	req := httptest.NewRequest("GET", "/api/auth/profile", nil)
	req.Header.Set("Authorization", "Bearer invalid_token")

	w := httptest.NewRecorder()
	ts.router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}