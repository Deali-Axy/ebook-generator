# 电子书转换Web服务

这是一个基于Gin框架的Web服务，用于将txt文件转换为不同格式的电子书（epub、mobi、azw3）。

## 功能特性

- 📁 文件上传：支持txt文件上传，最大50MB
- 🔄 格式转换：支持转换为epub、mobi、azw3格式
- 📊 实时进度：通过SSE实时查看转换进度
- 📥 文件下载：转换完成后可下载电子书文件
- 🗑️ 自动清理：支持手动清理临时文件
- 📖 API文档：集成Swagger UI，方便调试

## 快速开始

### 1. 安装依赖

```bash
go mod tidy
```

### 2. 启动服务

```bash
go run cmd/web/main.go
```

服务将在 `http://localhost:8080` 启动

### 3. 访问API文档

打开浏览器访问：`http://localhost:8080/swagger/index.html`

## API接口

### 文件上传
```
POST /api/upload
Content-Type: multipart/form-data

参数：
- file: txt文件

响应：
{
  "code": 200,
  "message": "文件上传成功",
  "data": {
    "task_id": "task_1234567890",
    "filename": "example.txt",
    "size": 1024000,
    "upload_at": "2024-01-01T00:00:00Z"
  }
}
```

### 开始转换
```
POST /api/convert
Content-Type: application/json

{
  "task_id": "task_1234567890",
  "bookname": "示例小说",
  "author": "作者名",
  "format": "epub",
  "match": "^第[0-9一二三四五六七八九十零〇百千两 ]+[章回节集幕卷部]"
}
```

### 查询状态
```
GET /api/status/{taskId}

响应：
{
  "code": 200,
  "message": "获取状态成功",
  "data": {
    "task_id": "task_1234567890",
    "status": "completed",
    "progress": 100,
    "message": "转换完成",
    "files": [
      {
        "file_id": "file_1234567890",
        "format": "epub",
        "filename": "example.epub",
        "size": 2048000
      }
    ]
  }
}
```

### 实时事件流（SSE）
```
GET /api/events/{taskId}
Accept: text/event-stream

事件格式：
data: {
  "task_id": "task_1234567890",
  "event_type": "progress",
  "message": "正在解析文件",
  "progress": 50,
  "timestamp": "2024-01-01T00:00:00Z"
}
```

### 下载文件
```
GET /api/download/{fileId}

响应：文件流
```

### 清理任务
```
DELETE /api/cleanup/{taskId}

响应：
{
  "code": 200,
  "message": "清理成功",
  "data": {
    "task_id": "task_1234567890",
    "cleaned": true,
    "cleaned_files": ["/path/to/file1", "/path/to/file2"],
    "cleaned_at": "2024-01-01T00:00:00Z"
  }
}
```

## 转换参数说明

| 参数 | 类型 | 必填 | 默认值 | 说明 |
|------|------|------|--------|---------|
| task_id | string | 是 | - | 任务ID（从上传接口获取） |
| bookname | string | 是 | - | 书名 |
| author | string | 否 | "YSTYLE" | 作者 |
| format | string | 是 | - | 输出格式：epub/mobi/azw3/all |
| match | string | 否 | 默认规则 | 章节匹配正则表达式 |
| volume_match | string | 否 | 默认规则 | 卷匹配正则表达式 |
| exclusion_pattern | string | 否 | 默认规则 | 排除规则正则表达式 |
| max | uint | 否 | 35 | 标题最大字数 |
| indent | uint | 否 | 2 | 段落缩进 |
| align | string | 否 | "center" | 标题对齐方式 |
| unknow_title | string | 否 | "章节正文" | 未知章节名称 |
| cover | string | 否 | "gen" | 封面设置 |
| tips | bool | 否 | true | 是否添加教程文本 |
| lang | string | 否 | "zh" | 语言设置 |

## 任务状态

- `pending`: 等待中
- `processing`: 处理中
- `completed`: 已完成
- `failed`: 失败
- `cancelled`: 已取消

## 事件类型

- `start`: 开始
- `progress`: 进度更新
- `log`: 日志
- `complete`: 完成
- `error`: 错误
- `cancel`: 取消

## 环境变量

| 变量名 | 默认值 | 说明 |
|--------|--------|---------|
| PORT | 8080 | 服务端口 |
| GIN_MODE | debug | Gin运行模式 |

## 目录结构

```
web/
├── uploads/    # 上传文件临时目录
└── outputs/    # 转换后文件目录
```

## 使用示例

### 1. 使用curl上传文件

```bash
curl -X POST http://localhost:8080/api/upload \
  -F "file=@example.txt"
```

### 2. 开始转换

```bash
curl -X POST http://localhost:8080/api/convert \
  -H "Content-Type: application/json" \
  -d '{
    "task_id": "task_1234567890",
    "bookname": "示例小说",
    "author": "作者名",
    "format": "epub"
  }'
```

### 3. 查询状态

```bash
curl http://localhost:8080/api/status/task_1234567890
```

### 4. 下载文件

```bash
curl -O http://localhost:8080/api/download/file_1234567890
```

## 注意事项

1. 上传的文件必须是UTF-8编码的txt文件
2. 文件大小限制为50MB
3. 转换后的文件会保存在服务器上，建议定期清理
4. SSE连接会在30秒无活动后发送心跳包
5. 任务完成后建议调用清理接口释放存储空间

## 错误码

| 错误码 | 说明 |
|--------|---------|
| 200 | 成功 |
| 400 | 请求参数错误 |
| 404 | 资源不存在 |
| 429 | 请求过于频繁 |
| 500 | 服务器内部错误 |