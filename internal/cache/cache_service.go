package cache

import (
	"crypto/md5"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// CacheEntry 缓存条目
type CacheEntry struct {
	Key        string    `json:"key"`
	FilePath   string    `json:"file_path"`
	Metadata   Metadata  `json:"metadata"`
	CreatedAt  time.Time `json:"created_at"`
	AccessedAt time.Time `json:"accessed_at"`
	AccessCount int64    `json:"access_count"`
	Size       int64     `json:"size"`
}

// Metadata 缓存元数据
type Metadata struct {
	OriginalFile   string            `json:"original_file"`
	FileHash       string            `json:"file_hash"`
	OutputFormat   string            `json:"output_format"`
	ConvertOptions map[string]interface{} `json:"convert_options"`
	TaskID         string            `json:"task_id"`
	UserID         string            `json:"user_id,omitempty"`
}

// CacheConfig 缓存配置
type CacheConfig struct {
	CacheDir     string        `json:"cache_dir"`
	MaxSize      int64         `json:"max_size"`      // 最大缓存大小（字节）
	MaxEntries   int           `json:"max_entries"`   // 最大缓存条目数
	TTL          time.Duration `json:"ttl"`           // 缓存生存时间
	CleanupInterval time.Duration `json:"cleanup_interval"` // 清理间隔
	Enabled      bool          `json:"enabled"`
}

// CacheService 缓存服务
type CacheService struct {
	config    CacheConfig
	entries   map[string]*CacheEntry
	mutex     sync.RWMutex
	stopChan  chan struct{}
	running   bool
	currentSize int64
}

// CacheStats 缓存统计
type CacheStats struct {
	TotalEntries   int   `json:"total_entries"`
	TotalSize      int64 `json:"total_size"`
	HitCount       int64 `json:"hit_count"`
	MissCount      int64 `json:"miss_count"`
	HitRate        float64 `json:"hit_rate"`
	OldestEntry    time.Time `json:"oldest_entry"`
	NewestEntry    time.Time `json:"newest_entry"`
}

// NewCacheService 创建缓存服务
func NewCacheService(config CacheConfig) (*CacheService, error) {
	// 设置默认值
	if config.MaxSize == 0 {
		config.MaxSize = 1024 * 1024 * 1024 // 默认1GB
	}
	if config.MaxEntries == 0 {
		config.MaxEntries = 1000 // 默认1000个条目
	}
	if config.TTL == 0 {
		config.TTL = 24 * time.Hour // 默认24小时
	}
	if config.CleanupInterval == 0 {
		config.CleanupInterval = 1 * time.Hour // 默认1小时清理一次
	}

	// 创建缓存目录
	if err := os.MkdirAll(config.CacheDir, 0755); err != nil {
		return nil, fmt.Errorf("创建缓存目录失败: %v", err)
	}

	cs := &CacheService{
		config:   config,
		entries:  make(map[string]*CacheEntry),
		stopChan: make(chan struct{}),
	}

	// 加载现有缓存
	if err := cs.loadCache(); err != nil {
		return nil, fmt.Errorf("加载缓存失败: %v", err)
	}

	return cs, nil
}

// Start 启动缓存服务
func (cs *CacheService) Start() error {
	if !cs.config.Enabled {
		return nil
	}

	if cs.running {
		return fmt.Errorf("缓存服务已在运行")
	}

	cs.running = true

	// 启动清理goroutine
	go cs.cleanupRoutine()

	return nil
}

// Stop 停止缓存服务
func (cs *CacheService) Stop() error {
	if !cs.running {
		return nil
	}

	close(cs.stopChan)
	cs.running = false

	// 保存缓存索引
	return cs.saveCache()
}

// GenerateCacheKey 生成缓存键
func (cs *CacheService) GenerateCacheKey(fileHash, outputFormat string, options map[string]interface{}) string {
	// 将选项序列化为JSON
	optionsJSON, _ := json.Marshal(options)
	
	// 生成组合哈希
	combined := fmt.Sprintf("%s:%s:%s", fileHash, outputFormat, string(optionsJSON))
	hash := md5.Sum([]byte(combined))
	return fmt.Sprintf("%x", hash)
}

// Put 存储缓存
func (cs *CacheService) Put(key string, filePath string, metadata Metadata) error {
	if !cs.config.Enabled {
		return nil
	}

	cs.mutex.Lock()
	defer cs.mutex.Unlock()

	// 检查文件是否存在
	fileInfo, err := os.Stat(filePath)
	if err != nil {
		return fmt.Errorf("缓存文件不存在: %v", err)
	}

	// 复制文件到缓存目录
	cacheFilePath := filepath.Join(cs.config.CacheDir, key)
	if err := cs.copyFile(filePath, cacheFilePath); err != nil {
		return fmt.Errorf("复制文件到缓存失败: %v", err)
	}

	// 创建缓存条目
	entry := &CacheEntry{
		Key:         key,
		FilePath:    cacheFilePath,
		Metadata:    metadata,
		CreatedAt:   time.Now(),
		AccessedAt:  time.Now(),
		AccessCount: 0,
		Size:        fileInfo.Size(),
	}

	// 检查是否需要清理空间
	if cs.currentSize+entry.Size > cs.config.MaxSize || len(cs.entries) >= cs.config.MaxEntries {
		if err := cs.evictEntries(entry.Size); err != nil {
			os.Remove(cacheFilePath) // 清理刚复制的文件
			return fmt.Errorf("清理缓存空间失败: %v", err)
		}
	}

	// 如果键已存在，先删除旧条目
	if oldEntry, exists := cs.entries[key]; exists {
		os.Remove(oldEntry.FilePath)
		cs.currentSize -= oldEntry.Size
	}

	// 添加新条目
	cs.entries[key] = entry
	cs.currentSize += entry.Size

	return nil
}

// Get 获取缓存
func (cs *CacheService) Get(key string) (string, *Metadata, error) {
	if !cs.config.Enabled {
		return "", nil, fmt.Errorf("缓存未启用")
	}

	cs.mutex.Lock()
	defer cs.mutex.Unlock()

	entry, exists := cs.entries[key]
	if !exists {
		return "", nil, fmt.Errorf("缓存未命中")
	}

	// 检查TTL
	if time.Since(entry.CreatedAt) > cs.config.TTL {
		// 缓存过期，删除
		os.Remove(entry.FilePath)
		cs.currentSize -= entry.Size
		delete(cs.entries, key)
		return "", nil, fmt.Errorf("缓存已过期")
	}

	// 检查文件是否仍然存在
	if _, err := os.Stat(entry.FilePath); os.IsNotExist(err) {
		// 文件不存在，删除条目
		cs.currentSize -= entry.Size
		delete(cs.entries, key)
		return "", nil, fmt.Errorf("缓存文件不存在")
	}

	// 更新访问信息
	entry.AccessedAt = time.Now()
	entry.AccessCount++

	return entry.FilePath, &entry.Metadata, nil
}

// Delete 删除缓存
func (cs *CacheService) Delete(key string) error {
	cs.mutex.Lock()
	defer cs.mutex.Unlock()

	entry, exists := cs.entries[key]
	if !exists {
		return nil // 不存在就当作删除成功
	}

	// 删除文件
	os.Remove(entry.FilePath)
	cs.currentSize -= entry.Size
	delete(cs.entries, key)

	return nil
}

// Clear 清空缓存
func (cs *CacheService) Clear() error {
	cs.mutex.Lock()
	defer cs.mutex.Unlock()

	// 删除所有缓存文件
	for _, entry := range cs.entries {
		os.Remove(entry.FilePath)
	}

	// 清空内存中的条目
	cs.entries = make(map[string]*CacheEntry)
	cs.currentSize = 0

	return nil
}

// GetStats 获取缓存统计
func (cs *CacheService) GetStats() *CacheStats {
	cs.mutex.RLock()
	defer cs.mutex.RUnlock()

	stats := &CacheStats{
		TotalEntries: len(cs.entries),
		TotalSize:    cs.currentSize,
	}

	// 计算命中率和时间范围
	var totalHits, totalMisses int64
	var oldest, newest time.Time
	first := true

	for _, entry := range cs.entries {
		totalHits += entry.AccessCount
		
		if first {
			oldest = entry.CreatedAt
			newest = entry.CreatedAt
			first = false
		} else {
			if entry.CreatedAt.Before(oldest) {
				oldest = entry.CreatedAt
			}
			if entry.CreatedAt.After(newest) {
				newest = entry.CreatedAt
			}
		}
	}

	stats.HitCount = totalHits
	stats.MissCount = totalMisses
	if totalHits+totalMisses > 0 {
		stats.HitRate = float64(totalHits) / float64(totalHits+totalMisses)
	}
	stats.OldestEntry = oldest
	stats.NewestEntry = newest

	return stats
}

// evictEntries 驱逐缓存条目
func (cs *CacheService) evictEntries(neededSize int64) error {
	// 使用LRU策略驱逐条目
	type entryWithKey struct {
		key   string
		entry *CacheEntry
	}

	// 收集所有条目并按访问时间排序
	var entries []entryWithKey
	for key, entry := range cs.entries {
		entries = append(entries, entryWithKey{key: key, entry: entry})
	}

	// 按访问时间排序（最久未访问的在前）
	for i := 0; i < len(entries)-1; i++ {
		for j := i + 1; j < len(entries); j++ {
			if entries[i].entry.AccessedAt.After(entries[j].entry.AccessedAt) {
				entries[i], entries[j] = entries[j], entries[i]
			}
		}
	}

	// 删除最久未访问的条目，直到有足够空间
	freedSize := int64(0)
	for _, item := range entries {
		if cs.currentSize-freedSize+neededSize <= cs.config.MaxSize && 
		   len(cs.entries)-1 < cs.config.MaxEntries {
			break
		}

		// 删除文件和条目
		os.Remove(item.entry.FilePath)
		freedSize += item.entry.Size
		delete(cs.entries, item.key)
	}

	cs.currentSize -= freedSize
	return nil
}

// cleanupRoutine 清理例程
func (cs *CacheService) cleanupRoutine() {
	ticker := time.NewTicker(cs.config.CleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			cs.cleanup()
		case <-cs.stopChan:
			return
		}
	}
}

// cleanup 清理过期缓存
func (cs *CacheService) cleanup() {
	cs.mutex.Lock()
	defer cs.mutex.Unlock()

	now := time.Now()
	var toDelete []string

	// 找出过期的条目
	for key, entry := range cs.entries {
		if now.Sub(entry.CreatedAt) > cs.config.TTL {
			toDelete = append(toDelete, key)
		}
	}

	// 删除过期条目
	for _, key := range toDelete {
		entry := cs.entries[key]
		os.Remove(entry.FilePath)
		cs.currentSize -= entry.Size
		delete(cs.entries, key)
	}
}

// copyFile 复制文件
func (cs *CacheService) copyFile(src, dst string) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	destFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destFile.Close()

	_, err = io.Copy(destFile, sourceFile)
	return err
}

// loadCache 加载缓存索引
func (cs *CacheService) loadCache() error {
	indexFile := filepath.Join(cs.config.CacheDir, "cache_index.json")
	if _, err := os.Stat(indexFile); os.IsNotExist(err) {
		return nil // 索引文件不存在，跳过
	}

	data, err := os.ReadFile(indexFile)
	if err != nil {
		return err
	}

	var entries map[string]*CacheEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		return err
	}

	// 验证缓存文件是否仍然存在
	for key, entry := range entries {
		if _, err := os.Stat(entry.FilePath); os.IsNotExist(err) {
			continue // 文件不存在，跳过
		}
		cs.entries[key] = entry
		cs.currentSize += entry.Size
	}

	return nil
}

// saveCache 保存缓存索引
func (cs *CacheService) saveCache() error {
	indexFile := filepath.Join(cs.config.CacheDir, "cache_index.json")
	data, err := json.MarshalIndent(cs.entries, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(indexFile, data, 0644)
}

// CalculateFileHash 计算文件哈希
func CalculateFileHash(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	hash := md5.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}

	return fmt.Sprintf("%x", hash.Sum(nil)), nil
}