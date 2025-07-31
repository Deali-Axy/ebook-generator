package models

import (
	"fmt"
	"time"
	"gorm.io/gorm"
)

// ConversionHistory 转换历史记录
type ConversionHistory struct {
	ID               uint                   `json:"id" gorm:"primaryKey"`
	UserID           uint                   `json:"user_id" gorm:"index"`
	TaskID           string                 `json:"task_id" gorm:"uniqueIndex;size:255"`
	OriginalFileName string                 `json:"original_file_name" gorm:"size:255"`
	OriginalFileSize int64                  `json:"original_file_size"`
	OriginalFileHash string                 `json:"original_file_hash" gorm:"size:64"`
	OutputFormat     string                 `json:"output_format" gorm:"size:10"`
	OutputFileName   string                 `json:"output_file_name" gorm:"size:255"`
	OutputFileSize   int64                  `json:"output_file_size"`
	ConvertOptions   ConvertOptionsJSON     `json:"convert_options" gorm:"type:text"`
	Status           string                 `json:"status" gorm:"size:20;index"`
	StartTime        time.Time              `json:"start_time"`
	EndTime          *time.Time             `json:"end_time,omitempty"`
	Duration         int64                  `json:"duration"` // 转换耗时（毫秒）
	ErrorMessage     string                 `json:"error_message,omitempty" gorm:"type:text"`
	DownloadCount    int                    `json:"download_count" gorm:"default:0"`
	LastDownloadAt   *time.Time             `json:"last_download_at,omitempty"`
	IsDeleted        bool                   `json:"is_deleted" gorm:"default:false;index"`
	CreatedAt        time.Time              `json:"created_at"`
	UpdatedAt        time.Time              `json:"updated_at"`
	DeletedAt        gorm.DeletedAt         `json:"-" gorm:"index"`

	// 关联
	User *User `json:"user,omitempty" gorm:"foreignKey:UserID"`
}

// ConvertOptionsJSON 转换选项JSON类型
type ConvertOptionsJSON map[string]interface{}

// HistoryListRequest 历史记录列表请求
type HistoryListRequest struct {
	Page       int    `json:"page" form:"page" binding:"min=1"`
	PageSize   int    `json:"page_size" form:"page_size" binding:"min=1,max=100"`
	Status     string `json:"status" form:"status"`
	Format     string `json:"format" form:"format"`
	StartDate  string `json:"start_date" form:"start_date"`
	EndDate    string `json:"end_date" form:"end_date"`
	Keyword    string `json:"keyword" form:"keyword"`
	SortBy     string `json:"sort_by" form:"sort_by"`
	SortOrder  string `json:"sort_order" form:"sort_order"`
}

// HistoryListResponse 历史记录列表响应
type HistoryListResponse struct {
	Items      []ConversionHistory `json:"items"`
	Total      int64               `json:"total"`
	Page       int                 `json:"page"`
	PageSize   int                 `json:"page_size"`
	TotalPages int                 `json:"total_pages"`
}

// HistoryStatsResponse 历史统计响应
type HistoryStatsResponse struct {
	TotalConversions    int64                    `json:"total_conversions"`
	SuccessfulConversions int64                  `json:"successful_conversions"`
	FailedConversions   int64                    `json:"failed_conversions"`
	SuccessRate         float64                  `json:"success_rate"`
	TotalFileSize       int64                    `json:"total_file_size"`
	AverageDuration     float64                  `json:"average_duration"`
	FormatStats         map[string]int64         `json:"format_stats"`
	MonthlyStats        []MonthlyStats           `json:"monthly_stats"`
	RecentActivity      []ConversionHistory      `json:"recent_activity"`
}

// MonthlyStats 月度统计
type MonthlyStats struct {
	Month       string `json:"month"`
	Year        int    `json:"year"`
	Count       int64  `json:"count"`
	SuccessCount int64  `json:"success_count"`
	FailureCount int64  `json:"failure_count"`
}

// BatchConversionRequest 批量转换请求
type BatchConversionRequest struct {
	Files          []BatchFileInfo `json:"files" binding:"required,min=1,max=10"`
	OutputFormat   string          `json:"output_format" binding:"required,oneof=epub mobi azw3 pdf"`
	BookTitle      string          `json:"book_title"`
	Author         string          `json:"author"`
	CommonOptions  map[string]interface{} `json:"common_options"`
}

// BatchFileInfo 批量文件信息
type BatchFileInfo struct {
	FileName      string                 `json:"file_name" binding:"required"`
	FileSize      int64                  `json:"file_size" binding:"required,min=1"`
	FileHash      string                 `json:"file_hash"`
	CustomOptions map[string]interface{} `json:"custom_options"`
}

// BatchConversionResponse 批量转换响应
type BatchConversionResponse struct {
	BatchID    string                    `json:"batch_id"`
	TotalFiles int                       `json:"total_files"`
	Tasks      []BatchTaskInfo           `json:"tasks"`
	Status     string                    `json:"status"`
	CreatedAt  time.Time                 `json:"created_at"`
}

// BatchTaskInfo 批量任务信息
type BatchTaskInfo struct {
	TaskID       string `json:"task_id"`
	FileName     string `json:"file_name"`
	Status       string `json:"status"`
	ErrorMessage string `json:"error_message,omitempty"`
}

// ConversionPreset 转换预设
type ConversionPreset struct {
	ID          uint                   `json:"id" gorm:"primaryKey"`
	UserID      uint                   `json:"user_id" gorm:"index"`
	Name        string                 `json:"name" gorm:"size:100;not null"`
	Description string                 `json:"description" gorm:"size:500"`
	OutputFormat string                `json:"output_format" gorm:"size:10;not null"`
	Options     ConvertOptionsJSON     `json:"options" gorm:"type:text"`
	IsDefault   bool                   `json:"is_default" gorm:"default:false"`
	IsPublic    bool                   `json:"is_public" gorm:"default:false"`
	UsageCount  int64                  `json:"usage_count" gorm:"default:0"`
	CreatedAt   time.Time              `json:"created_at"`
	UpdatedAt   time.Time              `json:"updated_at"`
	DeletedAt   gorm.DeletedAt         `json:"-" gorm:"index"`

	// 关联
	User *User `json:"user,omitempty" gorm:"foreignKey:UserID"`
}

// PresetListRequest 预设列表请求
type PresetListRequest struct {
	Page       int    `json:"page" form:"page" binding:"min=1"`
	PageSize   int    `json:"page_size" form:"page_size" binding:"min=1,max=100"`
	Format     string `json:"format" form:"format"`
	Keyword    string `json:"keyword" form:"keyword"`
	IsPublic   *bool  `json:"is_public" form:"is_public"`
	SortBy     string `json:"sort_by" form:"sort_by"`
	SortOrder  string `json:"sort_order" form:"sort_order"`
}

// PresetCreateRequest 创建预设请求
type PresetCreateRequest struct {
	Name         string                 `json:"name" binding:"required,min=1,max=100"`
	Description  string                 `json:"description" binding:"max=500"`
	OutputFormat string                 `json:"output_format" binding:"required,oneof=epub mobi azw3 pdf"`
	Options      map[string]interface{} `json:"options"`
	IsDefault    bool                   `json:"is_default"`
	IsPublic     bool                   `json:"is_public"`
}

// PresetUpdateRequest 更新预设请求
type PresetUpdateRequest struct {
	Name         string                 `json:"name" binding:"min=1,max=100"`
	Description  string                 `json:"description" binding:"max=500"`
	OutputFormat string                 `json:"output_format" binding:"oneof=epub mobi azw3 pdf"`
	Options      map[string]interface{} `json:"options"`
	IsDefault    bool                   `json:"is_default"`
	IsPublic     bool                   `json:"is_public"`
}

// DownloadRecord 下载记录
type DownloadRecord struct {
	ID         uint      `json:"id" gorm:"primaryKey"`
	UserID     uint      `json:"user_id" gorm:"index"`
	HistoryID  uint      `json:"history_id" gorm:"index"`
	TaskID     string    `json:"task_id" gorm:"size:255;index"`
	FileName   string    `json:"file_name" gorm:"size:255"`
	FileSize   int64     `json:"file_size"`
	ClientIP   string    `json:"client_ip" gorm:"size:45"`
	UserAgent  string    `json:"user_agent" gorm:"size:500"`
	DownloadAt time.Time `json:"download_at"`

	// 关联
	User    *User               `json:"user,omitempty" gorm:"foreignKey:UserID"`
	History *ConversionHistory  `json:"history,omitempty" gorm:"foreignKey:HistoryID"`
}

// TableName 指定表名
func (ConversionHistory) TableName() string {
	return "conversion_histories"
}

func (ConversionPreset) TableName() string {
	return "conversion_presets"
}

func (DownloadRecord) TableName() string {
	return "download_records"
}

// BeforeCreate 创建前钩子
func (h *ConversionHistory) BeforeCreate(tx *gorm.DB) error {
	if h.StartTime.IsZero() {
		h.StartTime = time.Now()
	}
	return nil
}

// MarkAsCompleted 标记为完成
func (h *ConversionHistory) MarkAsCompleted(outputFileName string, outputFileSize int64) {
	now := time.Now()
	h.Status = "completed"
	h.EndTime = &now
	h.OutputFileName = outputFileName
	h.OutputFileSize = outputFileSize
	h.Duration = now.Sub(h.StartTime).Milliseconds()
}

// MarkAsFailed 标记为失败
func (h *ConversionHistory) MarkAsFailed(errorMessage string) {
	now := time.Now()
	h.Status = "failed"
	h.EndTime = &now
	h.ErrorMessage = errorMessage
	h.Duration = now.Sub(h.StartTime).Milliseconds()
}

// IncrementDownloadCount 增加下载次数
func (h *ConversionHistory) IncrementDownloadCount() {
	h.DownloadCount++
	now := time.Now()
	h.LastDownloadAt = &now
}

// GetDurationString 获取持续时间字符串
func (h *ConversionHistory) GetDurationString() string {
	if h.Duration == 0 {
		return "0s"
	}
	duration := time.Duration(h.Duration) * time.Millisecond
	if duration < time.Second {
		return fmt.Sprintf("%dms", h.Duration)
	}
	return duration.String()
}

// IsExpired 检查是否过期
func (h *ConversionHistory) IsExpired(ttl time.Duration) bool {
	if h.EndTime == nil {
		return false
	}
	return time.Since(*h.EndTime) > ttl
}