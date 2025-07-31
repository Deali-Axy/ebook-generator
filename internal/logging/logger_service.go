package logging

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

// LoggerService 日志服务
type LoggerService struct {
	config    LogConfig
	writers   []io.Writer
	mutex     sync.RWMutex
	buffer    []LogEntry
	bufferMux sync.Mutex
	rotator   *LogRotator
}

// LogConfig 日志配置
type LogConfig struct {
	Level          LogLevel `json:"level"`
	Format         LogFormat `json:"format"`
	Output         []string `json:"output"` // console, file, syslog
	FilePath       string   `json:"file_path"`
	MaxSize        int64    `json:"max_size"`        // MB
	MaxAge         int      `json:"max_age"`         // days
	MaxBackups     int      `json:"max_backups"`
	Compress       bool     `json:"compress"`
	BufferSize     int      `json:"buffer_size"`
	FlushInterval  time.Duration `json:"flush_interval"`
	IncludeSource  bool     `json:"include_source"`
	IncludeStack   bool     `json:"include_stack"`
	Timezone       string   `json:"timezone"`
}

// LogLevel 日志级别
type LogLevel int

const (
	LogLevelTrace LogLevel = iota
	LogLevelDebug
	LogLevelInfo
	LogLevelWarn
	LogLevelError
	LogLevelFatal
)

// LogFormat 日志格式
type LogFormat string

const (
	LogFormatJSON LogFormat = "json"
	LogFormatText LogFormat = "text"
)

// LogEntry 日志条目
type LogEntry struct {
	Timestamp time.Time              `json:"timestamp"`
	Level     LogLevel               `json:"level"`
	Message   string                 `json:"message"`
	Fields    map[string]interface{} `json:"fields,omitempty"`
	Source    *SourceInfo            `json:"source,omitempty"`
	Stack     string                 `json:"stack,omitempty"`
	TraceID   string                 `json:"trace_id,omitempty"`
	SpanID    string                 `json:"span_id,omitempty"`
	UserID    string                 `json:"user_id,omitempty"`
	RequestID string                 `json:"request_id,omitempty"`
}

// SourceInfo 源码信息
type SourceInfo struct {
	File     string `json:"file"`
	Line     int    `json:"line"`
	Function string `json:"function"`
}

// LogRotator 日志轮转器
type LogRotator struct {
	filePath   string
	maxSize    int64
	maxAge     int
	maxBackups int
	compress   bool
	currentFile *os.File
	mutex      sync.Mutex
}

// LogAnalyzer 日志分析器
type LogAnalyzer struct {
	logger *LoggerService
}

// LogStats 日志统计
type LogStats struct {
	TotalLogs    int64                    `json:"total_logs"`
	LevelCounts  map[LogLevel]int64       `json:"level_counts"`
	HourlyStats  map[string]int64         `json:"hourly_stats"`
	TopErrors    []ErrorStat              `json:"top_errors"`
	TopSources   []SourceStat             `json:"top_sources"`
	ResponseTime map[string]float64       `json:"response_time"`
	UserActivity map[string]int64         `json:"user_activity"`
}

// ErrorStat 错误统计
type ErrorStat struct {
	Message string `json:"message"`
	Count   int64  `json:"count"`
	Source  string `json:"source"`
}

// SourceStat 源码统计
type SourceStat struct {
	File  string `json:"file"`
	Count int64  `json:"count"`
}

// String 返回日志级别字符串
func (l LogLevel) String() string {
	switch l {
	case LogLevelTrace:
		return "TRACE"
	case LogLevelDebug:
		return "DEBUG"
	case LogLevelInfo:
		return "INFO"
	case LogLevelWarn:
		return "WARN"
	case LogLevelError:
		return "ERROR"
	case LogLevelFatal:
		return "FATAL"
	default:
		return "UNKNOWN"
	}
}

// NewLoggerService 创建日志服务
func NewLoggerService(config LogConfig) (*LoggerService, error) {
	// 设置默认值
	if config.Level == 0 {
		config.Level = LogLevelInfo
	}
	if config.Format == "" {
		config.Format = LogFormatJSON
	}
	if len(config.Output) == 0 {
		config.Output = []string{"console"}
	}
	if config.BufferSize == 0 {
		config.BufferSize = 1000
	}
	if config.FlushInterval == 0 {
		config.FlushInterval = 5 * time.Second
	}
	if config.MaxSize == 0 {
		config.MaxSize = 100 // 100MB
	}
	if config.MaxAge == 0 {
		config.MaxAge = 30 // 30 days
	}
	if config.MaxBackups == 0 {
		config.MaxBackups = 10
	}

	logger := &LoggerService{
		config: config,
		buffer: make([]LogEntry, 0, config.BufferSize),
	}

	// 初始化输出
	if err := logger.initializeOutputs(); err != nil {
		return nil, fmt.Errorf("failed to initialize outputs: %w", err)
	}

	// 启动缓冲区刷新
	go logger.flushBuffer()

	return logger, nil
}

// initializeOutputs 初始化输出
func (ls *LoggerService) initializeOutputs() error {
	ls.writers = make([]io.Writer, 0)

	for _, output := range ls.config.Output {
		switch output {
		case "console":
			ls.writers = append(ls.writers, os.Stdout)
		case "file":
			if ls.config.FilePath == "" {
				ls.config.FilePath = "logs/app.log"
			}

			// 确保目录存在
			dir := filepath.Dir(ls.config.FilePath)
			if err := os.MkdirAll(dir, 0755); err != nil {
				return fmt.Errorf("failed to create log directory: %w", err)
			}

			// 创建日志轮转器
			ls.rotator = &LogRotator{
				filePath:   ls.config.FilePath,
				maxSize:    ls.config.MaxSize * 1024 * 1024, // 转换为字节
				maxAge:     ls.config.MaxAge,
				maxBackups: ls.config.MaxBackups,
				compress:   ls.config.Compress,
			}

			if err := ls.rotator.openFile(); err != nil {
				return fmt.Errorf("failed to open log file: %w", err)
			}

			ls.writers = append(ls.writers, ls.rotator)
		}
	}

	return nil
}

// Log 记录日志
func (ls *LoggerService) Log(level LogLevel, message string, fields map[string]interface{}) {
	if level < ls.config.Level {
		return
	}

	entry := LogEntry{
		Timestamp: time.Now(),
		Level:     level,
		Message:   message,
		Fields:    fields,
	}

	// 添加源码信息
	if ls.config.IncludeSource {
		entry.Source = ls.getSourceInfo(2)
	}

	// 添加堆栈信息
	if ls.config.IncludeStack && level >= LogLevelError {
		entry.Stack = ls.getStackTrace()
	}

	// 添加到缓冲区
	ls.bufferMux.Lock()
	ls.buffer = append(ls.buffer, entry)
	ls.bufferMux.Unlock()

	// 如果是致命错误，立即刷新
	if level == LogLevelFatal {
		ls.flush()
		os.Exit(1)
	}
}

// Trace 记录跟踪日志
func (ls *LoggerService) Trace(message string, fields ...map[string]interface{}) {
	var f map[string]interface{}
	if len(fields) > 0 {
		f = fields[0]
	}
	ls.Log(LogLevelTrace, message, f)
}

// Debug 记录调试日志
func (ls *LoggerService) Debug(message string, fields ...map[string]interface{}) {
	var f map[string]interface{}
	if len(fields) > 0 {
		f = fields[0]
	}
	ls.Log(LogLevelDebug, message, f)
}

// Info 记录信息日志
func (ls *LoggerService) Info(message string, fields ...map[string]interface{}) {
	var f map[string]interface{}
	if len(fields) > 0 {
		f = fields[0]
	}
	ls.Log(LogLevelInfo, message, f)
}

// Warn 记录警告日志
func (ls *LoggerService) Warn(message string, fields ...map[string]interface{}) {
	var f map[string]interface{}
	if len(fields) > 0 {
		f = fields[0]
	}
	ls.Log(LogLevelWarn, message, f)
}

// Error 记录错误日志
func (ls *LoggerService) Error(message string, fields ...map[string]interface{}) {
	var f map[string]interface{}
	if len(fields) > 0 {
		f = fields[0]
	}
	ls.Log(LogLevelError, message, f)
}

// Fatal 记录致命错误日志
func (ls *LoggerService) Fatal(message string, fields ...map[string]interface{}) {
	var f map[string]interface{}
	if len(fields) > 0 {
		f = fields[0]
	}
	ls.Log(LogLevelFatal, message, f)
}

// WithFields 添加字段
func (ls *LoggerService) WithFields(fields map[string]interface{}) *FieldLogger {
	return &FieldLogger{
		logger: ls,
		fields: fields,
	}
}

// WithTraceID 添加跟踪ID
func (ls *LoggerService) WithTraceID(traceID string) *FieldLogger {
	return &FieldLogger{
		logger: ls,
		fields: map[string]interface{}{"trace_id": traceID},
	}
}

// WithUserID 添加用户ID
func (ls *LoggerService) WithUserID(userID string) *FieldLogger {
	return &FieldLogger{
		logger: ls,
		fields: map[string]interface{}{"user_id": userID},
	}
}

// FieldLogger 字段日志记录器
type FieldLogger struct {
	logger *LoggerService
	fields map[string]interface{}
}

// Trace 记录跟踪日志
func (fl *FieldLogger) Trace(message string) {
	fl.logger.Log(LogLevelTrace, message, fl.fields)
}

// Debug 记录调试日志
func (fl *FieldLogger) Debug(message string) {
	fl.logger.Log(LogLevelDebug, message, fl.fields)
}

// Info 记录信息日志
func (fl *FieldLogger) Info(message string) {
	fl.logger.Log(LogLevelInfo, message, fl.fields)
}

// Warn 记录警告日志
func (fl *FieldLogger) Warn(message string) {
	fl.logger.Log(LogLevelWarn, message, fl.fields)
}

// Error 记录错误日志
func (fl *FieldLogger) Error(message string) {
	fl.logger.Log(LogLevelError, message, fl.fields)
}

// Fatal 记录致命错误日志
func (fl *FieldLogger) Fatal(message string) {
	fl.logger.Log(LogLevelFatal, message, fl.fields)
}

// getSourceInfo 获取源码信息
func (ls *LoggerService) getSourceInfo(skip int) *SourceInfo {
	pc, file, line, ok := runtime.Caller(skip)
	if !ok {
		return nil
	}

	func_name := runtime.FuncForPC(pc).Name()
	// 简化文件路径
	if idx := strings.LastIndex(file, "/"); idx != -1 {
		file = file[idx+1:]
	}
	// 简化函数名
	if idx := strings.LastIndex(func_name, "."); idx != -1 {
		func_name = func_name[idx+1:]
	}

	return &SourceInfo{
		File:     file,
		Line:     line,
		Function: func_name,
	}
}

// getStackTrace 获取堆栈跟踪
func (ls *LoggerService) getStackTrace() string {
	buf := make([]byte, 4096)
	n := runtime.Stack(buf, false)
	return string(buf[:n])
}

// flushBuffer 刷新缓冲区
func (ls *LoggerService) flushBuffer() {
	ticker := time.NewTicker(ls.config.FlushInterval)
	defer ticker.Stop()

	for range ticker.C {
		ls.flush()
	}
}

// flush 刷新日志
func (ls *LoggerService) flush() {
	ls.bufferMux.Lock()
	if len(ls.buffer) == 0 {
		ls.bufferMux.Unlock()
		return
	}

	// 复制缓冲区
	entries := make([]LogEntry, len(ls.buffer))
	copy(entries, ls.buffer)
	ls.buffer = ls.buffer[:0]
	ls.bufferMux.Unlock()

	// 写入日志
	ls.mutex.RLock()
	defer ls.mutex.RUnlock()

	for _, entry := range entries {
		var output string
		if ls.config.Format == LogFormatJSON {
			data, _ := json.Marshal(entry)
			output = string(data) + "\n"
		} else {
			output = ls.formatTextLog(entry)
		}

		for _, writer := range ls.writers {
			writer.Write([]byte(output))
		}
	}
}

// formatTextLog 格式化文本日志
func (ls *LoggerService) formatTextLog(entry LogEntry) string {
	timestamp := entry.Timestamp.Format("2006-01-02 15:04:05")
	level := entry.Level.String()
	message := entry.Message

	var parts []string
	parts = append(parts, fmt.Sprintf("[%s]", timestamp))
	parts = append(parts, fmt.Sprintf("[%s]", level))

	if entry.Source != nil {
		parts = append(parts, fmt.Sprintf("[%s:%d]", entry.Source.File, entry.Source.Line))
	}

	parts = append(parts, message)

	if len(entry.Fields) > 0 {
		fieldsStr := ""
		for k, v := range entry.Fields {
			if fieldsStr != "" {
				fieldsStr += " "
			}
			fieldsStr += fmt.Sprintf("%s=%v", k, v)
		}
		parts = append(parts, fmt.Sprintf("[%s]", fieldsStr))
	}

	return strings.Join(parts, " ") + "\n"
}

// Close 关闭日志服务
func (ls *LoggerService) Close() error {
	ls.flush()
	if ls.rotator != nil && ls.rotator.currentFile != nil {
		return ls.rotator.currentFile.Close()
	}
	return nil
}

// Write 实现io.Writer接口
func (lr *LogRotator) Write(p []byte) (n int, err error) {
	lr.mutex.Lock()
	defer lr.mutex.Unlock()

	// 检查是否需要轮转
	if err := lr.checkRotation(); err != nil {
		return 0, err
	}

	return lr.currentFile.Write(p)
}

// openFile 打开文件
func (lr *LogRotator) openFile() error {
	file, err := os.OpenFile(lr.filePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return err
	}
	lr.currentFile = file
	return nil
}

// checkRotation 检查是否需要轮转
func (lr *LogRotator) checkRotation() error {
	if lr.currentFile == nil {
		return lr.openFile()
	}

	// 获取文件信息
	info, err := lr.currentFile.Stat()
	if err != nil {
		return err
	}

	// 检查文件大小
	if info.Size() >= lr.maxSize {
		return lr.rotate()
	}

	return nil
}

// rotate 轮转日志文件
func (lr *LogRotator) rotate() error {
	// 关闭当前文件
	if err := lr.currentFile.Close(); err != nil {
		return err
	}

	// 生成备份文件名
	backupPath := fmt.Sprintf("%s.%s", lr.filePath, time.Now().Format("20060102-150405"))

	// 重命名当前文件
	if err := os.Rename(lr.filePath, backupPath); err != nil {
		return err
	}

	// 压缩备份文件（如果启用）
	if lr.compress {
		go lr.compressFile(backupPath)
	}

	// 清理旧文件
	go lr.cleanupOldFiles()

	// 打开新文件
	return lr.openFile()
}

// compressFile 压缩文件
func (lr *LogRotator) compressFile(filePath string) {
	// 这里可以实现文件压缩逻辑
	// 为简化示例，这里只是添加.gz后缀
	compressedPath := filePath + ".gz"
	_ = os.Rename(filePath, compressedPath)
}

// cleanupOldFiles 清理旧文件
func (lr *LogRotator) cleanupOldFiles() {
	dir := filepath.Dir(lr.filePath)
	baseName := filepath.Base(lr.filePath)

	files, err := filepath.Glob(filepath.Join(dir, baseName+".*"))
	if err != nil {
		return
	}

	// 按修改时间排序并删除超出数量的文件
	if len(files) > lr.maxBackups {
		// 这里应该按时间排序，为简化示例直接删除多余的
		for i := lr.maxBackups; i < len(files); i++ {
			os.Remove(files[i])
		}
	}

	// 删除过期文件
	cutoff := time.Now().AddDate(0, 0, -lr.maxAge)
	for _, file := range files {
		info, err := os.Stat(file)
		if err != nil {
			continue
		}
		if info.ModTime().Before(cutoff) {
			os.Remove(file)
		}
	}
}

// NewLogAnalyzer 创建日志分析器
func NewLogAnalyzer(logger *LoggerService) *LogAnalyzer {
	return &LogAnalyzer{
		logger: logger,
	}
}

// AnalyzeLogs 分析日志
func (la *LogAnalyzer) AnalyzeLogs(startTime, endTime time.Time) (*LogStats, error) {
	// 这里应该从日志文件或数据库中读取日志进行分析
	// 为简化示例，返回模拟数据
	stats := &LogStats{
		LevelCounts:  make(map[LogLevel]int64),
		HourlyStats:  make(map[string]int64),
		TopErrors:    make([]ErrorStat, 0),
		TopSources:   make([]SourceStat, 0),
		ResponseTime: make(map[string]float64),
		UserActivity: make(map[string]int64),
	}

	return stats, nil
}

// SearchLogs 搜索日志
func (la *LogAnalyzer) SearchLogs(query string, startTime, endTime time.Time, level LogLevel) ([]LogEntry, error) {
	// 这里应该实现日志搜索逻辑
	// 为简化示例，返回空结果
	return []LogEntry{}, nil
}