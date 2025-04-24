package service

import (
	"context"
	"fmt"
	"math"
	"sync"
	"time"
)

// RateLimitStrategy 限流策略
type RateLimitStrategy string

const (
	RateLimitStrategyTokenBucket RateLimitStrategy = "token_bucket" // 令牌桶
	RateLimitStrategyLeakyBucket  RateLimitStrategy = "leaky_bucket"  // 漏桶
	RateLimitStrategyCounter      RateLimitStrategy = "counter"      // 计数器
)

// RateLimitConfig 限流配置
type RateLimitConfig struct {
	Strategy          RateLimitStrategy // 限流策略
	Rate              float64          // 速率（每秒）
	Burst             int              // 突发容量
	WindowSize        time.Duration    // 时间窗口大小
	CleanupInterval   time.Duration    // 清理间隔
	EnableDistributed bool             // 启用分布式限流
	DistributedKey    string           // 分布式限流键
	DistributedTimeout time.Duration   // 分布式超时时间
}

// RateLimitStats 限流统计
type RateLimitStats struct {
	TotalRequests    int64         // 总请求数
	AllowedRequests  int64         // 允许的请求数
	RejectedRequests int64         // 拒绝的请求数
	LastRequestTime  time.Time     // 最后请求时间
	CurrentRate      float64       // 当前速率
	Tokens           float64       // 当前令牌数（令牌桶）
	LastRefillTime   time.Time     // 最后填充时间（令牌桶）
	BucketSize       float64       // 桶大小（漏桶）
	LastLeakTime     time.Time     // 最后泄漏时间（漏桶）
	Counter          int64         // 计数器（计数器策略）
	WindowStart      time.Time     // 窗口开始时间（计数器策略）
}

// RateLimiter 限流器
type RateLimiter struct {
	config     *RateLimitConfig
	logger     *Logger
	db         *Database
	stats      map[string]*RateLimitStats
	mu         sync.RWMutex
	stopCh     chan struct{}
}

// TokenBucket 令牌桶
type TokenBucket struct {
	rate       float64
	capacity   int
	tokens     float64
	lastUpdate time.Time
	mu         sync.Mutex
}

// LeakyBucket 漏桶
type LeakyBucket struct {
	rate       float64
	capacity   int
	water      int
	lastUpdate time.Time
	mu         sync.Mutex
}

// Counter 计数器
type Counter struct {
	rate       float64
	window     time.Duration
	count      int
	windowStart time.Time
	mu         sync.Mutex
}

// NewRateLimiter 创建限流器实例
func NewRateLimiter(config *RateLimitConfig, logger *Logger, db *Database) *RateLimiter {
	limiter := &RateLimiter{
		config: config,
		logger: logger,
		db:     db,
		stats:  make(map[string]*RateLimitStats),
		stopCh: make(chan struct{}),
	}

	// 启动清理
	go limiter.cleanup()

	return limiter
}

// Allow 检查是否允许请求
func (l *RateLimiter) Allow(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	// 获取或创建统计信息
	stats, exists := l.stats[key]
	if !exists {
		stats = &RateLimitStats{
			LastRequestTime: time.Now(),
			LastRefillTime: time.Now(),
			LastLeakTime:   time.Now(),
			WindowStart:    time.Now(),
		}
		l.stats[key] = stats
	}

	// 更新统计信息
	stats.TotalRequests++
	now := time.Now()

	// 根据策略检查是否允许请求
	allowed := false
	switch l.config.Strategy {
	case RateLimitStrategyTokenBucket:
		allowed = l.checkTokenBucket(stats, now)
	case RateLimitStrategyLeakyBucket:
		allowed = l.checkLeakyBucket(stats, now)
	case RateLimitStrategyCounter:
		allowed = l.checkCounter(stats, now)
	}

	// 更新统计信息
	if allowed {
		stats.AllowedRequests++
	} else {
		stats.RejectedRequests++
	}
	stats.LastRequestTime = now

	return allowed
}

// checkTokenBucket 检查令牌桶
func (l *RateLimiter) checkTokenBucket(stats *RateLimitStats, now time.Time) bool {
	// 计算新增令牌
	elapsed := now.Sub(stats.LastRefillTime).Seconds()
	newTokens := elapsed * l.config.Rate
	stats.Tokens = math.Min(stats.Tokens+newTokens, float64(l.config.Burst))
	stats.LastRefillTime = now

	// 检查是否有足够的令牌
	if stats.Tokens >= 1 {
		stats.Tokens--
		return true
	}
	return false
}

// checkLeakyBucket 检查漏桶
func (l *RateLimiter) checkLeakyBucket(stats *RateLimitStats, now time.Time) bool {
	// 计算泄漏量
	elapsed := now.Sub(stats.LastLeakTime).Seconds()
	leaked := elapsed * l.config.Rate
	stats.BucketSize = math.Max(0, stats.BucketSize-leaked)
	stats.LastLeakTime = now

	// 检查桶是否有空间
	if stats.BucketSize < float64(l.config.Burst) {
		stats.BucketSize++
		return true
	}
	return false
}

// checkCounter 检查计数器
func (l *RateLimiter) checkCounter(stats *RateLimitStats, now time.Time) bool {
	// 检查是否需要重置窗口
	if now.Sub(stats.WindowStart) >= l.config.WindowSize {
		stats.Counter = 0
		stats.WindowStart = now
	}

	// 检查是否超过限制
	if stats.Counter < l.config.Burst {
		stats.Counter++
		return true
	}
	return false
}

// cleanup 清理过期的统计信息
func (l *RateLimiter) cleanup() {
	ticker := time.NewTicker(l.config.CleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-l.stopCh:
			return
		case <-ticker.C:
			l.mu.Lock()
			now := time.Now()
			for key, stats := range l.stats {
				// 清理超过窗口大小的统计信息
				if now.Sub(stats.LastRequestTime) > l.config.WindowSize {
					delete(l.stats, key)
				}
			}
			l.mu.Unlock()
		}
	}
}

// GetStats 获取限流统计信息
func (l *RateLimiter) GetStats(key string) *RateLimitStats {
	l.mu.RLock()
	defer l.mu.RUnlock()

	if stats, exists := l.stats[key]; exists {
		return stats
	}
	return nil
}

// Reset 重置限流统计
func (l *RateLimiter) Reset(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()

	if stats, exists := l.stats[key]; exists {
		stats.TotalRequests = 0
		stats.AllowedRequests = 0
		stats.RejectedRequests = 0
		stats.LastRequestTime = time.Now()
		stats.CurrentRate = 0
		stats.Tokens = float64(l.config.Burst)
		stats.LastRefillTime = time.Now()
		stats.BucketSize = 0
		stats.LastLeakTime = time.Now()
		stats.Counter = 0
		stats.WindowStart = time.Now()
	}
}

// Stop 停止限流器
func (l *RateLimiter) Stop() {
	close(l.stopCh)
}

// min 返回两个float64中的较小值
func min(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

// max 返回两个int中的较大值
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
} 