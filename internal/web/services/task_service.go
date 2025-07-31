package services

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/Deali-Axy/ebook-generator/internal/core"
	"github.com/Deali-Axy/ebook-generator/internal/model"
	"github.com/Deali-Axy/ebook-generator/internal/storage"
	"github.com/Deali-Axy/ebook-generator/internal/web/models"
)

// TaskService 任务服务
type TaskService struct {
	mu               sync.RWMutex
	tasks            map[string]*TaskInfo
	storageService   *storage.StorageService
	converterService *ConverterService
	eventChannels    map[string]chan models.TaskEvent
}

// TaskInfo 任务信息
type TaskInfo struct {
	ID          string                 `json:"id"`
	Status      string                 `json:"status"`
	Progress    int                    `json:"progress"`
	Message     string                 `json:"message"`
	StartedAt   time.Time              `json:"started_at"`
	CompletedAt *time.Time             `json:"completed_at,omitempty"`
	Error       string                 `json:"error,omitempty"`
	Files       []models.ConvertedFile `json:"files,omitempty"`
	Logs        []string               `json:"logs,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
	Request     *models.ConvertRequest `json:"request,omitempty"`
	ctx         context.Context
	cancel      context.CancelFunc
}

// NewTaskService 创建任务服务
func NewTaskService(storageService *storage.StorageService, converterService *ConverterService) *TaskService {
	return &TaskService{
		tasks:            make(map[string]*TaskInfo),
		storageService:   storageService,
		converterService: converterService,
		eventChannels:    make(map[string]chan models.TaskEvent),
	}
}

// CreateTask 创建新任务
func (s *TaskService) CreateTask(taskID string, request *models.ConvertRequest) *TaskInfo {
	s.mu.Lock()
	defer s.mu.Unlock()

	ctx, cancel := context.WithCancel(context.Background())
	task := &TaskInfo{
		ID:        taskID,
		Status:    models.TaskStatusPending,
		Progress:  0,
		Message:   "任务已创建，等待处理",
		StartedAt: time.Now(),
		Logs:      []string{},
		Metadata:  make(map[string]interface{}),
		Request:   request,
		ctx:       ctx,
		cancel:    cancel,
	}

	s.tasks[taskID] = task
	s.eventChannels[taskID] = make(chan models.TaskEvent, 100)

	return task
}

// GetTask 获取任务信息
func (s *TaskService) GetTask(taskID string) (*TaskInfo, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	task, exists := s.tasks[taskID]
	return task, exists
}

// GetTaskStatus 获取任务状态响应
func (s *TaskService) GetTaskStatus(taskID string) (*models.TaskStatusResponse, error) {
	task, exists := s.GetTask(taskID)
	if !exists {
		return nil, fmt.Errorf("任务不存在: %s", taskID)
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	response := &models.TaskStatusResponse{
		TaskID:    task.ID,
		Status:    task.Status,
		Progress:  task.Progress,
		Message:   task.Message,
		StartedAt: task.StartedAt.Format(time.RFC3339),
		Error:     task.Error,
		Files:     task.Files,
		Logs:      task.Logs,
		Metadata:  task.Metadata,
	}

	if task.CompletedAt != nil {
		completedStr := task.CompletedAt.Format(time.RFC3339)
		response.CompletedAt = &completedStr
	}

	return response, nil
}

// StartConversion 开始转换任务
func (s *TaskService) StartConversion(taskID string) error {
	task, exists := s.GetTask(taskID)
	if !exists {
		return fmt.Errorf("任务不存在: %s", taskID)
	}

	// 启动异步转换
	go s.processConversion(task)

	return nil
}

// processConversion 处理转换任务
func (s *TaskService) processConversion(task *TaskInfo) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("转换任务异常: %v", r)
			s.updateTaskStatus(task.ID, models.TaskStatusFailed, 0, fmt.Sprintf("转换异常: %v", r), fmt.Sprintf("%v", r))
			s.sendEvent(task.ID, models.EventTypeError, fmt.Sprintf("转换异常: %v", r), 0, nil)
			s.closeEventChannel(task.ID)
		}
	}()

	// 更新状态为处理中
	s.updateTaskStatus(task.ID, models.TaskStatusProcessing, 10, "开始转换任务", "")
	s.sendEvent(task.ID, models.EventTypeStart, "开始转换任务", 10, nil)

	// 获取上传的文件路径
	filePath, err := s.storageService.GetUploadedFilePath(task.ID)
	if err != nil {
		s.updateTaskStatus(task.ID, models.TaskStatusFailed, 0, "获取文件失败", err.Error())
		s.sendEvent(task.ID, models.EventTypeError, "获取文件失败: "+err.Error(), 0, nil)
		s.closeEventChannel(task.ID)
		return
	}

	// 创建Book对象
	book := s.createBookFromRequest(task.Request, filePath)

	// 检查和验证
	s.updateTaskStatus(task.ID, models.TaskStatusProcessing, 20, "验证文件和参数", "")
	s.sendEvent(task.ID, models.EventTypeProgress, "验证文件和参数", 20, nil)

	if err := core.Check(book, "1.0.0"); err != nil {
		s.updateTaskStatus(task.ID, models.TaskStatusFailed, 0, "文件验证失败", err.Error())
		s.sendEvent(task.ID, models.EventTypeError, "文件验证失败: "+err.Error(), 0, nil)
		s.closeEventChannel(task.ID)
		return
	}

	// 解析文件
	s.updateTaskStatus(task.ID, models.TaskStatusProcessing, 40, "解析文本文件", "")
	s.sendEvent(task.ID, models.EventTypeProgress, "解析文本文件", 40, nil)

	if err := core.Parse(book); err != nil {
		s.updateTaskStatus(task.ID, models.TaskStatusFailed, 0, "文件解析失败", err.Error())
		s.sendEvent(task.ID, models.EventTypeError, "文件解析失败: "+err.Error(), 0, nil)
		s.closeEventChannel(task.ID)
		return
	}

	// 转换为电子书
	s.updateTaskStatus(task.ID, models.TaskStatusProcessing, 60, "生成电子书文件", "")
	s.sendEvent(task.ID, models.EventTypeProgress, "生成电子书文件", 60, nil)

	convertedFiles, err := s.converterService.ConvertBook(book, task.Request.Format)
	if err != nil {
		s.updateTaskStatus(task.ID, models.TaskStatusFailed, 0, "转换失败", err.Error())
		s.sendEvent(task.ID, models.EventTypeError, "转换失败: "+err.Error(), 0, nil)
		s.closeEventChannel(task.ID)
		return
	}

	// 保存转换后的文件
	s.updateTaskStatus(task.ID, models.TaskStatusProcessing, 80, "保存转换结果", "")
	s.sendEvent(task.ID, models.EventTypeProgress, "保存转换结果", 80, nil)

	files, err := s.storageService.SaveConvertedFiles(task.ID, convertedFiles)
	if err != nil {
		s.updateTaskStatus(task.ID, models.TaskStatusFailed, 0, "保存文件失败", err.Error())
		s.sendEvent(task.ID, models.EventTypeError, "保存文件失败: "+err.Error(), 0, nil)
		s.closeEventChannel(task.ID)
		return
	}

	// 更新任务完成状态
	s.mu.Lock()
	if task, exists := s.tasks[task.ID]; exists {
		task.Status = models.TaskStatusCompleted
		task.Progress = 100
		task.Message = "转换完成"
		task.Files = files
		now := time.Now()
		task.CompletedAt = &now
		task.Logs = append(task.Logs, fmt.Sprintf("[%s] 转换完成，生成了%d个文件", time.Now().Format("15:04:05"), len(files)))
	}
	s.mu.Unlock()

	s.sendEvent(task.ID, models.EventTypeComplete, "转换完成", 100, map[string]interface{}{
		"files": files,
	})

	// 发送完成事件后立即关闭事件通道
	s.closeEventChannel(task.ID)
}

// createBookFromRequest 从请求创建Book对象
func (s *TaskService) createBookFromRequest(req *models.ConvertRequest, filePath string) *model.Book {
	book := &model.Book{
		Filename:         filePath,
		Bookname:         req.Bookname,
		Author:           req.Author,
		Match:            req.Match,
		VolumeMatch:      req.VolumeMatch,
		ExclusionPattern: req.ExclusionPattern,
		Max:              req.Max,
		Indent:           req.Indent,
		Align:            req.Align,
		UnknowTitle:      req.UnknowTitle,
		Cover:            req.Cover,
		CoverOrlyColor:   req.CoverOrlyColor,
		CoverOrlyIdx:     req.CoverOrlyIdx,
		Font:             req.Font,
		Bottom:           req.Bottom,
		LineHeight:       req.LineHeight,
		Tips:             req.Tips,
		Lang:             req.Lang,
		Format:           req.Format,
		Out:              req.Bookname,
	}

	// 设置默认值
	model.SetDefault(book)

	return book
}

// updateTaskStatus 更新任务状态
func (s *TaskService) updateTaskStatus(taskID, status string, progress int, message, errorMsg string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if task, exists := s.tasks[taskID]; exists {
		task.Status = status
		task.Progress = progress
		task.Message = message
		if errorMsg != "" {
			task.Error = errorMsg
		}
		task.Logs = append(task.Logs, fmt.Sprintf("[%s] %s", time.Now().Format("15:04:05"), message))
	}
}

// sendEvent 发送SSE事件
func (s *TaskService) sendEvent(taskID, eventType, message string, progress int, data map[string]interface{}) {
	s.mu.RLock()
	ch, exists := s.eventChannels[taskID]
	s.mu.RUnlock()

	if !exists {
		return
	}

	event := models.TaskEvent{
		TaskID:    taskID,
		EventType: eventType,
		Message:   message,
		Progress:  progress,
		Timestamp: time.Now(),
		Data:      data,
	}

	select {
	case ch <- event:
	default:
		// 通道满了，丢弃事件
		log.Printf("事件通道满了，丢弃事件: %s", taskID)
	}
}

// closeEventChannel 关闭事件通道
func (s *TaskService) closeEventChannel(taskID string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if ch, exists := s.eventChannels[taskID]; exists {
		close(ch)
		delete(s.eventChannels, taskID)
		log.Printf("已关闭任务 %s 的事件通道", taskID)
	}
}

// GetEventChannel 获取事件通道
func (s *TaskService) GetEventChannel(taskID string) (chan models.TaskEvent, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	ch, exists := s.eventChannels[taskID]
	return ch, exists
}

// CancelTask 取消任务
func (s *TaskService) CancelTask(taskID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	task, exists := s.tasks[taskID]
	if !exists {
		return fmt.Errorf("任务不存在: %s", taskID)
	}

	if task.Status == models.TaskStatusCompleted || task.Status == models.TaskStatusFailed {
		return fmt.Errorf("任务已完成，无法取消")
	}

	task.cancel()
	task.Status = models.TaskStatusCancelled
	task.Message = "任务已取消"
	now := time.Now()
	task.CompletedAt = &now

	s.sendEvent(taskID, models.EventTypeCancel, "任务已取消", task.Progress, nil)

	return nil
}

// CleanupTask 清理任务
func (s *TaskService) CleanupTask(taskID string) (*models.CleanupResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	task, exists := s.tasks[taskID]
	if !exists {
		return nil, fmt.Errorf("任务不存在: %s", taskID)
	}

	// 取消任务（如果还在运行）
	if task.Status == models.TaskStatusPending || task.Status == models.TaskStatusProcessing {
		task.cancel()
	}

	// 清理文件
	cleanedFiles, err := s.storageService.CleanupTask(taskID)
	if err != nil {
		return nil, fmt.Errorf("清理文件失败: %w", err)
	}

	// 关闭事件通道
	if ch, exists := s.eventChannels[taskID]; exists {
		close(ch)
		delete(s.eventChannels, taskID)
	}

	// 删除任务记录
	delete(s.tasks, taskID)

	return &models.CleanupResponse{
		TaskID:       taskID,
		Cleaned:      true,
		CleanedFiles: cleanedFiles,
		Message:      "清理完成",
		CleanedAt:    time.Now().Format(time.RFC3339),
	}, nil
}
