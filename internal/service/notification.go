package service

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

// NotificationType 通知类型
type NotificationType string

const (
	NotificationTypeSystem   NotificationType = "system"   // 系统通知
	NotificationTypeUser     NotificationType = "user"     // 用户通知
	NotificationTypeAlert    NotificationType = "alert"    // 警告通知
	NotificationTypeError    NotificationType = "error"    // 错误通知
	NotificationTypeSuccess  NotificationType = "success"  // 成功通知
	NotificationTypeInfo     NotificationType = "info"     // 信息通知
)

// NotificationPriority 通知优先级
type NotificationPriority int

const (
	NotificationPriorityLow    NotificationPriority = 0 // 低优先级
	NotificationPriorityNormal NotificationPriority = 1 // 普通优先级
	NotificationPriorityHigh   NotificationPriority = 2 // 高优先级
	NotificationPriorityUrgent NotificationPriority = 3 // 紧急优先级
)

// NotificationStatus 通知状态
type NotificationStatus string

const (
	NotificationStatusPending   NotificationStatus = "pending"   // 待发送
	NotificationStatusSent      NotificationStatus = "sent"      // 已发送
	NotificationStatusFailed    NotificationStatus = "failed"    // 发送失败
	NotificationStatusRead      NotificationStatus = "read"      // 已读
	NotificationStatusArchived  NotificationStatus = "archived"  // 已归档
)

// NotificationChannel 通知渠道
type NotificationChannel string

const (
	NotificationChannelEmail    NotificationChannel = "email"    // 邮件
	NotificationChannelSMS      NotificationChannel = "sms"      // 短信
	NotificationChannelWeb      NotificationChannel = "web"      // 网页
	NotificationChannelApp      NotificationChannel = "app"      // 应用内
	NotificationChannelWeChat   NotificationChannel = "wechat"   // 微信
)

// Notification 通知
type Notification struct {
	ID          string              `json:"id"`
	Type        NotificationType    `json:"type"`
	Title       string              `json:"title"`
	Content     string              `json:"content"`
	Priority    NotificationPriority `json:"priority"`
	Status      NotificationStatus  `json:"status"`
	Channels    []NotificationChannel `json:"channels"`
	Recipients  []string            `json:"recipients"`
	Data        map[string]interface{} `json:"data,omitempty"`
	CreatedAt   time.Time           `json:"created_at"`
	UpdatedAt   time.Time           `json:"updated_at"`
	SentAt      *time.Time          `json:"sent_at,omitempty"`
	ReadAt      *time.Time          `json:"read_at,omitempty"`
	Error       string              `json:"error,omitempty"`
}

// NotificationConfig 通知配置
type NotificationConfig struct {
	DefaultChannels []NotificationChannel // 默认通知渠道
	RetryInterval   time.Duration        // 重试间隔
	MaxRetries      int                  // 最大重试次数
	BatchSize       int                  // 批量发送大小
	QueueSize       int                  // 队列大小
}

// NotificationService 通知服务
type NotificationService struct {
	config     *NotificationConfig
	logger     *Logger
	mail       *Mail
	sms        *SMS
	cache      *Cache
	queue      chan *Notification
	mu         sync.RWMutex
	notifications map[string]*Notification
}

// NewNotificationService 创建通知服务实例
func NewNotificationService(config *NotificationConfig, logger *Logger, mail *Mail, sms *SMS, cache *Cache) *NotificationService {
	service := &NotificationService{
		config:     config,
		logger:     logger,
		mail:       mail,
		sms:        sms,
		cache:      cache,
		queue:      make(chan *Notification, config.QueueSize),
		notifications: make(map[string]*Notification),
	}

	// 启动处理队列的goroutine
	go service.processQueue()

	return service
}

// Send 发送通知
func (s *NotificationService) Send(notification *Notification) error {
	// 设置默认值
	if len(notification.Channels) == 0 {
		notification.Channels = s.config.DefaultChannels
	}
	if notification.Priority == 0 {
		notification.Priority = NotificationPriorityNormal
	}
	if notification.Status == "" {
		notification.Status = NotificationStatusPending
	}

	// 生成ID和时间戳
	notification.ID = generateID()
	notification.CreatedAt = time.Now()
	notification.UpdatedAt = notification.CreatedAt

	// 保存通知
	s.mu.Lock()
	s.notifications[notification.ID] = notification
	s.mu.Unlock()

	// 加入队列
	s.queue <- notification

	return nil
}

// SendBatch 批量发送通知
func (s *NotificationService) SendBatch(notifications []*Notification) error {
	for _, notification := range notifications {
		if err := s.Send(notification); err != nil {
			return fmt.Errorf("failed to send notification: %v", err)
		}
	}
	return nil
}

// processQueue 处理通知队列
func (s *NotificationService) processQueue() {
	for notification := range s.queue {
		// 发送通知
		if err := s.sendNotification(notification); err != nil {
			s.logger.Error("Failed to send notification: %v", err)
			notification.Status = NotificationStatusFailed
			notification.Error = err.Error()
		} else {
			notification.Status = NotificationStatusSent
			now := time.Now()
			notification.SentAt = &now
		}

		// 更新通知
		notification.UpdatedAt = time.Now()
		s.mu.Lock()
		s.notifications[notification.ID] = notification
		s.mu.Unlock()
	}
}

// sendNotification 发送单个通知
func (s *NotificationService) sendNotification(notification *Notification) error {
	var lastErr error

	for _, channel := range notification.Channels {
		switch channel {
		case NotificationChannelEmail:
			if err := s.sendEmail(notification); err != nil {
				lastErr = err
				s.logger.Error("Failed to send email notification: %v", err)
			}
		case NotificationChannelSMS:
			if err := s.sendSMS(notification); err != nil {
				lastErr = err
				s.logger.Error("Failed to send SMS notification: %v", err)
			}
		case NotificationChannelWeb:
			if err := s.sendWeb(notification); err != nil {
				lastErr = err
				s.logger.Error("Failed to send web notification: %v", err)
			}
		case NotificationChannelApp:
			if err := s.sendApp(notification); err != nil {
				lastErr = err
				s.logger.Error("Failed to send app notification: %v", err)
			}
		case NotificationChannelWeChat:
			if err := s.sendWeChat(notification); err != nil {
				lastErr = err
				s.logger.Error("Failed to send WeChat notification: %v", err)
			}
		}
	}

	return lastErr
}

// sendEmail 发送邮件通知
func (s *NotificationService) sendEmail(notification *Notification) error {
	// 构建邮件内容
	content := fmt.Sprintf("%s\n\n%s", notification.Title, notification.Content)
	
	// 发送邮件
	return s.mail.SendMail(notification.Recipients[0], notification.Title, content)
}

// sendSMS 发送短信通知
func (s *NotificationService) sendSMS(notification *Notification) error {
	// 发送短信
	return s.sms.SendSMS(notification.Recipients[0], notification.Content)
}

// sendWeb 发送网页通知
func (s *NotificationService) sendWeb(notification *Notification) error {
	// 将通知保存到缓存
	key := fmt.Sprintf("notification:web:%s", notification.ID)
	return s.cache.Set(key, notification, 24*time.Hour)
}

// sendApp 发送应用内通知
func (s *NotificationService) sendApp(notification *Notification) error {
	// 将通知保存到缓存
	key := fmt.Sprintf("notification:app:%s", notification.ID)
	return s.cache.Set(key, notification, 24*time.Hour)
}

// sendWeChat 发送微信通知
func (s *NotificationService) sendWeChat(notification *Notification) error {
	// TODO: 实现微信通知发送
	return nil
}

// Get 获取通知
func (s *NotificationService) Get(id string) (*Notification, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	notification, exists := s.notifications[id]
	if !exists {
		return nil, fmt.Errorf("notification not found: %s", id)
	}

	return notification, nil
}

// List 获取通知列表
func (s *NotificationService) List(userID string, status NotificationStatus, page, pageSize int) ([]*Notification, int, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var notifications []*Notification
	for _, notification := range s.notifications {
		// 检查用户是否在接收者列表中
		isRecipient := false
		for _, recipient := range notification.Recipients {
			if recipient == userID {
				isRecipient = true
				break
			}
		}
		if !isRecipient {
			continue
		}

		// 检查状态
		if status != "" && notification.Status != status {
			continue
		}

		notifications = append(notifications, notification)
	}

	// 计算总数
	total := len(notifications)

	// 应用分页
	start := (page - 1) * pageSize
	end := start + pageSize
	if start >= total {
		return []*Notification{}, total, nil
	}
	if end > total {
		end = total
	}

	return notifications[start:end], total, nil
}

// MarkAsRead 标记通知为已读
func (s *NotificationService) MarkAsRead(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	notification, exists := s.notifications[id]
	if !exists {
		return fmt.Errorf("notification not found: %s", id)
	}

	notification.Status = NotificationStatusRead
	now := time.Now()
	notification.ReadAt = &now
	notification.UpdatedAt = now

	return nil
}

// MarkAsArchived 标记通知为已归档
func (s *NotificationService) MarkAsArchived(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	notification, exists := s.notifications[id]
	if !exists {
		return fmt.Errorf("notification not found: %s", id)
	}

	notification.Status = NotificationStatusArchived
	notification.UpdatedAt = time.Now()

	return nil
}

// Delete 删除通知
func (s *NotificationService) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.notifications[id]; !exists {
		return fmt.Errorf("notification not found: %s", id)
	}

	delete(s.notifications, id)
	return nil
}

// GetStats 获取通知统计信息
func (s *NotificationService) GetStats() map[string]interface{} {
	s.mu.RLock()
	defer s.mu.RUnlock()

	stats := map[string]interface{}{
		"total":           len(s.notifications),
		"pending":         0,
		"sent":           0,
		"failed":         0,
		"read":           0,
		"archived":       0,
		"by_type":        make(map[NotificationType]int),
		"by_priority":    make(map[NotificationPriority]int),
		"by_channel":     make(map[NotificationChannel]int),
	}

	for _, notification := range s.notifications {
		// 统计状态
		switch notification.Status {
		case NotificationStatusPending:
			stats["pending"] = stats["pending"].(int) + 1
		case NotificationStatusSent:
			stats["sent"] = stats["sent"].(int) + 1
		case NotificationStatusFailed:
			stats["failed"] = stats["failed"].(int) + 1
		case NotificationStatusRead:
			stats["read"] = stats["read"].(int) + 1
		case NotificationStatusArchived:
			stats["archived"] = stats["archived"].(int) + 1
		}

		// 统计类型
		stats["by_type"].(map[NotificationType]int)[notification.Type]++

		// 统计优先级
		stats["by_priority"].(map[NotificationPriority]int)[notification.Priority]++

		// 统计渠道
		for _, channel := range notification.Channels {
			stats["by_channel"].(map[NotificationChannel]int)[channel]++
		}
	}

	return stats
} 