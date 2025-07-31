# API 测试文档

本文档说明如何运行电子书生成器的API接口测试。

## 📋 测试概述

我们为电子书生成器创建了全面的API测试套件，包括：

### 🔐 认证功能测试
- 用户注册 (`POST /api/auth/register`)
- 用户登录 (`POST /api/auth/login`)
- 获取用户资料 (`GET /api/auth/profile`)
- 更新用户资料 (`PUT /api/auth/profile`)
- 用户登出 (`POST /api/auth/logout`)
- 刷新访问令牌 (`POST /api/auth/refresh`)

### 📚 转换历史测试
- 获取转换历史列表 (`GET /api/history`)
- 获取转换统计信息 (`GET /api/history/stats`)
- 删除转换历史 (`DELETE /api/history/{id}`)

### ⚙️ 转换预设测试
- 创建转换预设 (`POST /api/presets`)
- 获取预设列表 (`GET /api/presets`)
- 更新转换预设 (`PUT /api/presets/{id}`)
- 删除转换预设 (`DELETE /api/presets/{id}`)

### 🔄 批量转换测试
- 批量转换文件 (`POST /api/batch/convert`)

### 🛡️ 安全性测试
- 未授权访问测试
- 无效token测试
- 数据验证测试
- 错误处理测试

### 🚀 性能和并发测试
- 并发访问测试
- 性能基准测试

## 🚀 快速开始

### 方法一：使用PowerShell脚本（推荐）

```powershell
# 在项目根目录下运行
.\run_tests.ps1
```

### 方法二：手动运行

```bash
# 1. 安装测试依赖
go get github.com/stretchr/testify/assert
go get github.com/stretchr/testify/require

# 2. 运行所有测试
go test -v ./tests/...

# 3. 运行特定测试
go test -v ./tests/ -run TestAuthEndpoints

# 4. 生成覆盖率报告
go test -v ./tests/... -coverprofile=coverage.out
go tool cover -html=coverage.out -o coverage.html
```

## 📁 测试文件结构

```
tests/
├── api_test.go          # 主要API功能测试
└── integration_test.go  # 集成测试和高级测试
```

## 🧪 测试用例详情

### 基础功能测试

#### 认证测试 (`TestAuthEndpoints`)
- ✅ 用户注册功能
- ✅ 用户登录功能
- ✅ 获取用户资料
- ✅ 更新用户资料

#### 历史记录测试 (`TestHistoryEndpoints`)
- ✅ 获取转换历史列表
- ✅ 获取转换统计信息
- ✅ 删除转换历史

#### 预设管理测试 (`TestPresetEndpoints`)
- ✅ 创建转换预设
- ✅ 获取预设列表
- ✅ 更新转换预设
- ✅ 删除转换预设

#### 批量转换测试 (`TestBatchConversionEndpoint`)
- ✅ 批量文件上传和转换

### 安全性测试

#### 未授权访问测试 (`TestUnauthorizedAccess`)
- ✅ 验证所有受保护的端点都需要认证

#### 无效Token测试 (`TestInvalidToken`)
- ✅ 验证无效token被正确拒绝

### 集成测试

#### 完整工作流程测试 (`TestAPIIntegration`)
- ✅ 注册 → 登录 → 创建预设 → 查看历史 → 注销的完整流程

#### 错误处理测试 (`TestErrorHandling`)
- ✅ 重复注册相同邮箱
- ✅ 错误的登录凭据
- ✅ 访问不存在的资源

#### 数据验证测试 (`TestDataValidation`)
- ✅ 缺少必填字段
- ✅ 无效的邮箱格式
- ✅ 密码长度验证

#### 并发测试 (`TestConcurrentAccess`)
- ✅ 并发创建预设
- ✅ 数据一致性验证

#### 性能测试 (`TestPerformance`)
- ✅ 大量数据下的查询性能
- ✅ 响应时间基准测试

## 📊 测试报告

运行测试后，您将获得：

1. **控制台输出**：详细的测试执行结果
2. **覆盖率报告**：`coverage.out` 文件
3. **HTML报告**：`coverage.html` 文件（可在浏览器中查看）

## 🔧 测试配置

测试使用以下配置：

- **数据库**：内存SQLite数据库（每个测试独立）
- **认证**：JWT token认证
- **环境**：Gin测试模式
- **超时**：30秒测试超时

## 🐛 故障排除

### 常见问题

1. **依赖问题**
   ```bash
   go mod tidy
   go mod download
   ```

2. **权限问题**
   ```powershell
   Set-ExecutionPolicy -ExecutionPolicy RemoteSigned -Scope CurrentUser
   ```

3. **端口占用**
   - 测试使用内存数据库和模拟HTTP请求，不需要实际端口

4. **测试失败**
   - 检查Go版本（建议1.19+）
   - 确保所有依赖已正确安装
   - 查看详细错误信息

## 📈 扩展测试

要添加新的测试用例：

1. 在 `tests/api_test.go` 中添加新的测试函数
2. 使用 `setupTestServer()` 创建测试环境
3. 使用 `registerTestUser()` 创建认证用户
4. 编写断言验证预期行为

示例：
```go
func TestNewFeature(t *testing.T) {
    ts := setupTestServer(t)
    ts.registerTestUser(t)
    
    // 您的测试逻辑
    req := httptest.NewRequest("GET", "/api/new-endpoint", nil)
    req.Header.Set("Authorization", "Bearer "+ts.token)
    
    w := httptest.NewRecorder()
    ts.router.ServeHTTP(w, req)
    
    assert.Equal(t, http.StatusOK, w.Code)
}
```

## 🎯 最佳实践

1. **独立性**：每个测试都是独立的，使用独立的数据库
2. **清理**：测试后自动清理资源
3. **断言**：使用明确的断言验证预期结果
4. **覆盖率**：保持高测试覆盖率
5. **性能**：包含性能基准测试

---

**注意**：这些测试专门针对新增的API功能。确保在运行测试前，Web服务器已正确配置并包含所有必要的处理器和服务。