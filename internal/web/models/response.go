package models

import "time"

// APIResponse 统一API响应格式
type APIResponse struct {
	Code    int         `json:"code" example:"200"`           // 状态码
	Message string      `json:"message" example:"success"`   // 消息
	Data    interface{} `json:"data,omitempty"`              // 数据
	Error   string      `json:"error,omitempty" example:""` // 错误信息
}

// UploadResponse 文件上传响应
type UploadResponse struct {
	TaskID   string `json:"task_id" example:"task_123456789"`   // 任务ID
	Filename string `json:"filename" example:"example.txt"`     // 文件名
	Size     int64  `json:"size" example:"1024000"`             // 文件大小(字节)
	UploadAt string `json:"upload_at" example:"2024-01-01T00:00:00Z"` // 上传时间
}

// ConvertResponse 转换响应
type ConvertResponse struct {
	TaskID    string `json:"task_id" example:"task_123456789"`     // 任务ID
	Status    string `json:"status" example:"processing"`          // 状态
	Message   string `json:"message" example:"转换任务已开始"`          // 消息
	StartedAt string `json:"started_at" example:"2024-01-01T00:00:00Z"` // 开始时间
}

// TaskStatus 任务状态枚举
const (
	TaskStatusPending    = "pending"     // 等待中
	TaskStatusProcessing = "processing"  // 处理中
	TaskStatusCompleted  = "completed"   // 已完成
	TaskStatusFailed     = "failed"      // 失败
	TaskStatusCancelled  = "cancelled"   // 已取消
)

// TaskStatusResponse 任务状态响应
type TaskStatusResponse struct {
	TaskID      string                 `json:"task_id" example:"task_123456789"`       // 任务ID
	Status      string                 `json:"status" example:"completed"`             // 状态
	Progress    int                    `json:"progress" example:"100"`                 // 进度百分比
	Message     string                 `json:"message" example:"转换完成"`                // 当前状态消息
	StartedAt   string                 `json:"started_at" example:"2024-01-01T00:00:00Z"` // 开始时间
	CompletedAt *string                `json:"completed_at,omitempty" example:"2024-01-01T00:05:00Z"` // 完成时间
	Error       string                 `json:"error,omitempty" example:""`             // 错误信息
	Files       []ConvertedFile        `json:"files,omitempty"`                        // 转换后的文件列表
	Logs        []string               `json:"logs,omitempty"`                         // 处理日志
	Metadata    map[string]interface{} `json:"metadata,omitempty"`                     // 元数据
}

// ConvertedFile 转换后的文件信息
type ConvertedFile struct {
	FileID   string `json:"file_id" example:"file_123456789"`   // 文件ID
	Format   string `json:"format" example:"epub"`             // 格式
	Filename string `json:"filename" example:"example.epub"`   // 文件名
	Size     int64  `json:"size" example:"2048000"`            // 文件大小(字节)
	Path     string `json:"path,omitempty"`                    // 文件路径(内部使用)
}

// DownloadResponse 下载响应(实际返回文件流)
type DownloadResponse struct {
	FileID      string `json:"file_id"`
	Filename    string `json:"filename"`
	ContentType string `json:"content_type"`
	Size        int64  `json:"size"`
}

// CleanupResponse 清理响应
type CleanupResponse struct {
	TaskID        string   `json:"task_id" example:"task_123456789"`     // 任务ID
	Cleaned       bool     `json:"cleaned" example:"true"`               // 是否清理成功
	CleanedFiles  []string `json:"cleaned_files,omitempty"`              // 已清理的文件列表
	Message       string   `json:"message" example:"清理完成"`             // 消息
	CleanedAt     string   `json:"cleaned_at" example:"2024-01-01T00:10:00Z"` // 清理时间
}

// TaskEvent SSE事件
type TaskEvent struct {
	TaskID    string                 `json:"task_id"`    // 任务ID
	EventType string                 `json:"event_type"` // 事件类型
	Message   string                 `json:"message"`    // 消息
	Progress  int                    `json:"progress"`   // 进度
	Timestamp time.Time              `json:"timestamp"`  // 时间戳
	Data      map[string]interface{} `json:"data,omitempty"` // 额外数据
}

// 事件类型常量
const (
	EventTypeStart    = "start"     // 开始
	EventTypeProgress = "progress"  // 进度更新
	EventTypeLog      = "log"       // 日志
	EventTypeComplete = "complete"  // 完成
	EventTypeError    = "error"     // 错误
	EventTypeCancel   = "cancel"    // 取消
)