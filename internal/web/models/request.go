package models

import (
	"mime/multipart"
)

// UploadRequest 文件上传请求
type UploadRequest struct {
	File *multipart.FileHeader `form:"file" binding:"required" swaggerignore:"true"`
}

// ConvertRequest 转换请求
type ConvertRequest struct {
	TaskID           string `json:"task_id" binding:"required" example:"task_123456789"`                    // 任务ID
	Bookname         string `json:"bookname" binding:"required" example:"示例小说"`                          // 书名
	Author           string `json:"author" example:"作者名"`                                               // 作者
	Format           string `json:"format" binding:"required,oneof=epub mobi azw3 all" example:"epub"` // 输出格式
	Match            string `json:"match" example:"^第[0-9一二三四五六七八九十零〇百千两 ]+[章回节集幕卷部]"`              // 章节匹配规则
	VolumeMatch      string `json:"volume_match" example:"^第[0-9一二三四五六七八九十零〇百千两 ]+[卷部]"`         // 卷匹配规则
	ExclusionPattern string `json:"exclusion_pattern" example:"^第[0-9一二三四五六七八九十零〇百千两 ]+(部门|部队)"` // 排除规则
	Max              uint   `json:"max" example:"35"`                                                   // 标题最大字数
	Indent           uint   `json:"indent" example:"2"`                                                 // 段落缩进
	Align            string `json:"align" example:"center"`                                             // 标题对齐方式
	UnknowTitle      string `json:"unknow_title" example:"章节正文"`                                       // 未知章节名称
	Cover            string `json:"cover" example:"gen"`                                                // 封面设置
	CoverOrlyColor   string `json:"cover_orly_color" example:"#FF6B6B"`                                // 封面颜色
	CoverOrlyIdx     int    `json:"cover_orly_idx" example:"1"`                                        // 封面动物索引
	Font             string `json:"font" example:""`                                                    // 嵌入字体
	Bottom           string `json:"bottom" example:"1em"`                                               // 段落间距
	LineHeight       string `json:"line_height" example:"1.5"`                                         // 行高
	Tips             bool   `json:"tips" example:"true"`                                                // 是否添加教程文本
	Lang             string `json:"lang" example:"zh"`                                                  // 语言设置
}

// TaskStatusRequest 任务状态查询请求
type TaskStatusRequest struct {
	TaskID string `uri:"taskId" binding:"required" example:"task_123456789"` // 任务ID
}

// DownloadRequest 下载请求
type DownloadRequest struct {
	FileID string `uri:"fileId" binding:"required" example:"file_123456789"` // 文件ID
}

// CleanupRequest 清理请求
type CleanupRequest struct {
	TaskID string `uri:"taskId" binding:"required" example:"task_123456789"` // 任务ID
}