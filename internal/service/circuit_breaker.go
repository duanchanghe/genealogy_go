package service

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// CircuitState 熔断器状态
type CircuitState string

const (
	CircuitStateClosed     CircuitState = "closed"     // 闭合状态（正常）
	CircuitStateOpen       CircuitState = "open"       // 断开状态（熔断）
	CircuitStateHalfOpen   CircuitState = "half_open"  // 半开状态（恢复中）
)

// CircuitConfig 熔断器配置
type CircuitConfig struct {
	FailureThreshold     int           // 失败阈值
	ResetTimeout         time.Duration // 重置超时时间
	HalfOpenRequests     int           // 半开状态请求数
	WindowSize           time.Duration // 时间窗口大小
	ErrorRateThreshold   float64       // 错误率阈值
	EnableDistributed    bool          // 启用分布式熔断
	DistributedKey       string        // 分布式熔断键
	DistributedTimeout   time.Duration // 分布式超时时间
}

// CircuitStats 熔断器统计
type CircuitStats struct {
	TotalRequests     int       // 总请求数
	FailedRequests    int       // 失败请求数
	LastFailureTime   time.Time // 最后失败时间
	LastSuccessTime   time.Time // 最后成功时间
	CurrentState      CircuitState // 当前状态
	HalfOpenSuccess   int       // 半开状态成功数
	HalfOpenFailures  int       // 半开状态失败数
}

// CircuitBreaker 熔断器
type CircuitBreaker struct {
	config     *CircuitConfig
	logger     *Logger
	db         *Database
	stats      map[string]*CircuitStats
	mu         sync.RWMutex
	stopCh     chan struct{}
}

// NewCircuitBreaker 创建熔断器实例
func NewCircuitBreaker(config *CircuitConfig, logger *Logger, db *Database) *CircuitBreaker {
	breaker := &CircuitBreaker{
		config:  config,
		logger:  logger,
		db:      db,
		stats:   make(map[string]*CircuitStats),
		stopCh:  make(chan struct{}),
	}

	// 启动状态检查
	go breaker.checkState()

	return breaker
}

// Execute 执行请求
func (b *CircuitBreaker) Execute(key string, fn func() error) error {
	// 获取或创建统计信息
	b.mu.Lock()
	stats, exists := b.stats[key]
	if !exists {
		stats = &CircuitStats{
			CurrentState: CircuitStateClosed,
		}
		b.stats[key] = stats
	}
	b.mu.Unlock()

	// 检查熔断器状态
	if !b.allowRequest(stats) {
		return fmt.Errorf("circuit breaker is open for key: %s", key)
	}

	// 执行请求
	err := fn()

	// 更新统计信息
	b.mu.Lock()
	defer b.mu.Unlock()

	stats.TotalRequests++
	if err != nil {
		stats.FailedRequests++
		stats.LastFailureTime = time.Now()

		// 检查是否需要熔断
		if b.shouldTrip(stats) {
			stats.CurrentState = CircuitStateOpen
			b.logger.Warn("Circuit breaker tripped for key: %s", key)
		}
	} else {
		stats.LastSuccessTime = time.Now()
		if stats.CurrentState == CircuitStateHalfOpen {
			stats.HalfOpenSuccess++
			if stats.HalfOpenSuccess >= b.config.HalfOpenRequests {
				stats.CurrentState = CircuitStateClosed
				stats.HalfOpenSuccess = 0
				stats.HalfOpenFailures = 0
				b.logger.Info("Circuit breaker reset for key: %s", key)
			}
		}
	}

	return err
}

// allowRequest 检查是否允许请求
func (b *CircuitBreaker) allowRequest(stats *CircuitStats) bool {
	switch stats.CurrentState {
	case CircuitStateClosed:
		return true
	case CircuitStateOpen:
		// 检查是否超过重置时间
		if time.Since(stats.LastFailureTime) >= b.config.ResetTimeout {
			stats.CurrentState = CircuitStateHalfOpen
			b.logger.Info("Circuit breaker entering half-open state")
			return true
		}
		return false
	case CircuitStateHalfOpen:
		// 限制半开状态的请求数
		return stats.HalfOpenSuccess+stats.HalfOpenFailures < b.config.HalfOpenRequests
	default:
		return true
	}
}

// shouldTrip 检查是否需要熔断
func (b *CircuitBreaker) shouldTrip(stats *CircuitStats) bool {
	// 检查失败次数
	if stats.FailedRequests >= b.config.FailureThreshold {
		return true
	}

	// 检查错误率
	if stats.TotalRequests > 0 {
		errorRate := float64(stats.FailedRequests) / float64(stats.TotalRequests)
		if errorRate >= b.config.ErrorRateThreshold {
			return true
		}
	}

	return false
}

// checkState 检查熔断器状态
func (b *CircuitBreaker) checkState() {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-b.stopCh:
			return
		case <-ticker.C:
			b.mu.Lock()
			now := time.Now()
			for key, stats := range b.stats {
				// 检查是否需要重置统计信息
				if now.Sub(stats.LastFailureTime) >= b.config.WindowSize {
					stats.TotalRequests = 0
					stats.FailedRequests = 0
					stats.HalfOpenSuccess = 0
					stats.HalfOpenFailures = 0
				}

				// 检查是否需要从断开状态恢复
				if stats.CurrentState == CircuitStateOpen &&
					now.Sub(stats.LastFailureTime) >= b.config.ResetTimeout {
					stats.CurrentState = CircuitStateHalfOpen
					b.logger.Info("Circuit breaker entering half-open state for key: %s", key)
				}
			}
			b.mu.Unlock()
		}
	}
}

// GetState 获取熔断器状态
func (b *CircuitBreaker) GetState(key string) CircuitState {
	b.mu.RLock()
	defer b.mu.RUnlock()

	if stats, exists := b.stats[key]; exists {
		return stats.CurrentState
	}
	return CircuitStateClosed
}

// GetStats 获取熔断器统计信息
func (b *CircuitBreaker) GetStats(key string) *CircuitStats {
	b.mu.RLock()
	defer b.mu.RUnlock()

	if stats, exists := b.stats[key]; exists {
		return stats
	}
	return nil
}

// Reset 重置熔断器
func (b *CircuitBreaker) Reset(key string) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if stats, exists := b.stats[key]; exists {
		stats.CurrentState = CircuitStateClosed
		stats.TotalRequests = 0
		stats.FailedRequests = 0
		stats.HalfOpenSuccess = 0
		stats.HalfOpenFailures = 0
	}
}

// Stop 停止熔断器
func (b *CircuitBreaker) Stop() {
	close(b.stopCh)
} 