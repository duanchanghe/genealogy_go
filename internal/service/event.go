package service

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// EventType 事件类型
type EventType string

const (
	EventTypeSystem   EventType = "system"   // 系统事件
	EventTypeUser     EventType = "user"     // 用户事件
	EventTypeBusiness EventType = "business" // 业务事件
	EventTypeError    EventType = "error"    // 错误事件
	EventTypeWarning  EventType = "warning"  // 警告事件
	EventTypeInfo     EventType = "info"     // 信息事件
)

// EventPriority 事件优先级
type EventPriority int

const (
	EventPriorityLow    EventPriority = 0 // 低优先级
	EventPriorityNormal EventPriority = 1 // 普通优先级
	EventPriorityHigh   EventPriority = 2 // 高优先级
	EventPriorityUrgent EventPriority = 3 // 紧急优先级
)

// Event 事件
type Event struct {
	ID          string                 `json:"id"`
	Type        EventType             `json:"type"`
	Source      string                `json:"source"`
	Priority    EventPriority         `json:"priority"`
	Data        map[string]interface{} `json:"data"`
	CreatedAt   time.Time             `json:"created_at"`
	ProcessedAt *time.Time            `json:"processed_at,omitempty"`
	Error       string                `json:"error,omitempty"`
}

// EventHandler 事件处理器
type EventHandler func(ctx context.Context, event *Event) error

// EventConfig 事件配置
type EventConfig struct {
	QueueSize          int           // 队列大小
	MaxHandlers        int           // 最大处理器数
	HandlerTimeout     time.Duration // 处理器超时时间
	RetryInterval      time.Duration // 重试间隔
	MaxRetries         int           // 最大重试次数
	CleanupInterval    time.Duration // 清理间隔
}

// EventService 事件服务
type EventService struct {
	config     *EventConfig
	logger     *Logger
	handlers   map[EventType][]EventHandler
	queue      chan *Event
	mu         sync.RWMutex
	stopCh     chan struct{}
	workerPool chan struct{}
}

// NewEventService 创建事件服务实例
func NewEventService(config *EventConfig, logger *Logger) *EventService {
	service := &EventService{
		config:     config,
		logger:     logger,
		handlers:   make(map[EventType][]EventHandler),
		queue:      make(chan *Event, config.QueueSize),
		stopCh:     make(chan struct{}),
		workerPool: make(chan struct{}, config.MaxHandlers),
	}

	// 启动事件处理
	go service.processEvents()

	return service
}

// Publish 发布事件
func (s *EventService) Publish(event *Event) error {
	// 设置事件属性
	event.ID = generateID()
	event.CreatedAt = time.Now()

	// 加入队列
	s.queue <- event

	return nil
}

// Subscribe 订阅事件
func (s *EventService) Subscribe(eventType EventType, handler EventHandler) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.handlers[eventType] = append(s.handlers[eventType], handler)
}

// Unsubscribe 取消订阅事件
func (s *EventService) Unsubscribe(eventType EventType, handler EventHandler) {
	s.mu.Lock()
	defer s.mu.Unlock()

	handlers := s.handlers[eventType]
	for i, h := range handlers {
		if &h == &handler {
			s.handlers[eventType] = append(handlers[:i], handlers[i+1:]...)
			break
		}
	}
}

// processEvents 处理事件
func (s *EventService) processEvents() {
	for {
		select {
		case <-s.stopCh:
			return
		case event := <-s.queue:
			// 获取工作协程
			s.workerPool <- struct{}{}

			go func(e *Event) {
				defer func() { <-s.workerPool }()

				// 处理事件
				if err := s.handleEvent(e); err != nil {
					s.logger.Error("Failed to handle event: %v", err)
					e.Error = err.Error()
				} else {
					now := time.Now()
					e.ProcessedAt = &now
				}
			}(event)
		}
	}
}

// handleEvent 处理单个事件
func (s *EventService) handleEvent(event *Event) error {
	s.mu.RLock()
	handlers := s.handlers[event.Type]
	s.mu.RUnlock()

	if len(handlers) == 0 {
		return fmt.Errorf("no handlers for event type: %s", event.Type)
	}

	var lastErr error
	for _, handler := range handlers {
		// 创建上下文
		ctx, cancel := context.WithTimeout(context.Background(), s.config.HandlerTimeout)
		defer cancel()

		// 执行处理器
		if err := handler(ctx, event); err != nil {
			lastErr = err
			s.logger.Error("Handler failed for event %s: %v", event.ID, err)
		}
	}

	return lastErr
}

// PublishAsync 异步发布事件
func (s *EventService) PublishAsync(event *Event) {
	go func() {
		if err := s.Publish(event); err != nil {
			s.logger.Error("Failed to publish event: %v", err)
		}
	}()
}

// PublishBatch 批量发布事件
func (s *EventService) PublishBatch(events []*Event) error {
	for _, event := range events {
		if err := s.Publish(event); err != nil {
			return fmt.Errorf("failed to publish event: %v", err)
		}
	}
	return nil
}

// GetHandlers 获取事件处理器
func (s *EventService) GetHandlers(eventType EventType) []EventHandler {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.handlers[eventType]
}

// GetStats 获取事件服务统计信息
func (s *EventService) GetStats() map[string]interface{} {
	s.mu.RLock()
	defer s.mu.RUnlock()

	stats := map[string]interface{}{
		"queue_size":      len(s.queue),
		"worker_count":    len(s.workerPool),
		"handler_count":   make(map[EventType]int),
		"total_handlers":  0,
	}

	for eventType, handlers := range s.handlers {
		stats["handler_count"].(map[EventType]int)[eventType] = len(handlers)
		stats["total_handlers"] = stats["total_handlers"].(int) + len(handlers)
	}

	return stats
}

// Stop 停止事件服务
func (s *EventService) Stop() {
	close(s.stopCh)
	close(s.queue)
} 