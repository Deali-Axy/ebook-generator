package upload

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// StreamUploadService 流式上传服务
type StreamUploadService struct {
	uploads    map[string]*UploadSession
	mutex      sync.RWMutex
	config     UploadConfig
	tempDir    string
	cleanupTicker *time.Ticker
	stopCleanup   chan bool
}

// UploadConfig 上传配置
type UploadConfig struct {
	ChunkSize       int64         `json:"chunk_size"`        // 分块大小（字节）
	MaxFileSize     int64         `json:"max_file_size"`     // 最大文件大小
	MaxConcurrent   int           `json:"max_concurrent"`    // 最大并发上传数
	SessionTimeout  time.Duration `json:"session_timeout"`   // 会话超时时间
	CleanupInterval time.Duration `json:"cleanup_interval"`  // 清理间隔
	TempDir         string        `json:"temp_dir"`          // 临时目录
	AllowedTypes    []string      `json:"allowed_types"`     // 允许的文件类型
	ChecksumType    string        `json:"checksum_type"`     // 校验和类型
}

// UploadSession 上传会话
type UploadSession struct {
	ID           string                 `json:"id"`
	FileName     string                 `json:"file_name"`
	FileSize     int64                  `json:"file_size"`
	ChunkSize    int64                  `json:"chunk_size"`
	TotalChunks  int                    `json:"total_chunks"`
	Chunks       map[int]*ChunkInfo     `json:"chunks"`
	UploadedSize int64                  `json:"uploaded_size"`
	Status       UploadStatus           `json:"status"`
	CreatedAt    time.Time              `json:"created_at"`
	UpdatedAt    time.Time              `json:"updated_at"`
	Metadata     map[string]interface{} `json:"metadata"`
	Checksum     string                 `json:"checksum"`
	TempPath     string                 `json:"temp_path"`
	mutex        sync.RWMutex           `json:"-"`
}

// ChunkInfo 分块信息
type ChunkInfo struct {
	Index     int       `json:"index"`
	Size      int64     `json:"size"`
	Checksum  string    `json:"checksum"`
	Uploaded  bool      `json:"uploaded"`
	UploadedAt time.Time `json:"uploaded_at"`
	Retries   int       `json:"retries"`
}

// UploadStatus 上传状态
type UploadStatus string

const (
	UploadStatusInitialized UploadStatus = "initialized"
	UploadStatusUploading   UploadStatus = "uploading"
	UploadStatusCompleted   UploadStatus = "completed"
	UploadStatusFailed      UploadStatus = "failed"
	UploadStatusCancelled   UploadStatus = "cancelled"
	UploadStatusExpired     UploadStatus = "expired"
)

// UploadRequest 上传请求
type UploadRequest struct {
	FileName string                 `json:"file_name"`
	FileSize int64                  `json:"file_size"`
	Checksum string                 `json:"checksum"`
	Metadata map[string]interface{} `json:"metadata"`
}

// UploadResponse 上传响应
type UploadResponse struct {
	SessionID   string `json:"session_id"`
	ChunkSize   int64  `json:"chunk_size"`
	TotalChunks int    `json:"total_chunks"`
	UploadURL   string `json:"upload_url"`
}

// ChunkUploadRequest 分块上传请求
type ChunkUploadRequest struct {
	SessionID   string `json:"session_id"`
	ChunkIndex  int    `json:"chunk_index"`
	ChunkSize   int64  `json:"chunk_size"`
	Checksum    string `json:"checksum"`
	Data        []byte `json:"-"`
}

// ChunkUploadResponse 分块上传响应
type ChunkUploadResponse struct {
	Success     bool   `json:"success"`
	Message     string `json:"message"`
	NextChunk   int    `json:"next_chunk,omitempty"`
	Progress    float64 `json:"progress"`
	Completed   bool   `json:"completed"`
}

// UploadProgress 上传进度
type UploadProgress struct {
	SessionID      string        `json:"session_id"`
	FileName       string        `json:"file_name"`
	TotalSize      int64         `json:"total_size"`
	UploadedSize   int64         `json:"uploaded_size"`
	Progress       float64       `json:"progress"`
	Speed          int64         `json:"speed"`          // 字节/秒
	RemainingTime  time.Duration `json:"remaining_time"`
	UploadedChunks int           `json:"uploaded_chunks"`
	TotalChunks    int           `json:"total_chunks"`
	Status         UploadStatus  `json:"status"`
	StartTime      time.Time     `json:"start_time"`
	LastUpdate     time.Time     `json:"last_update"`
}

// NewStreamUploadService 创建流式上传服务
func NewStreamUploadService(config UploadConfig) (*StreamUploadService, error) {
	// 设置默认值
	if config.ChunkSize == 0 {
		config.ChunkSize = 1024 * 1024 // 1MB
	}
	if config.MaxFileSize == 0 {
		config.MaxFileSize = 100 * 1024 * 1024 // 100MB
	}
	if config.MaxConcurrent == 0 {
		config.MaxConcurrent = 10
	}
	if config.SessionTimeout == 0 {
		config.SessionTimeout = 24 * time.Hour
	}
	if config.CleanupInterval == 0 {
		config.CleanupInterval = 1 * time.Hour
	}
	if config.TempDir == "" {
		config.TempDir = "temp/uploads"
	}
	if config.ChecksumType == "" {
		config.ChecksumType = "md5"
	}

	// 创建临时目录
	if err := os.MkdirAll(config.TempDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create temp directory: %w", err)
	}

	sus := &StreamUploadService{
		uploads:     make(map[string]*UploadSession),
		config:      config,
		tempDir:     config.TempDir,
		stopCleanup: make(chan bool),
	}

	// 启动清理任务
	sus.startCleanup()

	return sus, nil
}

// InitiateUpload 初始化上传
func (sus *StreamUploadService) InitiateUpload(req UploadRequest) (*UploadResponse, error) {
	// 验证文件大小
	if req.FileSize > sus.config.MaxFileSize {
		return nil, fmt.Errorf("file size exceeds maximum allowed size")
	}

	// 验证文件类型
	if len(sus.config.AllowedTypes) > 0 {
		if !sus.isAllowedType(req.FileName) {
			return nil, fmt.Errorf("file type not allowed")
		}
	}

	// 检查并发上传数
	sus.mutex.RLock()
	activeUploads := 0
	for _, session := range sus.uploads {
		if session.Status == UploadStatusUploading {
			activeUploads++
		}
	}
	sus.mutex.RUnlock()

	if activeUploads >= sus.config.MaxConcurrent {
		return nil, fmt.Errorf("too many concurrent uploads")
	}

	// 生成会话ID
	sessionID := sus.generateSessionID()

	// 计算分块数
	totalChunks := int((req.FileSize + sus.config.ChunkSize - 1) / sus.config.ChunkSize)

	// 创建上传会话
	session := &UploadSession{
		ID:           sessionID,
		FileName:     req.FileName,
		FileSize:     req.FileSize,
		ChunkSize:    sus.config.ChunkSize,
		TotalChunks:  totalChunks,
		Chunks:       make(map[int]*ChunkInfo),
		UploadedSize: 0,
		Status:       UploadStatusInitialized,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
		Metadata:     req.Metadata,
		Checksum:     req.Checksum,
		TempPath:     filepath.Join(sus.tempDir, sessionID),
	}

	// 初始化分块信息
	for i := 0; i < totalChunks; i++ {
		chunkSize := sus.config.ChunkSize
		if i == totalChunks-1 {
			// 最后一个分块可能较小
			chunkSize = req.FileSize - int64(i)*sus.config.ChunkSize
		}
		session.Chunks[i] = &ChunkInfo{
			Index:    i,
			Size:     chunkSize,
			Uploaded: false,
			Retries:  0,
		}
	}

	// 创建临时文件
	if err := sus.createTempFile(session); err != nil {
		return nil, fmt.Errorf("failed to create temp file: %w", err)
	}

	// 保存会话
	sus.mutex.Lock()
	sus.uploads[sessionID] = session
	sus.mutex.Unlock()

	return &UploadResponse{
		SessionID:   sessionID,
		ChunkSize:   sus.config.ChunkSize,
		TotalChunks: totalChunks,
		UploadURL:   fmt.Sprintf("/upload/chunk/%s", sessionID),
	}, nil
}

// UploadChunk 上传分块
func (sus *StreamUploadService) UploadChunk(req ChunkUploadRequest) (*ChunkUploadResponse, error) {
	// 获取上传会话
	sus.mutex.RLock()
	session, exists := sus.uploads[req.SessionID]
	sus.mutex.RUnlock()

	if !exists {
		return nil, fmt.Errorf("upload session not found")
	}

	session.mutex.Lock()
	defer session.mutex.Unlock()

	// 检查会话状态
	if session.Status != UploadStatusInitialized && session.Status != UploadStatusUploading {
		return nil, fmt.Errorf("invalid session status: %s", session.Status)
	}

	// 检查分块索引
	if req.ChunkIndex < 0 || req.ChunkIndex >= session.TotalChunks {
		return nil, fmt.Errorf("invalid chunk index: %d", req.ChunkIndex)
	}

	chunk, exists := session.Chunks[req.ChunkIndex]
	if !exists {
		return nil, fmt.Errorf("chunk not found: %d", req.ChunkIndex)
	}

	// 检查分块是否已上传
	if chunk.Uploaded {
		return &ChunkUploadResponse{
			Success:   true,
			Message:   "Chunk already uploaded",
			Progress:  sus.calculateProgress(session),
			Completed: sus.isUploadCompleted(session),
		}, nil
	}

	// 验证分块大小
	if int64(len(req.Data)) != chunk.Size {
		return nil, fmt.Errorf("chunk size mismatch: expected %d, got %d", chunk.Size, len(req.Data))
	}

	// 验证校验和
	if req.Checksum != "" {
		calculatedChecksum := sus.calculateChecksum(req.Data)
		if req.Checksum != calculatedChecksum {
			return nil, fmt.Errorf("checksum mismatch")
		}
	}

	// 写入分块数据
	if err := sus.writeChunk(session, req.ChunkIndex, req.Data); err != nil {
		chunk.Retries++
		return nil, fmt.Errorf("failed to write chunk: %w", err)
	}

	// 更新分块状态
	chunk.Uploaded = true
	chunk.UploadedAt = time.Now()
	chunk.Checksum = req.Checksum

	// 更新会话状态
	session.UploadedSize += chunk.Size
	session.UpdatedAt = time.Now()
	session.Status = UploadStatusUploading

	// 检查是否上传完成
	completed := sus.isUploadCompleted(session)
	if completed {
		if err := sus.finalizeUpload(session); err != nil {
			session.Status = UploadStatusFailed
			return nil, fmt.Errorf("failed to finalize upload: %w", err)
		}
		session.Status = UploadStatusCompleted
	}

	// 计算下一个未上传的分块
	nextChunk := -1
	for i := 0; i < session.TotalChunks; i++ {
		if !session.Chunks[i].Uploaded {
			nextChunk = i
			break
		}
	}

	response := &ChunkUploadResponse{
		Success:   true,
		Message:   "Chunk uploaded successfully",
		Progress:  sus.calculateProgress(session),
		Completed: completed,
	}

	if nextChunk != -1 {
		response.NextChunk = nextChunk
	}

	return response, nil
}

// GetUploadProgress 获取上传进度
func (sus *StreamUploadService) GetUploadProgress(sessionID string) (*UploadProgress, error) {
	sus.mutex.RLock()
	session, exists := sus.uploads[sessionID]
	sus.mutex.RUnlock()

	if !exists {
		return nil, fmt.Errorf("upload session not found")
	}

	session.mutex.RLock()
	defer session.mutex.RUnlock()

	// 计算上传速度
	var speed int64
	elapsed := time.Since(session.CreatedAt)
	if elapsed.Seconds() > 0 {
		speed = int64(float64(session.UploadedSize) / elapsed.Seconds())
	}

	// 计算剩余时间
	var remainingTime time.Duration
	if speed > 0 {
		remainingBytes := session.FileSize - session.UploadedSize
		remainingTime = time.Duration(float64(remainingBytes)/float64(speed)) * time.Second
	}

	// 计算已上传分块数
	uploadedChunks := 0
	for _, chunk := range session.Chunks {
		if chunk.Uploaded {
			uploadedChunks++
		}
	}

	return &UploadProgress{
		SessionID:      sessionID,
		FileName:       session.FileName,
		TotalSize:      session.FileSize,
		UploadedSize:   session.UploadedSize,
		Progress:       sus.calculateProgress(session),
		Speed:          speed,
		RemainingTime:  remainingTime,
		UploadedChunks: uploadedChunks,
		TotalChunks:    session.TotalChunks,
		Status:         session.Status,
		StartTime:      session.CreatedAt,
		LastUpdate:     session.UpdatedAt,
	}, nil
}

// CancelUpload 取消上传
func (sus *StreamUploadService) CancelUpload(sessionID string) error {
	sus.mutex.Lock()
	session, exists := sus.uploads[sessionID]
	if exists {
		session.Status = UploadStatusCancelled
		delete(sus.uploads, sessionID)
	}
	sus.mutex.Unlock()

	if !exists {
		return fmt.Errorf("upload session not found")
	}

	// 删除临时文件
	return sus.cleanupSession(session)
}

// ResumeUpload 恢复上传
func (sus *StreamUploadService) ResumeUpload(sessionID string) (*UploadResponse, error) {
	sus.mutex.RLock()
	session, exists := sus.uploads[sessionID]
	sus.mutex.RUnlock()

	if !exists {
		return nil, fmt.Errorf("upload session not found")
	}

	session.mutex.Lock()
	defer session.mutex.Unlock()

	if session.Status == UploadStatusCompleted {
		return nil, fmt.Errorf("upload already completed")
	}

	if session.Status == UploadStatusCancelled {
		return nil, fmt.Errorf("upload was cancelled")
	}

	// 重置状态
	session.Status = UploadStatusUploading
	session.UpdatedAt = time.Now()

	return &UploadResponse{
		SessionID:   sessionID,
		ChunkSize:   session.ChunkSize,
		TotalChunks: session.TotalChunks,
		UploadURL:   fmt.Sprintf("/upload/chunk/%s", sessionID),
	}, nil
}

// GetMissingChunks 获取缺失的分块
func (sus *StreamUploadService) GetMissingChunks(sessionID string) ([]int, error) {
	sus.mutex.RLock()
	session, exists := sus.uploads[sessionID]
	sus.mutex.RUnlock()

	if !exists {
		return nil, fmt.Errorf("upload session not found")
	}

	session.mutex.RLock()
	defer session.mutex.RUnlock()

	var missingChunks []int
	for i := 0; i < session.TotalChunks; i++ {
		if !session.Chunks[i].Uploaded {
			missingChunks = append(missingChunks, i)
		}
	}

	return missingChunks, nil
}

// isAllowedType 检查文件类型是否允许
func (sus *StreamUploadService) isAllowedType(fileName string) bool {
	ext := strings.ToLower(filepath.Ext(fileName))
	for _, allowedType := range sus.config.AllowedTypes {
		if ext == allowedType {
			return true
		}
	}
	return false
}

// generateSessionID 生成会话ID
func (sus *StreamUploadService) generateSessionID() string {
	return fmt.Sprintf("%d_%s", time.Now().UnixNano(), sus.generateRandomString(8))
}

// generateRandomString 生成随机字符串
func (sus *StreamUploadService) generateRandomString(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[time.Now().UnixNano()%int64(len(charset))]
	}
	return string(b)
}

// calculateChecksum 计算校验和
func (sus *StreamUploadService) calculateChecksum(data []byte) string {
	switch sus.config.ChecksumType {
	case "md5":
		hash := md5.Sum(data)
		return hex.EncodeToString(hash[:])
	default:
		return ""
	}
}

// createTempFile 创建临时文件
func (sus *StreamUploadService) createTempFile(session *UploadSession) error {
	file, err := os.Create(session.TempPath)
	if err != nil {
		return err
	}
	defer file.Close()

	// 预分配文件空间
	return file.Truncate(session.FileSize)
}

// writeChunk 写入分块
func (sus *StreamUploadService) writeChunk(session *UploadSession, chunkIndex int, data []byte) error {
	file, err := os.OpenFile(session.TempPath, os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer file.Close()

	// 计算写入位置
	offset := int64(chunkIndex) * session.ChunkSize
	_, err = file.Seek(offset, 0)
	if err != nil {
		return err
	}

	// 写入数据
	_, err = file.Write(data)
	return err
}

// calculateProgress 计算上传进度
func (sus *StreamUploadService) calculateProgress(session *UploadSession) float64 {
	if session.FileSize == 0 {
		return 0
	}
	return float64(session.UploadedSize) / float64(session.FileSize) * 100
}

// isUploadCompleted 检查上传是否完成
func (sus *StreamUploadService) isUploadCompleted(session *UploadSession) bool {
	for _, chunk := range session.Chunks {
		if !chunk.Uploaded {
			return false
		}
	}
	return true
}

// finalizeUpload 完成上传
func (sus *StreamUploadService) finalizeUpload(session *UploadSession) error {
	// 验证文件完整性
	if session.Checksum != "" {
		if err := sus.verifyFileChecksum(session); err != nil {
			return fmt.Errorf("file verification failed: %w", err)
		}
	}

	// 这里可以添加其他完成逻辑，如移动文件到最终位置
	return nil
}

// verifyFileChecksum 验证文件校验和
func (sus *StreamUploadService) verifyFileChecksum(session *UploadSession) error {
	file, err := os.Open(session.TempPath)
	if err != nil {
		return err
	}
	defer file.Close()

	hash := md5.New()
	if _, err := io.Copy(hash, file); err != nil {
		return err
	}

	calculatedChecksum := hex.EncodeToString(hash.Sum(nil))
	if calculatedChecksum != session.Checksum {
		return fmt.Errorf("checksum mismatch: expected %s, got %s", session.Checksum, calculatedChecksum)
	}

	return nil
}

// startCleanup 启动清理任务
func (sus *StreamUploadService) startCleanup() {
	sus.cleanupTicker = time.NewTicker(sus.config.CleanupInterval)
	go func() {
		for {
			select {
			case <-sus.cleanupTicker.C:
				sus.cleanup()
			case <-sus.stopCleanup:
				sus.cleanupTicker.Stop()
				return
			}
		}
	}()
}

// cleanup 清理过期会话
func (sus *StreamUploadService) cleanup() {
	now := time.Now()
	expiredSessions := make([]*UploadSession, 0)

	sus.mutex.Lock()
	for sessionID, session := range sus.uploads {
		if now.Sub(session.UpdatedAt) > sus.config.SessionTimeout {
			session.Status = UploadStatusExpired
			expiredSessions = append(expiredSessions, session)
			delete(sus.uploads, sessionID)
		}
	}
	sus.mutex.Unlock()

	// 清理过期会话的临时文件
	for _, session := range expiredSessions {
		sus.cleanupSession(session)
	}
}

// cleanupSession 清理会话
func (sus *StreamUploadService) cleanupSession(session *UploadSession) error {
	return os.Remove(session.TempPath)
}

// Stop 停止服务
func (sus *StreamUploadService) Stop() {
	close(sus.stopCleanup)

	// 清理所有会话
	sus.mutex.Lock()
	for _, session := range sus.uploads {
		sus.cleanupSession(session)
	}
	sus.uploads = make(map[string]*UploadSession)
	sus.mutex.Unlock()
}

// GetAllSessions 获取所有会话
func (sus *StreamUploadService) GetAllSessions() map[string]*UploadSession {
	sus.mutex.RLock()
	defer sus.mutex.RUnlock()

	result := make(map[string]*UploadSession)
	for id, session := range sus.uploads {
		// 返回副本以避免并发问题
		sessionCopy := *session
		result[id] = &sessionCopy
	}
	return result
}

// GetSessionsByStatus 根据状态获取会话
func (sus *StreamUploadService) GetSessionsByStatus(status UploadStatus) []*UploadSession {
	sus.mutex.RLock()
	defer sus.mutex.RUnlock()

	var result []*UploadSession
	for _, session := range sus.uploads {
		if session.Status == status {
			sessionCopy := *session
			result = append(result, &sessionCopy)
		}
	}
	return result
}