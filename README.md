# 电子书转换器 (Ebook Generator)

> 基于 Web 的小说转换工具，支持将 txt 文件转换为 epub、mobi、azw3 等电子书格式

本项目是从 [ystyle/kaf-cli](https://github.com/ystyle/kaf-cli) fork 而来，在保留原有命令行功能的基础上，新增了 Web 可视化界面，提供更便捷的使用体验。

## ✨ 主要特性

### 🌐 Web 功能
- 📁 **文件上传**：支持 txt 文件上传，最大 50MB
- 🔄 **格式转换**：支持转换为 epub、mobi、azw3 格式
- 📊 **实时进度**：通过 SSE 实时查看转换进度
- 📥 **文件下载**：转换完成后可下载电子书文件
- 🗑️ **自动清理**：支持手动清理临时文件
- 📖 **API 文档**：集成 Swagger UI，方便调试
- 🎨 **可视化界面**：简洁易用的 HTML 界面

### 📚 转换功能
- 自动识别书名和章节
- 自动识别字符编码（解决中文乱码）
- 自定义章节标题识别规则
- 自定义卷的标题识别规则
- 自动给章节正文生成加粗居中的标题
- 段落自动识别和缩进
- 支持生成 Orly 风格的书籍封面
- 知轩藏书格式文件名自动提取书名和作者
- 超快速转换（epub 格式生成 300 章/s 以上速度）

## 🚀 快速开始

### 方式一：Web 服务（推荐）

#### 1. 克隆项目

```bash
git clone https://github.com/Deali-Axy/ebook-generator.git
cd ebook-generator
```

#### 2. 安装依赖

```bash
go mod tidy
```

#### 3. 启动 Web 服务

```bash
go run cmd/web/main.go
```

服务将在 `http://localhost:8080` 启动

#### 4. 使用可视化界面

- 打开浏览器访问：`http://localhost:8080`
- 上传 txt 文件
- 设置书名、作者等参数
- 选择输出格式
- 点击开始转换
- 实时查看转换进度
- 下载生成的电子书

#### 5. 查看 API 文档

打开浏览器访问：`http://localhost:8080/swagger/index.html`

### 方式二：下载可执行文件

- 电脑版 kaf-cli: [Github 下载](https://github.com/Deali-Axy/ebook-generator/releases/latest)
- 包管理器安装:
  - Archlinux: `yay -S kaf-cli`
  - Windows: `winget install kaf-cli`

## 📖 Web API 接口

### 文件上传

```http
POST /api/upload
Content-Type: multipart/form-data

参数：
- file: txt文件（最大50MB）

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

```http
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

```http
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

```http
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

```http
GET /api/download/{fileId}

响应：文件流
```

### 清理任务

```http
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

## ⚙️ 转换参数说明

| 参数 | 类型 | 必填 | 默认值 | 说明 |
|------|------|------|--------|----------|
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

## 🚀 部署指南

### Docker 部署

```bash
# 构建镜像
docker build -t ebook-generator .

# 运行容器
docker run -d -p 8080:8080 --name ebook-generator ebook-generator
```

### 环境变量

| 变量名 | 默认值 | 说明 |
|--------|--------|----------|
| PORT | 8080 | 服务端口 |
| GIN_MODE | debug | Gin运行模式 |

### 目录结构

```
web/
├── uploads/    # 上传文件临时目录
└── outputs/    # 转换后文件目录
```

## 💡 使用示例

### 使用 curl 调用 API

#### 1. 上传文件

```bash
curl -X POST http://localhost:8080/api/upload \
  -F "file=@example.txt"
```

#### 2. 开始转换

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

#### 3. 查询状态

```bash
curl http://localhost:8080/api/status/task_1234567890
```

#### 4. 下载文件

```bash
curl -O http://localhost:8080/api/download/file_1234567890
```

## 📝 任务状态说明

- `pending`: 等待中
- `processing`: 处理中
- `completed`: 已完成
- `failed`: 失败
- `cancelled`: 已取消

## ⚠️ 注意事项

1. 上传的文件必须是 UTF-8 编码的 txt 文件
2. 文件大小限制为 50MB
3. 转换后的文件会保存在服务器上，建议定期清理
4. SSE 连接会在 30 秒无活动后发送心跳包
5. 任务完成后建议调用清理接口释放存储空间

## 🔧 错误码

| 错误码 | 说明 |
|--------|----------|
| 200 | 成功 |
| 400 | 请求参数错误 |
| 404 | 资源不存在 |
| 429 | 请求过于频繁 |
| 500 | 服务器内部错误 |

## 📱 命令行模式（传统功能）

### 基本使用

```bash
# 简单模式（拖拽功能）
kaf-cli ~/全职法师.txt

# 自定义作者
kaf-cli -author 乱 -filename ~/全职法师.txt

# 自定义章节匹配
kaf-cli -filename ~/小说.txt -match "第.{1,8}节"
```

### 主要参数

- `-author`: 作者名
- `-bookname`: 书名
- `-format`: 输出格式（epub/mobi/azw3/all）
- `-match`: 章节匹配正则表达式
- `-cover`: 封面设置

更多详细参数请参考原项目文档或使用 `kaf-cli -h` 查看。

---

## 📄 许可证

本项目基于 MIT 许可证开源。详见 [LICENSE](LICENSE) 文件。

## 🤝 贡献

欢迎提交 Issue 和 Pull Request！

## 🙏 致谢

- 原项目：[ystyle/kaf-cli](https://github.com/ystyle/kaf-cli)
- 感谢所有贡献者的支持

## 📞 联系

如有问题或建议，请通过 GitHub Issues 联系。

