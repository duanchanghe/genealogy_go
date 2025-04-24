package service

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

// MessageType 消息类型
type MessageType string

const (
	MessageTypeTask     MessageType = "task"     // 任务消息
	MessageTypeEvent    MessageType = "event"    // 事件消息
	MessageTypeNotify   MessageType = "notify"   // 通知消息
	MessageTypeData     MessageType = "data"     // 数据消息
	MessageTypeCommand  MessageType = "command"  // 命令消息
	MessageTypeResponse MessageType = "response" // 响应消息
)

// MessagePriority 消息优先级
type MessagePriority int

const (
	MessagePriorityLow    MessagePriority = 0 // 低优先级
	MessagePriorityNormal MessagePriority = 1 // 普通优先级
	MessagePriorityHigh   MessagePriority = 2 // 高优先级
	MessagePriorityUrgent MessagePriority = 3 // 紧急优先级
)

// MessageStatus 消息状态
type MessageStatus string

const (
	MessageStatusPending   MessageStatus = "pending"   // 待处理
	MessageStatusProcessing MessageStatus = "processing" // 处理中
	MessageStatusCompleted MessageStatus = "completed" // 已完成
	MessageStatusFailed    MessageStatus = "failed"    // 处理失败
	MessageStatusRetrying  MessageStatus = "retrying"  // 重试中
)

// Message 消息
type Message struct {
	ID           string                 `json:"id"`
	Type         MessageType           `json:"type"`
	Topic        string                `json:"topic"`
	Priority     MessagePriority       `json:"priority"`
	Status       MessageStatus         `json:"status"`
	Data         map[string]interface{} `json:"data"`
	RetryCount   int                   `json:"retry_count"`
	MaxRetries   int                   `json:"max_retries"`
	CreatedAt    time.Time             `json:"created_at"`
	UpdatedAt    time.Time             `json:"updated_at"`
	ProcessedAt  *time.Time            `json:"processed_at,omitempty"`
	Error        string                `json:"error,omitempty"`
}

// MessageHandler 消息处理器
type MessageHandler func(ctx context.Context, message *Message) error

// QueueConfig 队列配置
type QueueConfig struct {
	QueueSize          int           // 队列大小
	MaxWorkers         int           // 最大工作协程数
	WorkerTimeout      time.Duration // 工作协程超时时间
	RetryInterval      time.Duration // 重试间隔
	MaxRetries         int           // 最大重试次数
	CleanupInterval    time.Duration // 清理间隔
	PersistencePath    string        // 持久化路径
	EnablePersistence  bool          // 启用持久化
}

// Queue 消息队列
type Queue struct {
	config     *QueueConfig
	logger     *Logger
	db         *Database
	handlers   map[string][]MessageHandler
	queues     map[string]chan *Message
	mu         sync.RWMutex
	stopCh     chan struct{}
	workerPool chan struct{}
}

// NewQueue 创建消息队列实例
func NewQueue(config *QueueConfig, logger *Logger, db *Database) *Queue {
	queue := &Queue{
		config:     config,
		logger:     logger,
		db:         db,
		handlers:   make(map[string][]MessageHandler),
		queues:     make(map[string]chan *Message),
		stopCh:     make(chan struct{}),
		workerPool: make(chan struct{}, config.MaxWorkers),
	}

	// 启动消息处理
	go queue.processMessages()

	// 如果启用持久化，加载持久化消息
	if config.EnablePersistence {
		queue.loadPersistedMessages()
	}

	return queue
}

// Publish 发布消息
func (q *Queue) Publish(topic string, message *Message) error {
	// 设置消息属性
	message.ID = generateID()
	message.Topic = topic
	message.Status = MessageStatusPending
	message.CreatedAt = time.Now()
	message.UpdatedAt = message.CreatedAt

	// 获取或创建主题队列
	q.mu.Lock()
	queue, exists := q.queues[topic]
	if !exists {
		queue = make(chan *Message, q.config.QueueSize)
		q.queues[topic] = queue
	}
	q.mu.Unlock()

	// 加入队列
	queue <- message

	// 如果启用持久化，保存消息
	if q.config.EnablePersistence {
		if err := q.persistMessage(message); err != nil {
			q.logger.Error("Failed to persist message: %v", err)
		}
	}

	return nil
}

// Subscribe 订阅消息
func (q *Queue) Subscribe(topic string, handler MessageHandler) {
	q.mu.Lock()
	defer q.mu.Unlock()

	q.handlers[topic] = append(q.handlers[topic], handler)
}

// Unsubscribe 取消订阅消息
func (q *Queue) Unsubscribe(topic string, handler MessageHandler) {
	q.mu.Lock()
	defer q.mu.Unlock()

	handlers := q.handlers[topic]
	for i, h := range handlers {
		if &h == &handler {
			q.handlers[topic] = append(handlers[:i], handlers[i+1:]...)
			break
		}
	}
}

// processMessages 处理消息
func (q *Queue) processMessages() {
	for {
		select {
		case <-q.stopCh:
			return
		default:
			q.mu.RLock()
			for topic, queue := range q.queues {
				select {
				case message := <-queue:
					// 获取工作协程
					q.workerPool <- struct{}{}

					go func(t string, m *Message) {
						defer func() { <-q.workerPool }()

						// 处理消息
						if err := q.handleMessage(t, m); err != nil {
							q.logger.Error("Failed to handle message: %v", err)
							m.Error = err.Error()
							m.Status = MessageStatusFailed
						} else {
							now := time.Now()
							m.ProcessedAt = &now
							m.Status = MessageStatusCompleted
						}
						m.UpdatedAt = time.Now()

						// 如果启用持久化，更新消息状态
						if q.config.EnablePersistence {
							if err := q.updateMessageStatus(m); err != nil {
								q.logger.Error("Failed to update message status: %v", err)
							}
						}
					}(topic, message)
				default:
					continue
				}
			}
			q.mu.RUnlock()
			time.Sleep(time.Millisecond * 100)
		}
	}
}

// handleMessage 处理单个消息
func (q *Queue) handleMessage(topic string, message *Message) error {
	q.mu.RLock()
	handlers := q.handlers[topic]
	q.mu.RUnlock()

	if len(handlers) == 0 {
		return fmt.Errorf("no handlers for topic: %s", topic)
	}

	// 更新消息状态
	message.Status = MessageStatusProcessing
	message.UpdatedAt = time.Now()

	var lastErr error
	for _, handler := range handlers {
		// 创建上下文
		ctx, cancel := context.WithTimeout(context.Background(), q.config.WorkerTimeout)
		defer cancel()

		// 执行处理器
		if err := handler(ctx, message); err != nil {
			lastErr = err
			q.logger.Error("Handler failed for message %s: %v", message.ID, err)

			// 如果未超过最大重试次数，则重试
			if message.RetryCount < message.MaxRetries {
				message.RetryCount++
				message.Status = MessageStatusRetrying
				message.UpdatedAt = time.Now()
				time.Sleep(q.config.RetryInterval)
				continue
			}
		}
		break
	}

	return lastErr
}

// persistMessage 持久化消息
func (q *Queue) persistMessage(message *Message) error {
	data, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %v", err)
	}

	// 保存到数据库
	return q.db.Create(&struct {
		ID        string    `gorm:"primaryKey"`
		Topic     string    `gorm:"index"`
		Data      []byte    `gorm:"type:jsonb"`
		Status    string    `gorm:"index"`
		CreatedAt time.Time `gorm:"index"`
		UpdatedAt time.Time
	}{
		ID:        message.ID,
		Topic:     message.Topic,
		Data:      data,
		Status:    string(message.Status),
		CreatedAt: message.CreatedAt,
		UpdatedAt: message.UpdatedAt,
	})
}

// updateMessageStatus 更新消息状态
func (q *Queue) updateMessageStatus(message *Message) error {
	return q.db.Model(&struct {
		ID        string    `gorm:"primaryKey"`
		Status    string    `gorm:"index"`
		UpdatedAt time.Time
	}{
		ID:        message.ID,
		Status:    string(message.Status),
		UpdatedAt: message.UpdatedAt,
	}).Updates(map[string]interface{}{
		"status":     message.Status,
		"updated_at": message.UpdatedAt,
	})
}

// loadPersistedMessages 加载持久化消息
func (q *Queue) loadPersistedMessages() {
	var messages []struct {
		ID        string    `gorm:"primaryKey"`
		Topic     string    `gorm:"index"`
		Data      []byte    `gorm:"type:jsonb"`
		Status    string    `gorm:"index"`
		CreatedAt time.Time `gorm:"index"`
		UpdatedAt time.Time
	}

	if err := q.db.Where("status IN ?", []string{
		string(MessageStatusPending),
		string(MessageStatusProcessing),
		string(MessageStatusRetrying),
	}).Find(&messages).Error; err != nil {
		q.logger.Error("Failed to load persisted messages: %v", err)
		return
	}

	for _, m := range messages {
		var message Message
		if err := json.Unmarshal(m.Data, &message); err != nil {
			q.logger.Error("Failed to unmarshal message: %v", err)
			continue
		}

		// 重新加入队列
		if err := q.Publish(m.Topic, &message); err != nil {
			q.logger.Error("Failed to republish message: %v", err)
		}
	}
}

// GetStats 获取队列统计信息
func (q *Queue) GetStats() map[string]interface{} {
	q.mu.RLock()
	defer q.mu.RUnlock()

	stats := map[string]interface{}{
		"topics":         len(q.queues),
		"handlers":       len(q.handlers),
		"worker_count":   len(q.workerPool),
		"queue_sizes":    make(map[string]int),
		"handler_counts": make(map[string]int),
	}

	for topic, queue := range q.queues {
		stats["queue_sizes"].(map[string]int)[topic] = len(queue)
	}
	for topic, handlers := range q.handlers {
		stats["handler_counts"].(map[string]int)[topic] = len(handlers)
	}

	return stats
}

// Stop 停止队列
func (q *Queue) Stop() {
	close(q.stopCh)
	for _, queue := range q.queues {
		close(queue)
	}
} 