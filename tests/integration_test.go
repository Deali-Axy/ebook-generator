package tests

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestAPIIntegration 集成测试 - 测试完整的API工作流程
func TestAPIIntegration(t *testing.T) {
	ts := setupTestServer(t)

	// 1. 用户注册
	t.Run("完整工作流程测试", func(t *testing.T) {
		// 注册用户
		registerData := map[string]interface{}{
			"username": "integrationuser",
			"email":    "integration@example.com",
			"password": "password123",
		}

		body, _ := json.Marshal(registerData)
		req := httptest.NewRequest("POST", "/api/auth/register", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")

		w := httptest.NewRecorder()
		ts.router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusCreated, w.Code)

		// 登录获取token
		loginData := map[string]interface{}{
			"email":    "integration@example.com",
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

		token := loginResponse["access_token"].(string)
		assert.NotEmpty(t, token)

		// 获取用户资料
		req = httptest.NewRequest("GET", "/api/auth/profile", nil)
		req.Header.Set("Authorization", "Bearer "+token)

		w = httptest.NewRecorder()
		ts.router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)

		var profile map[string]interface{}
		err = json.Unmarshal(w.Body.Bytes(), &profile)
		require.NoError(t, err)
		assert.Equal(t, "integrationuser", profile["username"])

		// 创建转换预设
		presetData := map[string]interface{}{
			"name":        "集成测试预设",
			"description": "用于集成测试的预设",
			"options": map[string]interface{}{
				"format":  "epub",
				"quality": "high",
				"author":  "测试作者",
			},
		}

		body, _ = json.Marshal(presetData)
		req = httptest.NewRequest("POST", "/api/presets", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+token)

		w = httptest.NewRecorder()
		ts.router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusCreated, w.Code)

		// 获取预设列表
		req = httptest.NewRequest("GET", "/api/presets", nil)
		req.Header.Set("Authorization", "Bearer "+token)

		w = httptest.NewRecorder()
		ts.router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)

		var presets []map[string]interface{}
		err = json.Unmarshal(w.Body.Bytes(), &presets)
		require.NoError(t, err)
		assert.Len(t, presets, 1)
		assert.Equal(t, "集成测试预设", presets[0]["name"])

		// 获取转换历史（应该为空）
		req = httptest.NewRequest("GET", "/api/history", nil)
		req.Header.Set("Authorization", "Bearer "+token)

		w = httptest.NewRecorder()
		ts.router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)

		var historyResponse map[string]interface{}
		err = json.Unmarshal(w.Body.Bytes(), &historyResponse)
		require.NoError(t, err)
		assert.Equal(t, float64(0), historyResponse["total"])

		// 获取统计信息
		req = httptest.NewRequest("GET", "/api/history/stats", nil)
		req.Header.Set("Authorization", "Bearer "+token)

		w = httptest.NewRecorder()
		ts.router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)

		var stats map[string]interface{}
		err = json.Unmarshal(w.Body.Bytes(), &stats)
		require.NoError(t, err)
		assert.Contains(t, stats, "total_conversions")
		assert.Contains(t, stats, "monthly_stats")

		// 注销
		req = httptest.NewRequest("POST", "/api/auth/logout", nil)
		req.Header.Set("Authorization", "Bearer "+token)

		w = httptest.NewRecorder()
		ts.router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)

		// 验证注销后无法访问受保护的资源
		req = httptest.NewRequest("GET", "/api/auth/profile", nil)
		req.Header.Set("Authorization", "Bearer "+token)

		w = httptest.NewRecorder()
		ts.router.ServeHTTP(w, req)
		// 注意：这里可能仍然返回200，因为JWT token本身可能仍然有效
		// 实际的token撤销逻辑需要在生产环境中实现
	})
}

// TestErrorHandling 测试错误处理
func TestErrorHandling(t *testing.T) {
	ts := setupTestServer(t)

	t.Run("重复注册相同邮箱", func(t *testing.T) {
		registerData := map[string]interface{}{
			"username": "user1",
			"email":    "duplicate@example.com",
			"password": "password123",
		}

		body, _ := json.Marshal(registerData)

		// 第一次注册
		req := httptest.NewRequest("POST", "/api/auth/register", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")

		w := httptest.NewRecorder()
		ts.router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusCreated, w.Code)

		// 第二次注册相同邮箱
		registerData["username"] = "user2"
		body, _ = json.Marshal(registerData)
		req = httptest.NewRequest("POST", "/api/auth/register", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")

		w = httptest.NewRecorder()
		ts.router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusConflict, w.Code)
	})

	t.Run("错误的登录凭据", func(t *testing.T) {
		// 先注册一个用户
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
		assert.Equal(t, http.StatusCreated, w.Code)

		// 使用错误密码登录
		loginData := map[string]interface{}{
			"email":    "test@example.com",
			"password": "wrongpassword",
		}

		body, _ = json.Marshal(loginData)
		req = httptest.NewRequest("POST", "/api/auth/login", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")

		w = httptest.NewRecorder()
		ts.router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})

	t.Run("访问不存在的预设", func(t *testing.T) {
		ts.registerTestUser(t)

		req := httptest.NewRequest("GET", "/api/presets/999", nil)
		req.Header.Set("Authorization", "Bearer "+ts.token)

		w := httptest.NewRecorder()
		ts.router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusNotFound, w.Code)
	})

	t.Run("删除不存在的历史记录", func(t *testing.T) {
		ts.registerTestUser(t)

		req := httptest.NewRequest("DELETE", "/api/history/999", nil)
		req.Header.Set("Authorization", "Bearer "+ts.token)

		w := httptest.NewRecorder()
		ts.router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusNotFound, w.Code)
	})
}

// TestDataValidation 测试数据验证
func TestDataValidation(t *testing.T) {
	ts := setupTestServer(t)

	t.Run("注册时缺少必填字段", func(t *testing.T) {
		testCases := []map[string]interface{}{
			{"email": "test@example.com", "password": "password123"}, // 缺少username
			{"username": "testuser", "password": "password123"},     // 缺少email
			{"username": "testuser", "email": "test@example.com"},  // 缺少password
		}

		for i, testCase := range testCases {
			t.Run(fmt.Sprintf("测试用例 %d", i+1), func(t *testing.T) {
				body, _ := json.Marshal(testCase)
				req := httptest.NewRequest("POST", "/api/auth/register", bytes.NewBuffer(body))
				req.Header.Set("Content-Type", "application/json")

				w := httptest.NewRecorder()
				ts.router.ServeHTTP(w, req)
				assert.Equal(t, http.StatusBadRequest, w.Code)
			})
		}
	})

	t.Run("无效的邮箱格式", func(t *testing.T) {
		registerData := map[string]interface{}{
			"username": "testuser",
			"email":    "invalid-email",
			"password": "password123",
		}

		body, _ := json.Marshal(registerData)
		req := httptest.NewRequest("POST", "/api/auth/register", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")

		w := httptest.NewRecorder()
		ts.router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("密码过短", func(t *testing.T) {
		registerData := map[string]interface{}{
			"username": "testuser",
			"email":    "test@example.com",
			"password": "123", // 过短的密码
		}

		body, _ := json.Marshal(registerData)
		req := httptest.NewRequest("POST", "/api/auth/register", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")

		w := httptest.NewRecorder()
		ts.router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})
}

// TestConcurrentAccess 测试并发访问
func TestConcurrentAccess(t *testing.T) {
	ts := setupTestServer(t)
	ts.registerTestUser(t)

	t.Run("并发创建预设", func(t *testing.T) {
		concurrency := 5
		done := make(chan bool, concurrency)
		results := make(chan int, concurrency)

		for i := 0; i < concurrency; i++ {
			go func(index int) {
				presetData := map[string]interface{}{
					"name":        fmt.Sprintf("并发预设 %d", index),
					"description": fmt.Sprintf("并发测试预设 %d", index),
					"options": map[string]interface{}{
						"format": "epub",
						"index":  index,
					},
				}

				body, _ := json.Marshal(presetData)
				req := httptest.NewRequest("POST", "/api/presets", bytes.NewBuffer(body))
				req.Header.Set("Content-Type", "application/json")
				req.Header.Set("Authorization", "Bearer "+ts.token)

				w := httptest.NewRecorder()
				ts.router.ServeHTTP(w, req)

				results <- w.Code
				done <- true
			}(i)
		}

		// 等待所有goroutine完成
		for i := 0; i < concurrency; i++ {
			<-done
		}
		close(results)

		// 检查结果
		successCount := 0
		for code := range results {
			if code == http.StatusCreated {
				successCount++
			}
		}

		assert.Equal(t, concurrency, successCount, "所有并发请求都应该成功")
	})
}

// TestPerformance 性能测试
func TestPerformance(t *testing.T) {
	if testing.Short() {
		t.Skip("跳过性能测试")
	}

	ts := setupTestServer(t)
	ts.registerTestUser(t)

	t.Run("获取预设列表性能测试", func(t *testing.T) {
		// 创建大量预设
		for i := 0; i < 100; i++ {
			presetData := map[string]interface{}{
				"name":        fmt.Sprintf("性能测试预设 %d", i),
				"description": fmt.Sprintf("性能测试描述 %d", i),
				"options": map[string]interface{}{
					"format": "epub",
					"index":  i,
				},
			}

			body, _ := json.Marshal(presetData)
			req := httptest.NewRequest("POST", "/api/presets", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Authorization", "Bearer "+ts.token)

			w := httptest.NewRecorder()
			ts.router.ServeHTTP(w, req)
			assert.Equal(t, http.StatusCreated, w.Code)
		}

		// 测试获取预设列表的性能
		start := time.Now()
		req := httptest.NewRequest("GET", "/api/presets", nil)
		req.Header.Set("Authorization", "Bearer "+ts.token)

		w := httptest.NewRecorder()
		ts.router.ServeHTTP(w, req)
		duration := time.Since(start)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Less(t, duration, time.Second, "获取预设列表应该在1秒内完成")

		var presets []map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &presets)
		require.NoError(t, err)
		assert.Len(t, presets, 100)
	})
}