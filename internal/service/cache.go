package service

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/go-redis/redis/v8"
)

// CachePolicy 缓存策略
type CachePolicy string

const (
	CachePolicyLRU     CachePolicy = "lru"     // 最近最少使用
	CachePolicyLFU     CachePolicy = "lfu"     // 最不经常使用
	CachePolicyFIFO    CachePolicy = "fifo"    // 先进先出
	CachePolicyRandom  CachePolicy = "random"  // 随机淘汰
)

// CacheItem 缓存项
type CacheItem struct {
	Key        string      // 缓存键
	Value      interface{} // 缓存值
	ExpireAt   time.Time   // 过期时间
	CreateAt   time.Time   // 创建时间
	UpdateAt   time.Time   // 更新时间
	AccessAt   time.Time   // 访问时间
	AccessCount int64      // 访问次数
	Size       int64       // 大小
}

// CacheConfig 缓存配置
type CacheConfig struct {
	Policy         CachePolicy     // 缓存策略
	MaxSize        int64          // 最大大小
	MaxItems       int            // 最大项数
	DefaultTTL     time.Duration  // 默认过期时间
	CleanInterval  time.Duration  // 清理间隔
	EnableMetrics  bool           // 启用指标
	EnableLogging  bool           // 启用日志
	PersistFile    string         // 持久化文件路径
}

// Cache 缓存服务
type Cache struct {
	config     *CacheConfig
	logger     *Logger
	metrics    *MetricsService
	items      map[string]*CacheItem
	mu         sync.RWMutex
	stopCh     chan struct{}
}

// CacheService 缓存服务
type CacheService struct {
	client *redis.Client
}

// NewCacheService 创建缓存服务实例
func NewCacheService(addr, password string, db int) *CacheService {
	client := redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: password,
		DB:       db,
	})

	return &CacheService{
		client: client,
	}
}

// NewCache 创建缓存服务实例
func NewCache(config *CacheConfig, logger *Logger, metrics *MetricsService) *Cache {
	cache := &Cache{
		config:  config,
		logger:  logger,
		metrics: metrics,
		items:   make(map[string]*CacheItem),
		stopCh:  make(chan struct{}),
	}

	// 如果配置了持久化文件，尝试加载缓存数据
	if config.PersistFile != "" {
		if err := cache.LoadFromFile(); err != nil {
			logger.Warn("Failed to load cache from file: %v", err)
		}
	}

	// 启动清理
	go cache.clean()

	return cache
}

// Set 设置缓存
func (s *CacheService) Set(ctx context.Context, key string, value interface{}, expiration time.Duration) error {
	data, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("failed to marshal value: %v", err)
	}

	return s.client.Set(ctx, key, data, expiration).Err()
}

// Get 获取缓存
func (s *CacheService) Get(ctx context.Context, key string, value interface{}) error {
	data, err := s.client.Get(ctx, key).Bytes()
	if err != nil {
		if err == redis.Nil {
			return nil
		}
		return fmt.Errorf("failed to get value: %v", err)
	}

	return json.Unmarshal(data, value)
}

// Delete 删除缓存
func (s *CacheService) Delete(ctx context.Context, key string) error {
	return s.client.Del(ctx, key).Err()
}

// Clear 清除所有缓存
func (s *CacheService) Clear(ctx context.Context) error {
	return s.client.FlushAll(ctx).Err()
}

// Close 关闭连接
func (s *CacheService) Close() error {
	return s.client.Close()
}

// Get 获取缓存
func (c *Cache) Get(key string) (interface{}, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	item, exists := c.items[key]
	if !exists {
		return nil, false
	}

	// 检查过期
	if !item.ExpireAt.IsZero() && time.Now().After(item.ExpireAt) {
		delete(c.items, key)
		return nil, false
	}

	// 更新访问信息
	item.AccessAt = time.Now()
	item.AccessCount++

	// 更新指标
	if c.config.EnableMetrics {
		c.metrics.Increment("cache_hits", 1)
	}

	return item.Value, true
}

// Set 设置缓存
func (c *Cache) Set(key string, value interface{}, ttl time.Duration) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// 检查容量
	if len(c.items) >= c.config.MaxItems {
		c.evict()
	}

	// 创建缓存项
	now := time.Now()
	item := &CacheItem{
		Key:         key,
		Value:       value,
		CreateAt:    now,
		UpdateAt:    now,
		AccessAt:    now,
		AccessCount: 0,
		Size:        c.getItemSize(value),
	}

	// 设置过期时间
	if ttl > 0 {
		item.ExpireAt = now.Add(ttl)
	} else if c.config.DefaultTTL > 0 {
		item.ExpireAt = now.Add(c.config.DefaultTTL)
	}

	// 保存缓存项
	c.items[key] = item

	// 更新指标
	if c.config.EnableMetrics {
		c.metrics.Increment("cache_sets", 1)
	}

	return nil
}

// Delete 删除缓存
func (c *Cache) Delete(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	delete(c.items, key)

	// 更新指标
	if c.config.EnableMetrics {
		c.metrics.Increment("cache_deletes", 1)
	}
}

// Clear 清空缓存
func (c *Cache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.items = make(map[string]*CacheItem)

	// 更新指标
	if c.config.EnableMetrics {
		c.metrics.Increment("cache_clears", 1)
	}
}

// GetStats 获取统计信息
func (c *Cache) GetStats() map[string]interface{} {
	c.mu.RLock()
	defer c.mu.RUnlock()

	stats := make(map[string]interface{})
	stats["items"] = len(c.items)
	stats["policy"] = c.config.Policy
	stats["max_items"] = c.config.MaxItems
	stats["max_size"] = c.config.MaxSize

	var totalSize int64
	var expiredCount int
	now := time.Now()

	for _, item := range c.items {
		totalSize += item.Size
		if !item.ExpireAt.IsZero() && now.After(item.ExpireAt) {
			expiredCount++
		}
	}

	stats["total_size"] = totalSize
	stats["expired_count"] = expiredCount

	return stats
}

// clean 清理过期缓存
func (c *Cache) clean() {
	ticker := time.NewTicker(c.config.CleanInterval)
	defer ticker.Stop()

	for {
		select {
		case <-c.stopCh:
			return
		case <-ticker.C:
			c.cleanExpired()
		}
	}
}

// cleanExpired 清理过期缓存
func (c *Cache) cleanExpired() {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()
	for key, item := range c.items {
		if !item.ExpireAt.IsZero() && now.After(item.ExpireAt) {
			delete(c.items, key)
		}
	}
}

// evict 淘汰缓存
func (c *Cache) evict() {
	switch c.config.Policy {
	case CachePolicyLRU:
		c.evictLRU()
	case CachePolicyLFU:
		c.evictLFU()
	case CachePolicyFIFO:
		c.evictFIFO()
	case CachePolicyRandom:
		c.evictRandom()
	}
}

// evictLRU 最近最少使用淘汰
func (c *Cache) evictLRU() {
	var oldestKey string
	var oldestTime time.Time

	for key, item := range c.items {
		if oldestKey == "" || item.AccessAt.Before(oldestTime) {
			oldestKey = key
			oldestTime = item.AccessAt
		}
	}

	if oldestKey != "" {
		delete(c.items, oldestKey)
	}
}

// evictLFU 最不经常使用淘汰
func (c *Cache) evictLFU() {
	var leastKey string
	var leastCount int64

	for key, item := range c.items {
		if leastKey == "" || item.AccessCount < leastCount {
			leastKey = key
			leastCount = item.AccessCount
		}
	}

	if leastKey != "" {
		delete(c.items, leastKey)
	}
}

// evictFIFO 先进先出淘汰
func (c *Cache) evictFIFO() {
	var oldestKey string
	var oldestTime time.Time

	for key, item := range c.items {
		if oldestKey == "" || item.CreateAt.Before(oldestTime) {
			oldestKey = key
			oldestTime = item.CreateAt
		}
	}

	if oldestKey != "" {
		delete(c.items, oldestKey)
	}
}

// evictRandom 随机淘汰
func (c *Cache) evictRandom() {
	for key := range c.items {
		delete(c.items, key)
		break
	}
}

// getItemSize 获取缓存项大小
func (c *Cache) getItemSize(value interface{}) int64 {
	// TODO: 实现大小计算
	return 0
}

// Stop 停止缓存服务
func (c *Cache) Stop() {
	close(c.stopCh)
}

// SaveToFile 保存缓存到文件
func (c *Cache) SaveToFile() error {
	if c.config.PersistFile == "" {
		return nil
	}

	c.mu.RLock()
	data := make(map[string]*CacheItem)
	for k, v := range c.items {
		data[k] = v
	}
	c.mu.RUnlock()

	file, err := os.Create(c.config.PersistFile)
	if err != nil {
		return fmt.Errorf("failed to create cache file: %v", err)
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	if err := encoder.Encode(data); err != nil {
		return fmt.Errorf("failed to encode cache data: %v", err)
	}

	return nil
}

// LoadFromFile 从文件加载缓存
func (c *Cache) LoadFromFile() error {
	if c.config.PersistFile == "" {
		return nil
	}

	file, err := os.Open(c.config.PersistFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("failed to open cache file: %v", err)
	}
	defer file.Close()

	var data map[string]*CacheItem
	decoder := json.NewDecoder(file)
	if err := decoder.Decode(&data); err != nil {
		return fmt.Errorf("failed to decode cache data: %v", err)
	}

	c.mu.Lock()
	c.items = data
	c.mu.Unlock()

	return nil
}

// GetWithTTL 获取缓存项及其剩余TTL
func (c *Cache) GetWithTTL(key string) (interface{}, time.Duration, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	item, exists := c.items[key]
	if !exists {
		return nil, 0, false
	}

	// 检查是否过期
	if item.ExpireAt.IsZero() {
		return item.Value, 0, true
	}

	ttl := item.ExpireAt.Sub(time.Now())
	if ttl <= 0 {
		delete(c.items, key)
		return nil, 0, false
	}

	return item.Value, ttl, true
}

// SetIfNotExists 如果键不存在则设置
func (c *Cache) SetIfNotExists(key string, value interface{}, ttl time.Duration) bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	if _, exists := c.items[key]; exists {
		return false
	}

	now := time.Now()
	item := &CacheItem{
		Key:         key,
		Value:       value,
		CreateAt:    now,
		UpdateAt:    now,
		AccessAt:    now,
		AccessCount: 0,
		Size:        c.getItemSize(value),
	}

	if ttl > 0 {
		item.ExpireAt = now.Add(ttl)
	} else if c.config.DefaultTTL > 0 {
		item.ExpireAt = now.Add(c.config.DefaultTTL)
	}

	c.items[key] = item
	return true
}

// Increment 增加数值
func (c *Cache) Increment(key string, delta int) (int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	item, exists := c.items[key]
	if !exists {
		return 0, fmt.Errorf("key not found: %s", key)
	}

	value, ok := item.Value.(int)
	if !ok {
		return 0, fmt.Errorf("value is not an integer: %s", key)
	}

	value += delta
	item.Value = value
	item.UpdateAt = time.Now()

	return value, nil
}

// Decrement 减少数值
func (c *Cache) Decrement(key string, delta int) (int, error) {
	return c.Increment(key, -delta)
}

// Close 关闭缓存服务
func (c *Cache) Close() error {
	close(c.stopCh)
	if c.config.PersistFile != "" {
		return c.SaveToFile()
	}
	return nil
} 