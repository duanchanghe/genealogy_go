package service

import (
	"context"
	"fmt"
	"math"
	"sync"
	"time"
)

// RetryStrategy 重试策略
type RetryStrategy string

const (
	RetryStrategyFixed     RetryStrategy = "fixed"      // 固定间隔
	RetryStrategyExponential RetryStrategy = "exponential" // 指数退避
	RetryStrategyLinear    RetryStrategy = "linear"     // 线性退避
	RetryStrategyRandom    RetryStrategy = "random"     // 随机退避
)

// RetryConfig 重试配置
type RetryConfig struct {
	MaxAttempts        int           // 最大重试次数
	InitialInterval    time.Duration // 初始间隔
	MaxInterval        time.Duration // 最大间隔
	Multiplier         float64       // 间隔乘数
	RandomizationFactor float64       // 随机因子
	Strategy           RetryStrategy // 重试策略
	EnableJitter       bool          // 启用抖动
	Timeout            time.Duration // 超时时间
}

// RetryStats 重试统计
type RetryStats struct {
	Attempts      int           // 尝试次数
	LastError     error         // 最后错误
	LastAttempt   time.Time     // 最后尝试时间
	TotalDuration time.Duration // 总持续时间
	Success       bool          // 是否成功
}

// Retry 重试器
type Retry struct {
	config     *RetryConfig
	logger     *Logger
	db         *Database
	stats      map[string]*RetryStats
	mu         sync.RWMutex
}

// NewRetry 创建重试器实例
func NewRetry(config *RetryConfig, logger *Logger, db *Database) *Retry {
	return &Retry{
		config: config,
		logger: logger,
		db:     db,
		stats:  make(map[string]*RetryStats),
	}
}

// Execute 执行重试
func (r *Retry) Execute(key string, fn func() error) error {
	// 创建上下文
	ctx, cancel := context.WithTimeout(context.Background(), r.config.Timeout)
	defer cancel()

	// 获取或创建统计信息
	r.mu.Lock()
	stats, exists := r.stats[key]
	if !exists {
		stats = &RetryStats{}
		r.stats[key] = stats
	}
	r.mu.Unlock()

	// 重置统计信息
	stats.Attempts = 0
	stats.TotalDuration = 0
	stats.Success = false
	startTime := time.Now()

	// 执行重试
	var lastErr error
	for stats.Attempts < r.config.MaxAttempts {
		// 检查上下文是否取消
		if ctx.Err() != nil {
			return fmt.Errorf("retry timeout: %v", ctx.Err())
		}

		// 执行函数
		err := fn()
		if err == nil {
			stats.Success = true
			stats.LastError = nil
			break
		}

		// 更新统计信息
		stats.Attempts++
		stats.LastError = err
		stats.LastAttempt = time.Now()
		stats.TotalDuration = time.Since(startTime)

		// 计算下次重试间隔
		interval := r.calculateInterval(stats.Attempts)
		if interval > 0 {
			// 添加随机抖动
			if r.config.EnableJitter {
				interval = r.addJitter(interval)
			}

			// 等待重试
			select {
			case <-ctx.Done():
				return fmt.Errorf("retry timeout: %v", ctx.Err())
			case <-time.After(interval):
				r.logger.Debug("Retry attempt %d for key %s after %v", stats.Attempts, key, interval)
			}
		}
	}

	// 记录最终结果
	if !stats.Success {
		r.logger.Error("All retry attempts failed for key %s: %v", key, stats.LastError)
	}

	return stats.LastError
}

// calculateInterval 计算重试间隔
func (r *Retry) calculateInterval(attempt int) time.Duration {
	switch r.config.Strategy {
	case RetryStrategyFixed:
		return r.config.InitialInterval
	case RetryStrategyExponential:
		interval := float64(r.config.InitialInterval) * math.Pow(r.config.Multiplier, float64(attempt-1))
		return time.Duration(math.Min(interval, float64(r.config.MaxInterval)))
	case RetryStrategyLinear:
		interval := float64(r.config.InitialInterval) * float64(attempt)
		return time.Duration(math.Min(interval, float64(r.config.MaxInterval)))
	case RetryStrategyRandom:
		interval := float64(r.config.InitialInterval) * (1 + r.config.RandomizationFactor*(2*rand.Float64()-1))
		return time.Duration(math.Min(interval, float64(r.config.MaxInterval)))
	default:
		return r.config.InitialInterval
	}
}

// addJitter 添加随机抖动
func (r *Retry) addJitter(interval time.Duration) time.Duration {
	jitter := float64(interval) * r.config.RandomizationFactor * (2*rand.Float64() - 1)
	return interval + time.Duration(jitter)
}

// GetStats 获取重试统计信息
func (r *Retry) GetStats(key string) *RetryStats {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if stats, exists := r.stats[key]; exists {
		return stats
	}
	return nil
}

// Reset 重置重试统计
func (r *Retry) Reset(key string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if stats, exists := r.stats[key]; exists {
		stats.Attempts = 0
		stats.LastError = nil
		stats.LastAttempt = time.Time{}
		stats.TotalDuration = 0
		stats.Success = false
	}
}

// WithContext 使用上下文执行重试
func (r *Retry) WithContext(ctx context.Context, key string, fn func() error) error {
	// 创建带超时的上下文
	timeoutCtx, cancel := context.WithTimeout(ctx, r.config.Timeout)
	defer cancel()

	// 获取或创建统计信息
	r.mu.Lock()
	stats, exists := r.stats[key]
	if !exists {
		stats = &RetryStats{}
		r.stats[key] = stats
	}
	r.mu.Unlock()

	// 重置统计信息
	stats.Attempts = 0
	stats.TotalDuration = 0
	stats.Success = false
	startTime := time.Now()

	// 执行重试
	var lastErr error
	for stats.Attempts < r.config.MaxAttempts {
		// 检查上下文是否取消
		if timeoutCtx.Err() != nil {
			return fmt.Errorf("retry timeout: %v", timeoutCtx.Err())
		}

		// 执行函数
		err := fn()
		if err == nil {
			stats.Success = true
			stats.LastError = nil
			break
		}

		// 更新统计信息
		stats.Attempts++
		stats.LastError = err
		stats.LastAttempt = time.Now()
		stats.TotalDuration = time.Since(startTime)

		// 计算下次重试间隔
		interval := r.calculateInterval(stats.Attempts)
		if interval > 0 {
			// 添加随机抖动
			if r.config.EnableJitter {
				interval = r.addJitter(interval)
			}

			// 等待重试
			select {
			case <-timeoutCtx.Done():
				return fmt.Errorf("retry timeout: %v", timeoutCtx.Err())
			case <-time.After(interval):
				r.logger.Debug("Retry attempt %d for key %s after %v", stats.Attempts, key, interval)
			}
		}
	}

	// 记录最终结果
	if !stats.Success {
		r.logger.Error("All retry attempts failed for key %s: %v", key, stats.LastError)
	}

	return stats.LastError
} 