package download

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

// DownloadManager 下载管理器
type DownloadManager struct {
	downloads    map[string]*DownloadTask
	queue        chan *DownloadTask
	workers      []*DownloadWorker
	mutex        sync.RWMutex
	config       DownloadConfig
	ctx          context.Context
	cancel       context.CancelFunc
	statistics   *DownloadStatistics
	statsMutex   sync.RWMutex
}

// DownloadConfig 下载配置
type DownloadConfig struct {
	MaxConcurrent   int           `json:"max_concurrent"`    // 最大并发下载数
	ChunkSize       int64         `json:"chunk_size"`        // 分块大小
	MaxRetries      int           `json:"max_retries"`       // 最大重试次数
	RetryDelay      time.Duration `json:"retry_delay"`       // 重试延迟
	Timeout         time.Duration `json:"timeout"`           // 超时时间
	DownloadDir     string        `json:"download_dir"`      // 下载目录
	TempDir         string        `json:"temp_dir"`          // 临时目录
	MaxFileSize     int64         `json:"max_file_size"`     // 最大文件大小
	CleanupInterval time.Duration `json:"cleanup_interval"`  // 清理间隔
	KeepCompleted   time.Duration `json:"keep_completed"`    // 保留已完成任务时间
	UserAgent       string        `json:"user_agent"`        // User-Agent
	Headers         map[string]string `json:"headers"`       // 自定义头部
}

// DownloadTask 下载任务
type DownloadTask struct {
	ID           string                 `json:"id"`
	URL          string                 `json:"url"`
	FileName     string                 `json:"file_name"`
	FilePath     string                 `json:"file_path"`
	TempPath     string                 `json:"temp_path"`
	FileSize     int64                  `json:"file_size"`
	Downloaded   int64                  `json:"downloaded"`
	Status       DownloadStatus         `json:"status"`
	Progress     float64                `json:"progress"`
	Speed        int64                  `json:"speed"`
	ETA          time.Duration          `json:"eta"`
	CreatedAt    time.Time              `json:"created_at"`
	StartedAt    *time.Time             `json:"started_at,omitempty"`
	CompletedAt  *time.Time             `json:"completed_at,omitempty"`
	LastUpdate   time.Time              `json:"last_update"`
	Retries      int                    `json:"retries"`
	Error        string                 `json:"error,omitempty"`
	Checksum     string                 `json:"checksum,omitempty"`
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
	Chunks       []*DownloadChunk       `json:"chunks,omitempty"`
	SupportsRange bool                  `json:"supports_range"`
	mutex        sync.RWMutex           `json:"-"`
	cancel       context.CancelFunc     `json:"-"`
}

// DownloadChunk 下载分块
type DownloadChunk struct {
	Index     int           `json:"index"`
	Start     int64         `json:"start"`
	End       int64         `json:"end"`
	Size      int64         `json:"size"`
	Downloaded int64        `json:"downloaded"`
	Status    ChunkStatus   `json:"status"`
	Retries   int           `json:"retries"`
	Error     string        `json:"error,omitempty"`
	StartTime time.Time     `json:"start_time"`
	EndTime   *time.Time    `json:"end_time,omitempty"`
}

// DownloadStatus 下载状态
type DownloadStatus string

const (
	DownloadStatusPending    DownloadStatus = "pending"
	DownloadStatusDownloading DownloadStatus = "downloading"
	DownloadStatusPaused     DownloadStatus = "paused"
	DownloadStatusCompleted  DownloadStatus = "completed"
	DownloadStatusFailed     DownloadStatus = "failed"
	DownloadStatusCancelled  DownloadStatus = "cancelled"
)

// ChunkStatus 分块状态
type ChunkStatus string

const (
	ChunkStatusPending    ChunkStatus = "pending"
	ChunkStatusDownloading ChunkStatus = "downloading"
	ChunkStatusCompleted  ChunkStatus = "completed"
	ChunkStatusFailed     ChunkStatus = "failed"
)

// DownloadRequest 下载请求
type DownloadRequest struct {
	URL      string                 `json:"url"`
	FileName string                 `json:"file_name,omitempty"`
	Headers  map[string]string      `json:"headers,omitempty"`
	Metadata map[string]interface{} `json:"metadata,omitempty"`
	Priority int                    `json:"priority,omitempty"`
}

// DownloadResponse 下载响应
type DownloadResponse struct {
	TaskID   string `json:"task_id"`
	FileName string `json:"file_name"`
	FileSize int64  `json:"file_size"`
	Status   DownloadStatus `json:"status"`
}

// DownloadWorker 下载工作器
type DownloadWorker struct {
	id      int
	manager *DownloadManager
	client  *http.Client
	ctx     context.Context
	cancel  context.CancelFunc
}

// DownloadStatistics 下载统计
type DownloadStatistics struct {
	TotalTasks      int64 `json:"total_tasks"`
	CompletedTasks  int64 `json:"completed_tasks"`
	FailedTasks     int64 `json:"failed_tasks"`
	CancelledTasks  int64 `json:"cancelled_tasks"`
	TotalBytes      int64 `json:"total_bytes"`
	DownloadedBytes int64 `json:"downloaded_bytes"`
	AverageSpeed    int64 `json:"average_speed"`
	ActiveTasks     int64 `json:"active_tasks"`
	QueuedTasks     int64 `json:"queued_tasks"`
}

// NewDownloadManager 创建下载管理器
func NewDownloadManager(config DownloadConfig) (*DownloadManager, error) {
	// 设置默认值
	if config.MaxConcurrent == 0 {
		config.MaxConcurrent = 3
	}
	if config.ChunkSize == 0 {
		config.ChunkSize = 1024 * 1024 // 1MB
	}
	if config.MaxRetries == 0 {
		config.MaxRetries = 3
	}
	if config.RetryDelay == 0 {
		config.RetryDelay = 5 * time.Second
	}
	if config.Timeout == 0 {
		config.Timeout = 30 * time.Second
	}
	if config.DownloadDir == "" {
		config.DownloadDir = "downloads"
	}
	if config.TempDir == "" {
		config.TempDir = "temp/downloads"
	}
	if config.MaxFileSize == 0 {
		config.MaxFileSize = 1024 * 1024 * 1024 // 1GB
	}
	if config.CleanupInterval == 0 {
		config.CleanupInterval = 1 * time.Hour
	}
	if config.KeepCompleted == 0 {
		config.KeepCompleted = 24 * time.Hour
	}
	if config.UserAgent == "" {
		config.UserAgent = "DownloadManager/1.0"
	}

	// 创建目录
	if err := os.MkdirAll(config.DownloadDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create download directory: %w", err)
	}
	if err := os.MkdirAll(config.TempDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create temp directory: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	dm := &DownloadManager{
		downloads:  make(map[string]*DownloadTask),
		queue:      make(chan *DownloadTask, 100),
		config:     config,
		ctx:        ctx,
		cancel:     cancel,
		statistics: &DownloadStatistics{},
	}

	// 启动工作器
	dm.startWorkers()

	// 启动清理任务
	go dm.cleanupLoop()

	return dm, nil
}

// AddDownload 添加下载任务
func (dm *DownloadManager) AddDownload(req DownloadRequest) (*DownloadResponse, error) {
	// 生成任务ID
	taskID := dm.generateTaskID()

	// 获取文件信息
	fileInfo, err := dm.getFileInfo(req.URL, req.Headers)
	if err != nil {
		return nil, fmt.Errorf("failed to get file info: %w", err)
	}

	// 检查文件大小
	if fileInfo.Size > dm.config.MaxFileSize {
		return nil, fmt.Errorf("file size exceeds maximum allowed size")
	}

	// 确定文件名
	fileName := req.FileName
	if fileName == "" {
		fileName = dm.extractFileName(req.URL, fileInfo.ContentDisposition)
	}

	// 创建下载任务
	task := &DownloadTask{
		ID:            taskID,
		URL:           req.URL,
		FileName:      fileName,
		FilePath:      filepath.Join(dm.config.DownloadDir, fileName),
		TempPath:      filepath.Join(dm.config.TempDir, taskID),
		FileSize:      fileInfo.Size,
		Downloaded:    0,
		Status:        DownloadStatusPending,
		Progress:      0,
		CreatedAt:     time.Now(),
		LastUpdate:    time.Now(),
		Retries:       0,
		Metadata:      req.Metadata,
		SupportsRange: fileInfo.SupportsRange,
	}

	// 创建分块（如果支持范围请求）
	if fileInfo.SupportsRange && fileInfo.Size > dm.config.ChunkSize {
		task.Chunks = dm.createChunks(fileInfo.Size)
	}

	// 保存任务
	dm.mutex.Lock()
	dm.downloads[taskID] = task
	dm.mutex.Unlock()

	// 更新统计
	dm.statsMutex.Lock()
	dm.statistics.TotalTasks++
	dm.statistics.TotalBytes += fileInfo.Size
	dm.statistics.QueuedTasks++
	dm.statsMutex.Unlock()

	// 添加到队列
	select {
	case dm.queue <- task:
	default:
		return nil, fmt.Errorf("download queue is full")
	}

	return &DownloadResponse{
		TaskID:   taskID,
		FileName: fileName,
		FileSize: fileInfo.Size,
		Status:   DownloadStatusPending,
	}, nil
}

// GetDownload 获取下载任务
func (dm *DownloadManager) GetDownload(taskID string) (*DownloadTask, error) {
	dm.mutex.RLock()
	task, exists := dm.downloads[taskID]
	dm.mutex.RUnlock()

	if !exists {
		return nil, fmt.Errorf("download task not found")
	}

	// 返回副本以避免并发问题
	task.mutex.RLock()
	taskCopy := *task
	task.mutex.RUnlock()

	return &taskCopy, nil
}

// PauseDownload 暂停下载
func (dm *DownloadManager) PauseDownload(taskID string) error {
	dm.mutex.RLock()
	task, exists := dm.downloads[taskID]
	dm.mutex.RUnlock()

	if !exists {
		return fmt.Errorf("download task not found")
	}

	task.mutex.Lock()
	defer task.mutex.Unlock()

	if task.Status != DownloadStatusDownloading {
		return fmt.Errorf("task is not downloading")
	}

	task.Status = DownloadStatusPaused
	if task.cancel != nil {
		task.cancel()
	}

	return nil
}

// ResumeDownload 恢复下载
func (dm *DownloadManager) ResumeDownload(taskID string) error {
	dm.mutex.RLock()
	task, exists := dm.downloads[taskID]
	dm.mutex.RUnlock()

	if !exists {
		return fmt.Errorf("download task not found")
	}

	task.mutex.Lock()
	defer task.mutex.Unlock()

	if task.Status != DownloadStatusPaused {
		return fmt.Errorf("task is not paused")
	}

	task.Status = DownloadStatusPending

	// 重新添加到队列
	select {
	case dm.queue <- task:
		return nil
	default:
		return fmt.Errorf("download queue is full")
	}
}

// CancelDownload 取消下载
func (dm *DownloadManager) CancelDownload(taskID string) error {
	dm.mutex.Lock()
	task, exists := dm.downloads[taskID]
	if exists {
		delete(dm.downloads, taskID)
	}
	dm.mutex.Unlock()

	if !exists {
		return fmt.Errorf("download task not found")
	}

	task.mutex.Lock()
	task.Status = DownloadStatusCancelled
	if task.cancel != nil {
		task.cancel()
	}
	task.mutex.Unlock()

	// 清理临时文件
	os.Remove(task.TempPath)

	// 更新统计
	dm.statsMutex.Lock()
	dm.statistics.CancelledTasks++
	dm.statsMutex.Unlock()

	return nil
}

// GetAllDownloads 获取所有下载任务
func (dm *DownloadManager) GetAllDownloads() []*DownloadTask {
	dm.mutex.RLock()
	defer dm.mutex.RUnlock()

	tasks := make([]*DownloadTask, 0, len(dm.downloads))
	for _, task := range dm.downloads {
		task.mutex.RLock()
		taskCopy := *task
		task.mutex.RUnlock()
		tasks = append(tasks, &taskCopy)
	}

	return tasks
}

// GetStatistics 获取下载统计
func (dm *DownloadManager) GetStatistics() *DownloadStatistics {
	dm.statsMutex.RLock()
	defer dm.statsMutex.RUnlock()

	stats := *dm.statistics
	return &stats
}

// startWorkers 启动工作器
func (dm *DownloadManager) startWorkers() {
	dm.workers = make([]*DownloadWorker, dm.config.MaxConcurrent)
	for i := 0; i < dm.config.MaxConcurrent; i++ {
		worker := &DownloadWorker{
			id:      i,
			manager: dm,
			client: &http.Client{
				Timeout: dm.config.Timeout,
			},
		}
		worker.ctx, worker.cancel = context.WithCancel(dm.ctx)
		dm.workers[i] = worker
		go worker.run()
	}
}

// run 工作器运行循环
func (w *DownloadWorker) run() {
	for {
		select {
		case <-w.ctx.Done():
			return
		case task := <-w.manager.queue:
			w.processTask(task)
		}
	}
}

// processTask 处理下载任务
func (w *DownloadWorker) processTask(task *DownloadTask) {
	task.mutex.Lock()
	if task.Status != DownloadStatusPending {
		task.mutex.Unlock()
		return
	}

	task.Status = DownloadStatusDownloading
	now := time.Now()
	task.StartedAt = &now
	task.LastUpdate = now
	task.mutex.Unlock()

	// 更新统计
	w.manager.statsMutex.Lock()
	w.manager.statistics.QueuedTasks--
	w.manager.statistics.ActiveTasks++
	w.manager.statsMutex.Unlock()

	// 执行下载
	err := w.downloadFile(task)

	task.mutex.Lock()
	if err != nil {
		task.Error = err.Error()
		task.Retries++

		if task.Retries < w.manager.config.MaxRetries {
			// 重试
			task.Status = DownloadStatusPending
			task.mutex.Unlock()

			// 延迟后重新添加到队列
			go func() {
				time.Sleep(w.manager.config.RetryDelay)
				select {
				case w.manager.queue <- task:
				default:
					// 队列满，标记为失败
					task.mutex.Lock()
					task.Status = DownloadStatusFailed
					task.mutex.Unlock()
				}
			}()
		} else {
			// 达到最大重试次数，标记为失败
			task.Status = DownloadStatusFailed
			now := time.Now()
			task.CompletedAt = &now
			task.mutex.Unlock()

			// 更新统计
			w.manager.statsMutex.Lock()
			w.manager.statistics.FailedTasks++
			w.manager.statsMutex.Unlock()
		}
	} else {
		// 下载成功
		task.Status = DownloadStatusCompleted
		task.Progress = 100
		now := time.Now()
		task.CompletedAt = &now
		task.mutex.Unlock()

		// 移动文件到最终位置
		w.finalizeDownload(task)

		// 更新统计
		w.manager.statsMutex.Lock()
		w.manager.statistics.CompletedTasks++
		w.manager.statistics.DownloadedBytes += task.FileSize
		w.manager.statsMutex.Unlock()
	}

	// 更新统计
	w.manager.statsMutex.Lock()
	w.manager.statistics.ActiveTasks--
	w.manager.statsMutex.Unlock()
}

// downloadFile 下载文件
func (w *DownloadWorker) downloadFile(task *DownloadTask) error {
	ctx, cancel := context.WithCancel(w.ctx)
	task.cancel = cancel
	defer cancel()

	if len(task.Chunks) > 0 {
		// 分块下载
		return w.downloadWithChunks(ctx, task)
	} else {
		// 单线程下载
		return w.downloadSingle(ctx, task)
	}
}

// downloadSingle 单线程下载
func (w *DownloadWorker) downloadSingle(ctx context.Context, task *DownloadTask) error {
	req, err := http.NewRequestWithContext(ctx, "GET", task.URL, nil)
	if err != nil {
		return err
	}

	// 设置头部
	req.Header.Set("User-Agent", w.manager.config.UserAgent)
	for k, v := range w.manager.config.Headers {
		req.Header.Set(k, v)
	}

	// 支持断点续传
	if task.Downloaded > 0 {
		req.Header.Set("Range", fmt.Sprintf("bytes=%d-", task.Downloaded))
	}

	resp, err := w.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// 检查状态码
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusPartialContent {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	// 创建或打开临时文件
	file, err := os.OpenFile(task.TempPath, os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer file.Close()

	// 定位到正确位置
	if task.Downloaded > 0 {
		if _, err := file.Seek(task.Downloaded, 0); err != nil {
			return err
		}
	}

	// 下载数据
	buffer := make([]byte, 32*1024) // 32KB buffer
	lastUpdate := time.Now()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		n, err := resp.Body.Read(buffer)
		if n > 0 {
			if _, writeErr := file.Write(buffer[:n]); writeErr != nil {
				return writeErr
			}

			task.mutex.Lock()
			task.Downloaded += int64(n)
			task.Progress = float64(task.Downloaded) / float64(task.FileSize) * 100
			task.LastUpdate = time.Now()

			// 计算速度
			if time.Since(lastUpdate) >= time.Second {
				elapsed := time.Since(lastUpdate)
				task.Speed = int64(float64(n) / elapsed.Seconds())
				lastUpdate = time.Now()

				// 计算ETA
				if task.Speed > 0 {
					remaining := task.FileSize - task.Downloaded
					task.ETA = time.Duration(float64(remaining)/float64(task.Speed)) * time.Second
				}
			}
			task.mutex.Unlock()
		}

		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
	}

	return nil
}

// downloadWithChunks 分块下载
func (w *DownloadWorker) downloadWithChunks(ctx context.Context, task *DownloadTask) error {
	// 创建临时文件
	file, err := os.Create(task.TempPath)
	if err != nil {
		return err
	}
	file.Close()

	// 并发下载分块
	var wg sync.WaitGroup
	errorChan := make(chan error, len(task.Chunks))
	semaphore := make(chan struct{}, 3) // 限制并发数

	for _, chunk := range task.Chunks {
		if chunk.Status == ChunkStatusCompleted {
			continue
		}

		wg.Add(1)
		go func(chunk *DownloadChunk) {
			defer wg.Done()
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			if err := w.downloadChunk(ctx, task, chunk); err != nil {
				errorChan <- err
			}
		}(chunk)
	}

	wg.Wait()
	close(errorChan)

	// 检查错误
	for err := range errorChan {
		if err != nil {
			return err
		}
	}

	return nil
}

// downloadChunk 下载分块
func (w *DownloadWorker) downloadChunk(ctx context.Context, task *DownloadTask, chunk *DownloadChunk) error {
	req, err := http.NewRequestWithContext(ctx, "GET", task.URL, nil)
	if err != nil {
		return err
	}

	// 设置范围头部
	req.Header.Set("Range", fmt.Sprintf("bytes=%d-%d", chunk.Start+chunk.Downloaded, chunk.End))
	req.Header.Set("User-Agent", w.manager.config.UserAgent)

	resp, err := w.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusPartialContent {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	// 打开临时文件
	file, err := os.OpenFile(task.TempPath, os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer file.Close()

	// 定位到分块位置
	if _, err := file.Seek(chunk.Start+chunk.Downloaded, 0); err != nil {
		return err
	}

	// 下载分块数据
	buffer := make([]byte, 32*1024)
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		n, err := resp.Body.Read(buffer)
		if n > 0 {
			if _, writeErr := file.Write(buffer[:n]); writeErr != nil {
				return writeErr
			}
			chunk.Downloaded += int64(n)
		}

		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
	}

	chunk.Status = ChunkStatusCompleted
	now := time.Now()
	chunk.EndTime = &now

	return nil
}

// finalizeDownload 完成下载
func (w *DownloadWorker) finalizeDownload(task *DownloadTask) error {
	// 移动文件到最终位置
	return os.Rename(task.TempPath, task.FilePath)
}

// getFileInfo 获取文件信息
func (dm *DownloadManager) getFileInfo(url string, headers map[string]string) (*FileInfo, error) {
	req, err := http.NewRequest("HEAD", url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("User-Agent", dm.config.UserAgent)
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	for k, v := range dm.config.Headers {
		req.Header.Set(k, v)
	}

	client := &http.Client{Timeout: dm.config.Timeout}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	fileInfo := &FileInfo{
		SupportsRange: resp.Header.Get("Accept-Ranges") == "bytes",
		ContentDisposition: resp.Header.Get("Content-Disposition"),
	}

	// 获取文件大小
	contentLength := resp.Header.Get("Content-Length")
	if contentLength != "" {
		size, err := strconv.ParseInt(contentLength, 10, 64)
		if err == nil {
			fileInfo.Size = size
		}
	}

	return fileInfo, nil
}

// FileInfo 文件信息
type FileInfo struct {
	Size               int64
	SupportsRange      bool
	ContentDisposition string
}

// extractFileName 提取文件名
func (dm *DownloadManager) extractFileName(url, contentDisposition string) string {
	// 从Content-Disposition头部提取
	if contentDisposition != "" {
		if strings.Contains(contentDisposition, "filename=") {
			parts := strings.Split(contentDisposition, "filename=")
			if len(parts) > 1 {
				filename := strings.Trim(parts[1], `"'`)
				if filename != "" {
					return filename
				}
			}
		}
	}

	// 从URL提取
	parts := strings.Split(url, "/")
	if len(parts) > 0 {
		lastPart := parts[len(parts)-1]
		if strings.Contains(lastPart, ".") {
			return lastPart
		}
	}

	// 默认文件名
	return fmt.Sprintf("download_%d", time.Now().Unix())
}

// createChunks 创建分块
func (dm *DownloadManager) createChunks(fileSize int64) []*DownloadChunk {
	chunkCount := int((fileSize + dm.config.ChunkSize - 1) / dm.config.ChunkSize)
	chunks := make([]*DownloadChunk, chunkCount)

	for i := 0; i < chunkCount; i++ {
		start := int64(i) * dm.config.ChunkSize
		end := start + dm.config.ChunkSize - 1
		if end >= fileSize {
			end = fileSize - 1
		}

		chunks[i] = &DownloadChunk{
			Index:     i,
			Start:     start,
			End:       end,
			Size:      end - start + 1,
			Downloaded: 0,
			Status:    ChunkStatusPending,
			StartTime: time.Now(),
		}
	}

	return chunks
}

// generateTaskID 生成任务ID
func (dm *DownloadManager) generateTaskID() string {
	return fmt.Sprintf("%d_%s", time.Now().UnixNano(), dm.generateRandomString(8))
}

// generateRandomString 生成随机字符串
func (dm *DownloadManager) generateRandomString(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[time.Now().UnixNano()%int64(len(charset))]
	}
	return string(b)
}

// cleanupLoop 清理循环
func (dm *DownloadManager) cleanupLoop() {
	ticker := time.NewTicker(dm.config.CleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-dm.ctx.Done():
			return
		case <-ticker.C:
			dm.cleanup()
		}
	}
}

// cleanup 清理过期任务
func (dm *DownloadManager) cleanup() {
	now := time.Now()
	expiredTasks := make([]*DownloadTask, 0)

	dm.mutex.Lock()
	for taskID, task := range dm.downloads {
		task.mutex.RLock()
		if task.Status == DownloadStatusCompleted && 
		   task.CompletedAt != nil && 
		   now.Sub(*task.CompletedAt) > dm.config.KeepCompleted {
			expiredTasks = append(expiredTasks, task)
			delete(dm.downloads, taskID)
		}
		task.mutex.RUnlock()
	}
	dm.mutex.Unlock()

	// 清理临时文件
	for _, task := range expiredTasks {
		os.Remove(task.TempPath)
	}
}

// Stop 停止下载管理器
func (dm *DownloadManager) Stop() {
	dm.cancel()

	// 停止所有工作器
	for _, worker := range dm.workers {
		worker.cancel()
	}

	// 取消所有下载
	dm.mutex.Lock()
	for _, task := range dm.downloads {
		task.mutex.Lock()
		if task.cancel != nil {
			task.cancel()
		}
		task.mutex.Unlock()
	}
	dm.mutex.Unlock()
}