package cleanup

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/Deali-Axy/ebook-generator/internal/web/services"
)

// CleanupConfig 清理配置
type CleanupConfig struct {
	UploadDir       string        // 上传目录
	OutputDir       string        // 输出目录
	TempDir         string        // 临时目录
	MaxFileAge      time.Duration // 文件最大保存时间
	CleanupInterval time.Duration // 清理间隔
	MaxDiskUsage    int64         // 最大磁盘使用量（字节）
	Enabled         bool          // 是否启用自动清理
}

// CleanupService 清理服务
type CleanupService struct {
	config      CleanupConfig
	taskService *services.TaskService
	stopChan    chan struct{}
	running     bool
}

// CleanupStats 清理统计
type CleanupStats struct {
	FilesDeleted    int   `json:"files_deleted"`
	BytesFreed      int64 `json:"bytes_freed"`
	TasksCleaned    int   `json:"tasks_cleaned"`
	Duration        time.Duration `json:"duration"`
	LastCleanupTime time.Time `json:"last_cleanup_time"`
	Errors          []string `json:"errors,omitempty"`
}

// NewCleanupService 创建清理服务
func NewCleanupService(config CleanupConfig, taskService *services.TaskService) *CleanupService {
	// 设置默认值
	if config.MaxFileAge == 0 {
		config.MaxFileAge = 24 * time.Hour // 默认24小时
	}
	if config.CleanupInterval == 0 {
		config.CleanupInterval = 1 * time.Hour // 默认1小时清理一次
	}
	if config.MaxDiskUsage == 0 {
		config.MaxDiskUsage = 10 * 1024 * 1024 * 1024 // 默认10GB
	}

	return &CleanupService{
		config:      config,
		taskService: taskService,
		stopChan:    make(chan struct{}),
	}
}

// Start 启动清理服务
func (cs *CleanupService) Start() error {
	if !cs.config.Enabled {
		log.Println("清理服务已禁用")
		return nil
	}

	if cs.running {
		return fmt.Errorf("清理服务已在运行")
	}

	cs.running = true
	log.Printf("启动清理服务，清理间隔: %v，文件最大保存时间: %v", cs.config.CleanupInterval, cs.config.MaxFileAge)

	// 立即执行一次清理
	go func() {
		if _, err := cs.RunCleanup(); err != nil {
			log.Printf("初始清理失败: %v", err)
		}
	}()

	// 启动定时清理
	go cs.scheduledCleanup()

	return nil
}

// Stop 停止清理服务
func (cs *CleanupService) Stop() {
	if !cs.running {
		return
	}

	log.Println("停止清理服务")
	close(cs.stopChan)
	cs.running = false
}

// scheduledCleanup 定时清理
func (cs *CleanupService) scheduledCleanup() {
	ticker := time.NewTicker(cs.config.CleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if stats, err := cs.RunCleanup(); err != nil {
				log.Printf("定时清理失败: %v", err)
			} else {
				log.Printf("定时清理完成: 删除 %d 个文件，释放 %d 字节，耗时 %v", 
					stats.FilesDeleted, stats.BytesFreed, stats.Duration)
			}
		case <-cs.stopChan:
			return
		}
	}
}

// RunCleanup 执行清理
func (cs *CleanupService) RunCleanup() (*CleanupStats, error) {
	start := time.Now()
	stats := &CleanupStats{
		LastCleanupTime: start,
	}

	log.Println("开始执行文件清理")

	// 清理上传目录
	if cs.config.UploadDir != "" {
		if err := cs.cleanupDirectory(cs.config.UploadDir, stats); err != nil {
			stats.Errors = append(stats.Errors, fmt.Sprintf("清理上传目录失败: %v", err))
		}
	}

	// 清理输出目录
	if cs.config.OutputDir != "" {
		if err := cs.cleanupDirectory(cs.config.OutputDir, stats); err != nil {
			stats.Errors = append(stats.Errors, fmt.Sprintf("清理输出目录失败: %v", err))
		}
	}

	// 清理临时目录
	if cs.config.TempDir != "" {
		if err := cs.cleanupDirectory(cs.config.TempDir, stats); err != nil {
			stats.Errors = append(stats.Errors, fmt.Sprintf("清理临时目录失败: %v", err))
		}
	}

	// 清理过期任务
	if cs.taskService != nil {
		if err := cs.cleanupExpiredTasks(stats); err != nil {
			stats.Errors = append(stats.Errors, fmt.Sprintf("清理过期任务失败: %v", err))
		}
	}

	// 检查磁盘使用量
	if err := cs.checkDiskUsage(stats); err != nil {
		stats.Errors = append(stats.Errors, fmt.Sprintf("检查磁盘使用量失败: %v", err))
	}

	stats.Duration = time.Since(start)
	log.Printf("文件清理完成，删除 %d 个文件，释放 %d 字节", stats.FilesDeleted, stats.BytesFreed)

	return stats, nil
}

// cleanupDirectory 清理目录
func (cs *CleanupService) cleanupDirectory(dir string, stats *CleanupStats) error {
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return nil // 目录不存在，跳过
	}

	cutoffTime := time.Now().Add(-cs.config.MaxFileAge)

	return filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// 跳过目录
		if info.IsDir() {
			return nil
		}

		// 检查文件是否过期
		if info.ModTime().Before(cutoffTime) {
			log.Printf("删除过期文件: %s (修改时间: %v)", path, info.ModTime())
			if err := os.Remove(path); err != nil {
				return fmt.Errorf("删除文件 %s 失败: %v", path, err)
			}
			stats.FilesDeleted++
			stats.BytesFreed += info.Size()
		}

		return nil
	})
}

// cleanupExpiredTasks 清理过期任务
func (cs *CleanupService) cleanupExpiredTasks(stats *CleanupStats) error {
	// 这里需要根据实际的TaskService实现来调整
	// 假设TaskService有GetExpiredTasks方法
	log.Println("清理过期任务")
	
	// 由于当前TaskService可能没有GetExpiredTasks方法，
	// 这里先记录日志，实际实现需要根据具体需求调整
	stats.TasksCleaned = 0
	return nil
}

// checkDiskUsage 检查磁盘使用量
func (cs *CleanupService) checkDiskUsage(stats *CleanupStats) error {
	totalSize := int64(0)

	// 计算所有目录的总大小
	dirs := []string{cs.config.UploadDir, cs.config.OutputDir, cs.config.TempDir}
	for _, dir := range dirs {
		if dir == "" {
			continue
		}
		size, err := cs.calculateDirSize(dir)
		if err != nil {
			log.Printf("计算目录 %s 大小失败: %v", dir, err)
			continue
		}
		totalSize += size
	}

	log.Printf("当前磁盘使用量: %d 字节，限制: %d 字节", totalSize, cs.config.MaxDiskUsage)

	// 如果超过限制，执行紧急清理
	if totalSize > cs.config.MaxDiskUsage {
		log.Println("磁盘使用量超过限制，执行紧急清理")
		return cs.emergencyCleanup(stats)
	}

	return nil
}

// calculateDirSize 计算目录大小
func (cs *CleanupService) calculateDirSize(dir string) (int64, error) {
	var size int64
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			size += info.Size()
		}
		return nil
	})
	return size, err
}

// emergencyCleanup 紧急清理
func (cs *CleanupService) emergencyCleanup(stats *CleanupStats) error {
	log.Println("执行紧急清理，删除最旧的文件")

	// 收集所有文件信息
	type fileInfo struct {
		path    string
		size    int64
		modTime time.Time
	}

	var files []fileInfo
	dirs := []string{cs.config.UploadDir, cs.config.OutputDir, cs.config.TempDir}

	for _, dir := range dirs {
		if dir == "" {
			continue
		}
		err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() {
				return err
			}
			files = append(files, fileInfo{
				path:    path,
				size:    info.Size(),
				modTime: info.ModTime(),
			})
			return nil
		})
		if err != nil {
			return err
		}
	}

	// 按修改时间排序（最旧的在前）
	for i := 0; i < len(files)-1; i++ {
		for j := i + 1; j < len(files); j++ {
			if files[i].modTime.After(files[j].modTime) {
				files[i], files[j] = files[j], files[i]
			}
		}
	}

	// 删除最旧的文件，直到磁盘使用量降到限制以下
	targetSize := cs.config.MaxDiskUsage * 80 / 100 // 删除到80%的限制
	freedSize := int64(0)

	for _, file := range files {
		if freedSize >= (cs.config.MaxDiskUsage - targetSize) {
			break
		}

		log.Printf("紧急删除文件: %s", file.path)
		if err := os.Remove(file.path); err != nil {
			log.Printf("删除文件失败: %v", err)
			continue
		}

		stats.FilesDeleted++
		stats.BytesFreed += file.size
		freedSize += file.size
	}

	return nil
}

// GetStats 获取清理统计信息
func (cs *CleanupService) GetStats() *CleanupStats {
	// 这里可以返回最近一次清理的统计信息
	// 实际实现中可能需要持久化这些统计信息
	return &CleanupStats{
		LastCleanupTime: time.Now(),
	}
}

// IsRunning 检查服务是否正在运行
func (cs *CleanupService) IsRunning() bool {
	return cs.running
}

// ForceCleanup 强制执行清理
func (cs *CleanupService) ForceCleanup() (*CleanupStats, error) {
	log.Println("强制执行清理")
	return cs.RunCleanup()
}

// CleanupByPattern 按模式清理文件
func (cs *CleanupService) CleanupByPattern(dir, pattern string) (*CleanupStats, error) {
	stats := &CleanupStats{
		LastCleanupTime: time.Now(),
	}

	matches, err := filepath.Glob(filepath.Join(dir, pattern))
	if err != nil {
		return stats, err
	}

	for _, match := range matches {
		info, err := os.Stat(match)
		if err != nil {
			continue
		}

		if !info.IsDir() {
			log.Printf("删除匹配文件: %s", match)
			if err := os.Remove(match); err != nil {
				stats.Errors = append(stats.Errors, err.Error())
			} else {
				stats.FilesDeleted++
				stats.BytesFreed += info.Size()
			}
		}
	}

	return stats, nil
}