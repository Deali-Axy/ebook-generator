package config

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"time"

	"gopkg.in/yaml.v3"
)

// ConfigManager 配置管理器
type ConfigManager struct {
	config       interface{}
	configPath   string
	watchers     []ConfigWatcher
	mutex        sync.RWMutex
	validators   []ConfigValidator
	changeHooks  []ChangeHook
	hotReload    bool
	fileWatcher  *FileWatcher
	envPrefix    string
	defaultValues map[string]interface{}
}

// ConfigWatcher 配置监听器接口
type ConfigWatcher interface {
	OnConfigChange(oldConfig, newConfig interface{}) error
}

// ConfigValidator 配置验证器接口
type ConfigValidator interface {
	Validate(config interface{}) error
}

// ChangeHook 配置变更钩子
type ChangeHook func(path string, oldValue, newValue interface{}) error

// FileWatcher 文件监听器
type FileWatcher struct {
	filePath    string
	lastModTime time.Time
	stopChan    chan bool
	manager     *ConfigManager
}

// ConfigOptions 配置选项
type ConfigOptions struct {
	ConfigPath    string
	HotReload     bool
	EnvPrefix     string
	DefaultValues map[string]interface{}
	Validators    []ConfigValidator
	Watchers      []ConfigWatcher
}

// ConfigSource 配置源
type ConfigSource interface {
	Load() (map[string]interface{}, error)
	Watch(callback func(map[string]interface{})) error
	Name() string
}

// FileConfigSource 文件配置源
type FileConfigSource struct {
	filePath string
	format   ConfigFormat
}

// EnvConfigSource 环境变量配置源
type EnvConfigSource struct {
	prefix string
}

// RemoteConfigSource 远程配置源
type RemoteConfigSource struct {
	url     string
	headers map[string]string
	timeout time.Duration
}

// ConfigFormat 配置格式
type ConfigFormat string

const (
	ConfigFormatJSON ConfigFormat = "json"
	ConfigFormatYAML ConfigFormat = "yaml"
	ConfigFormatTOML ConfigFormat = "toml"
)

// ConfigChange 配置变更
type ConfigChange struct {
	Path     string      `json:"path"`
	OldValue interface{} `json:"old_value"`
	NewValue interface{} `json:"new_value"`
	Action   ChangeAction `json:"action"`
}

// ChangeAction 变更动作
type ChangeAction string

const (
	ChangeActionAdd    ChangeAction = "add"
	ChangeActionUpdate ChangeAction = "update"
	ChangeActionDelete ChangeAction = "delete"
)

// NewConfigManager 创建配置管理器
func NewConfigManager(config interface{}, options ConfigOptions) (*ConfigManager, error) {
	cm := &ConfigManager{
		config:        config,
		configPath:    options.ConfigPath,
		watchers:      options.Watchers,
		validators:    options.Validators,
		hotReload:     options.HotReload,
		envPrefix:     options.EnvPrefix,
		defaultValues: options.DefaultValues,
		changeHooks:   make([]ChangeHook, 0),
	}

	// 加载配置
	if err := cm.Load(); err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	// 启动热重载
	if cm.hotReload && cm.configPath != "" {
		if err := cm.startFileWatcher(); err != nil {
			return nil, fmt.Errorf("failed to start file watcher: %w", err)
		}
	}

	return cm, nil
}

// Load 加载配置
func (cm *ConfigManager) Load() error {
	cm.mutex.Lock()
	defer cm.mutex.Unlock()

	// 应用默认值
	if err := cm.applyDefaults(); err != nil {
		return fmt.Errorf("failed to apply defaults: %w", err)
	}

	// 从文件加载
	if cm.configPath != "" {
		if err := cm.loadFromFile(); err != nil {
			return fmt.Errorf("failed to load from file: %w", err)
		}
	}

	// 从环境变量加载
	if err := cm.loadFromEnv(); err != nil {
		return fmt.Errorf("failed to load from env: %w", err)
	}

	// 验证配置
	if err := cm.validate(); err != nil {
		return fmt.Errorf("config validation failed: %w", err)
	}

	return nil
}

// Reload 重新加载配置
func (cm *ConfigManager) Reload() error {
	// 保存旧配置
	oldConfig := cm.cloneConfig()

	// 重新加载
	if err := cm.Load(); err != nil {
		return err
	}

	// 通知监听器
	for _, watcher := range cm.watchers {
		if err := watcher.OnConfigChange(oldConfig, cm.config); err != nil {
			return fmt.Errorf("watcher error: %w", err)
		}
	}

	return nil
}

// Get 获取配置值
func (cm *ConfigManager) Get(path string) (interface{}, error) {
	cm.mutex.RLock()
	defer cm.mutex.RUnlock()

	return cm.getValueByPath(cm.config, path)
}

// Set 设置配置值
func (cm *ConfigManager) Set(path string, value interface{}) error {
	cm.mutex.Lock()
	defer cm.mutex.Unlock()

	// 获取旧值
	oldValue, _ := cm.getValueByPath(cm.config, path)

	// 设置新值
	if err := cm.setValueByPath(cm.config, path, value); err != nil {
		return err
	}

	// 验证配置
	if err := cm.validate(); err != nil {
		// 回滚
		cm.setValueByPath(cm.config, path, oldValue)
		return fmt.Errorf("validation failed: %w", err)
	}

	// 执行变更钩子
	for _, hook := range cm.changeHooks {
		if err := hook(path, oldValue, value); err != nil {
			return fmt.Errorf("change hook error: %w", err)
		}
	}

	// 保存到文件
	if cm.configPath != "" {
		if err := cm.saveToFile(); err != nil {
			return fmt.Errorf("failed to save config: %w", err)
		}
	}

	return nil
}

// GetConfig 获取完整配置
func (cm *ConfigManager) GetConfig() interface{} {
	cm.mutex.RLock()
	defer cm.mutex.RUnlock()
	return cm.cloneConfig()
}

// SetConfig 设置完整配置
func (cm *ConfigManager) SetConfig(config interface{}) error {
	cm.mutex.Lock()
	defer cm.mutex.Unlock()

	oldConfig := cm.cloneConfig()
	cm.config = config

	// 验证配置
	if err := cm.validate(); err != nil {
		// 回滚
		cm.config = oldConfig
		return fmt.Errorf("validation failed: %w", err)
	}

	// 通知监听器
	for _, watcher := range cm.watchers {
		if err := watcher.OnConfigChange(oldConfig, cm.config); err != nil {
			return fmt.Errorf("watcher error: %w", err)
		}
	}

	// 保存到文件
	if cm.configPath != "" {
		if err := cm.saveToFile(); err != nil {
			return fmt.Errorf("failed to save config: %w", err)
		}
	}

	return nil
}

// AddWatcher 添加配置监听器
func (cm *ConfigManager) AddWatcher(watcher ConfigWatcher) {
	cm.mutex.Lock()
	defer cm.mutex.Unlock()
	cm.watchers = append(cm.watchers, watcher)
}

// AddValidator 添加配置验证器
func (cm *ConfigManager) AddValidator(validator ConfigValidator) {
	cm.mutex.Lock()
	defer cm.mutex.Unlock()
	cm.validators = append(cm.validators, validator)
}

// AddChangeHook 添加变更钩子
func (cm *ConfigManager) AddChangeHook(hook ChangeHook) {
	cm.mutex.Lock()
	defer cm.mutex.Unlock()
	cm.changeHooks = append(cm.changeHooks, hook)
}

// applyDefaults 应用默认值
func (cm *ConfigManager) applyDefaults() error {
	for path, value := range cm.defaultValues {
		if _, err := cm.getValueByPath(cm.config, path); err != nil {
			// 路径不存在，设置默认值
			if err := cm.setValueByPath(cm.config, path, value); err != nil {
				return err
			}
		}
	}
	return nil
}

// loadFromFile 从文件加载配置
func (cm *ConfigManager) loadFromFile() error {
	if _, err := os.Stat(cm.configPath); os.IsNotExist(err) {
		return nil // 文件不存在，跳过
	}

	data, err := ioutil.ReadFile(cm.configPath)
	if err != nil {
		return err
	}

	// 根据文件扩展名确定格式
	ext := strings.ToLower(filepath.Ext(cm.configPath))
	switch ext {
	case ".json":
		return json.Unmarshal(data, cm.config)
	case ".yaml", ".yml":
		return yaml.Unmarshal(data, cm.config)
	default:
		// 默认尝试JSON
		return json.Unmarshal(data, cm.config)
	}
}

// loadFromEnv 从环境变量加载配置
func (cm *ConfigManager) loadFromEnv() error {
	if cm.envPrefix == "" {
		return nil
	}

	// 获取所有环境变量
	envVars := os.Environ()
	prefix := cm.envPrefix + "_"

	for _, env := range envVars {
		parts := strings.SplitN(env, "=", 2)
		if len(parts) != 2 {
			continue
		}

		key := parts[0]
		value := parts[1]

		if !strings.HasPrefix(key, prefix) {
			continue
		}

		// 转换环境变量名为配置路径
		configPath := strings.ToLower(strings.TrimPrefix(key, prefix))
		configPath = strings.ReplaceAll(configPath, "_", ".")

		// 尝试转换值类型
		convertedValue := cm.convertEnvValue(value)

		// 设置配置值
		if err := cm.setValueByPath(cm.config, configPath, convertedValue); err != nil {
			// 忽略设置失败的环境变量
			continue
		}
	}

	return nil
}

// convertEnvValue 转换环境变量值
func (cm *ConfigManager) convertEnvValue(value string) interface{} {
	// 尝试转换为布尔值
	if b, err := strconv.ParseBool(value); err == nil {
		return b
	}

	// 尝试转换为整数
	if i, err := strconv.ParseInt(value, 10, 64); err == nil {
		return i
	}

	// 尝试转换为浮点数
	if f, err := strconv.ParseFloat(value, 64); err == nil {
		return f
	}

	// 默认返回字符串
	return value
}

// saveToFile 保存配置到文件
func (cm *ConfigManager) saveToFile() error {
	if cm.configPath == "" {
		return nil
	}

	// 确保目录存在
	dir := filepath.Dir(cm.configPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	// 根据文件扩展名确定格式
	ext := strings.ToLower(filepath.Ext(cm.configPath))
	var data []byte
	var err error

	switch ext {
	case ".json":
		data, err = json.MarshalIndent(cm.config, "", "  ")
	case ".yaml", ".yml":
		data, err = yaml.Marshal(cm.config)
	default:
		// 默认使用JSON
		data, err = json.MarshalIndent(cm.config, "", "  ")
	}

	if err != nil {
		return err
	}

	return ioutil.WriteFile(cm.configPath, data, 0644)
}

// validate 验证配置
func (cm *ConfigManager) validate() error {
	for _, validator := range cm.validators {
		if err := validator.Validate(cm.config); err != nil {
			return err
		}
	}
	return nil
}

// getValueByPath 根据路径获取值
func (cm *ConfigManager) getValueByPath(config interface{}, path string) (interface{}, error) {
	parts := strings.Split(path, ".")
	current := config

	for _, part := range parts {
		switch v := current.(type) {
		case map[string]interface{}:
			if val, exists := v[part]; exists {
				current = val
			} else {
				return nil, fmt.Errorf("path not found: %s", path)
			}
		default:
			// 使用反射处理结构体
			val := reflect.ValueOf(current)
			if val.Kind() == reflect.Ptr {
				val = val.Elem()
			}
			if val.Kind() != reflect.Struct {
				return nil, fmt.Errorf("invalid path: %s", path)
			}

			field := val.FieldByName(part)
			if !field.IsValid() {
				return nil, fmt.Errorf("field not found: %s", part)
			}
			current = field.Interface()
		}
	}

	return current, nil
}

// setValueByPath 根据路径设置值
func (cm *ConfigManager) setValueByPath(config interface{}, path string, value interface{}) error {
	parts := strings.Split(path, ".")
	current := config

	for i, part := range parts {
		if i == len(parts)-1 {
			// 最后一个部分，设置值
			switch v := current.(type) {
			case map[string]interface{}:
				v[part] = value
			default:
				// 使用反射处理结构体
				val := reflect.ValueOf(current)
				if val.Kind() == reflect.Ptr {
					val = val.Elem()
				}
				if val.Kind() != reflect.Struct {
					return fmt.Errorf("invalid path: %s", path)
				}

				field := val.FieldByName(part)
				if !field.IsValid() || !field.CanSet() {
					return fmt.Errorf("field not settable: %s", part)
				}
				field.Set(reflect.ValueOf(value))
			}
		} else {
			// 中间部分，继续遍历
			switch v := current.(type) {
			case map[string]interface{}:
				if val, exists := v[part]; exists {
					current = val
				} else {
					// 创建新的map
					newMap := make(map[string]interface{})
					v[part] = newMap
					current = newMap
				}
			default:
				// 使用反射处理结构体
				val := reflect.ValueOf(current)
				if val.Kind() == reflect.Ptr {
					val = val.Elem()
				}
				if val.Kind() != reflect.Struct {
					return fmt.Errorf("invalid path: %s", path)
				}

				field := val.FieldByName(part)
				if !field.IsValid() {
					return fmt.Errorf("field not found: %s", part)
				}
				current = field.Interface()
			}
		}
	}

	return nil
}

// cloneConfig 克隆配置
func (cm *ConfigManager) cloneConfig() interface{} {
	// 使用JSON序列化/反序列化进行深拷贝
	data, _ := json.Marshal(cm.config)
	var clone interface{}
	json.Unmarshal(data, &clone)
	return clone
}

// startFileWatcher 启动文件监听器
func (cm *ConfigManager) startFileWatcher() error {
	info, err := os.Stat(cm.configPath)
	if err != nil {
		return err
	}

	cm.fileWatcher = &FileWatcher{
		filePath:    cm.configPath,
		lastModTime: info.ModTime(),
		stopChan:    make(chan bool),
		manager:     cm,
	}

	go cm.fileWatcher.watch()
	return nil
}

// watch 监听文件变化
func (fw *FileWatcher) watch() {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-fw.stopChan:
			return
		case <-ticker.C:
			info, err := os.Stat(fw.filePath)
			if err != nil {
				continue
			}

			if info.ModTime().After(fw.lastModTime) {
				fw.lastModTime = info.ModTime()
				// 延迟一下，确保文件写入完成
				time.Sleep(100 * time.Millisecond)
				fw.manager.Reload()
			}
		}
	}
}

// Stop 停止配置管理器
func (cm *ConfigManager) Stop() {
	if cm.fileWatcher != nil {
		close(cm.fileWatcher.stopChan)
	}
}

// Export 导出配置
func (cm *ConfigManager) Export(format ConfigFormat) ([]byte, error) {
	cm.mutex.RLock()
	defer cm.mutex.RUnlock()

	switch format {
	case ConfigFormatJSON:
		return json.MarshalIndent(cm.config, "", "  ")
	case ConfigFormatYAML:
		return yaml.Marshal(cm.config)
	default:
		return nil, fmt.Errorf("unsupported format: %s", format)
	}
}

// Import 导入配置
func (cm *ConfigManager) Import(data []byte, format ConfigFormat) error {
	var newConfig interface{}

	switch format {
	case ConfigFormatJSON:
		if err := json.Unmarshal(data, &newConfig); err != nil {
			return err
		}
	case ConfigFormatYAML:
		if err := yaml.Unmarshal(data, &newConfig); err != nil {
			return err
		}
	default:
		return fmt.Errorf("unsupported format: %s", format)
	}

	return cm.SetConfig(newConfig)
}

// GetChanges 获取配置变更
func (cm *ConfigManager) GetChanges(oldConfig, newConfig interface{}) []ConfigChange {
	changes := make([]ConfigChange, 0)
	cm.compareConfigs("", oldConfig, newConfig, &changes)
	return changes
}

// compareConfigs 比较配置
func (cm *ConfigManager) compareConfigs(path string, old, new interface{}, changes *[]ConfigChange) {
	// 这里应该实现深度比较逻辑
	// 为简化示例，只做简单比较
	if !reflect.DeepEqual(old, new) {
		*changes = append(*changes, ConfigChange{
			Path:     path,
			OldValue: old,
			NewValue: new,
			Action:   ChangeActionUpdate,
		})
	}
}