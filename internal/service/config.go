package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// ConfigSource 配置源类型
type ConfigSource string

const (
	ConfigSourceFile    ConfigSource = "file"    // 文件
	ConfigSourceEnv     ConfigSource = "env"     // 环境变量
	ConfigSourceConsul  ConfigSource = "consul"  // Consul
	ConfigSourceEtcd    ConfigSource = "etcd"    // Etcd
	ConfigSourceRemote  ConfigSource = "remote"  // 远程配置
)

// ConfigValue 配置值
type ConfigValue struct {
	Value      interface{}   // 配置值
	Source     ConfigSource  // 配置源
	UpdateTime time.Time     // 更新时间
	Version    int64         // 版本号
}

// Config 配置
type Config struct {
	Key         string                 // 配置键
	Value       *ConfigValue           // 配置值
	Description string                 // 配置描述
	Type        string                 // 配置类型
	Default     interface{}            // 默认值
	Required    bool                   // 是否必需
	Validators  []ConfigValidator      // 验证器
	Watchers    []ConfigWatcher        // 观察者
}

// ConfigValidator 配置验证器
type ConfigValidator func(value interface{}) error

// ConfigWatcher 配置观察者
type ConfigWatcher func(key string, oldValue, newValue interface{})

// ConfigManager 配置管理器
type ConfigManager struct {
	configs    map[string]*Config
	values     map[string]*ConfigValue
	mu         sync.RWMutex
	watchers   map[string][]ConfigWatcher
	stopCh     chan struct{}
}

// NewConfigManager 创建配置管理器实例
func NewConfigManager() *ConfigManager {
	return &ConfigManager{
		configs:  make(map[string]*Config),
		values:   make(map[string]*ConfigValue),
		watchers: make(map[string][]ConfigWatcher),
		stopCh:   make(chan struct{}),
	}
}

// Register 注册配置
func (m *ConfigManager) Register(key string, config *Config) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.configs[key]; exists {
		return fmt.Errorf("config %s already exists", key)
	}

	m.configs[key] = config
	return nil
}

// Get 获取配置值
func (m *ConfigManager) Get(key string) (interface{}, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	config, exists := m.configs[key]
	if !exists {
		return nil, fmt.Errorf("config %s not found", key)
	}

	value, exists := m.values[key]
	if !exists {
		if config.Default != nil {
			return config.Default, nil
		}
		if config.Required {
			return nil, fmt.Errorf("config %s is required", key)
		}
		return nil, nil
	}

	return value.Value, nil
}

// Set 设置配置值
func (m *ConfigManager) Set(key string, value interface{}, source ConfigSource) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	config, exists := m.configs[key]
	if !exists {
		return fmt.Errorf("config %s not found", key)
	}

	// 验证配置值
	for _, validator := range config.Validators {
		if err := validator(value); err != nil {
			return fmt.Errorf("config %s validation failed: %v", key, err)
		}
	}

	// 获取旧值
	oldValue := m.values[key]

	// 设置新值
	m.values[key] = &ConfigValue{
		Value:      value,
		Source:     source,
		UpdateTime: time.Now(),
		Version:    time.Now().UnixNano(),
	}

	// 通知观察者
	if oldValue != nil {
		for _, watcher := range config.Watchers {
			watcher(key, oldValue.Value, value)
		}
	}

	return nil
}

// Watch 观察配置变化
func (m *ConfigManager) Watch(key string, watcher ConfigWatcher) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	config, exists := m.configs[key]
	if !exists {
		return fmt.Errorf("config %s not found", key)
	}

	config.Watchers = append(config.Watchers, watcher)
	return nil
}

// LoadFile 从文件加载配置
func (m *ConfigManager) LoadFile(path string) error {
	data, err := ioutil.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read config file: %v", err)
	}

	var configs map[string]interface{}
	if err := json.Unmarshal(data, &configs); err != nil {
		return fmt.Errorf("failed to parse config file: %v", err)
	}

	for key, value := range configs {
		if err := m.Set(key, value, ConfigSourceFile); err != nil {
			return fmt.Errorf("failed to set config %s: %v", key, err)
		}
	}

	return nil
}

// LoadEnv 从环境变量加载配置
func (m *ConfigManager) LoadEnv(prefix string) error {
	for _, env := range os.Environ() {
		if len(env) < len(prefix) || env[:len(prefix)] != prefix {
			continue
		}

		key := env[len(prefix):]
		value := os.Getenv(env)

		if err := m.Set(key, value, ConfigSourceEnv); err != nil {
			return fmt.Errorf("failed to set config %s: %v", key, err)
		}
	}

	return nil
}

// Save 保存配置到文件
func (m *ConfigManager) Save(path string) error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	configs := make(map[string]interface{})
	for key, value := range m.values {
		configs[key] = value.Value
	}

	data, err := json.MarshalIndent(configs, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal configs: %v", err)
	}

	if err := ioutil.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write config file: %v", err)
	}

	return nil
}

// WatchFile 观察配置文件变化
func (m *ConfigManager) WatchFile(path string) error {
	// 获取文件信息
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("failed to stat config file: %v", err)
	}

	// 启动观察
	go func() {
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()

		var lastModTime time.Time
		for {
			select {
			case <-m.stopCh:
				return
			case <-ticker.C:
				// 检查文件变化
				info, err := os.Stat(path)
				if err != nil {
					continue
				}

				if info.ModTime().After(lastModTime) {
					lastModTime = info.ModTime()
					if err := m.LoadFile(path); err != nil {
						// TODO: 处理错误
					}
				}
			}
		}
	}()

	return nil
}

// Stop 停止配置管理器
func (m *ConfigManager) Stop() {
	close(m.stopCh)
}

// Config 配置服务
type Config struct {
	mu     sync.RWMutex
	config map[string]interface{}
}

// NewConfig 创建配置服务实例
func NewConfig() *Config {
	return &Config{
		config: make(map[string]interface{}),
	}
}

// Load 从文件加载配置
func (c *Config) Load(configFile string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// 读取配置文件
	data, err := os.ReadFile(configFile)
	if err != nil {
		return fmt.Errorf("failed to read config file: %v", err)
	}

	// 解析JSON配置
	if err := json.Unmarshal(data, &c.config); err != nil {
		return fmt.Errorf("failed to parse config file: %v", err)
	}

	return nil
}

// Save 保存配置到文件
func (c *Config) Save(configFile string) error {
	c.mu.RLock()
	defer c.mu.RUnlock()

	// 确保配置目录存在
	dir := filepath.Dir(configFile)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %v", err)
	}

	// 序列化配置为JSON
	data, err := json.MarshalIndent(c.config, "", "    ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %v", err)
	}

	// 写入配置文件
	if err := os.WriteFile(configFile, data, 0644); err != nil {
		return fmt.Errorf("failed to write config file: %v", err)
	}

	return nil
}

// Get 获取配置项
func (c *Config) Get(key string) (interface{}, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	value, exists := c.config[key]
	return value, exists
}

// GetString 获取字符串配置项
func (c *Config) GetString(key string) (string, bool) {
	value, exists := c.Get(key)
	if !exists {
		return "", false
	}
	str, ok := value.(string)
	return str, ok
}

// GetInt 获取整数配置项
func (c *Config) GetInt(key string) (int, bool) {
	value, exists := c.Get(key)
	if !exists {
		return 0, false
	}
	
	// 处理JSON中的数字类型
	switch v := value.(type) {
	case int:
		return v, true
	case float64:
		return int(v), true
	default:
		return 0, false
	}
}

// GetBool 获取布尔配置项
func (c *Config) GetBool(key string) (bool, bool) {
	value, exists := c.Get(key)
	if !exists {
		return false, false
	}
	b, ok := value.(bool)
	return b, ok
}

// Set 设置配置项
func (c *Config) Set(key string, value interface{}) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.config[key] = value
}

// Delete 删除配置项
func (c *Config) Delete(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.config, key)
}

// GetAll 获取所有配置项
func (c *Config) GetAll() map[string]interface{} {
	c.mu.RLock()
	defer c.mu.RUnlock()

	// 创建配置的副本
	config := make(map[string]interface{})
	for k, v := range c.config {
		config[k] = v
	}
	return config
} 