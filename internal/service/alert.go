package service

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// AlertLevel 告警级别
type AlertLevel string

const (
	AlertLevelInfo     AlertLevel = "info"     // 信息
	AlertLevelWarning  AlertLevel = "warning"  // 警告
	AlertLevelError    AlertLevel = "error"    // 错误
	AlertLevelCritical AlertLevel = "critical" // 严重
)

// AlertStatus 告警状态
type AlertStatus string

const (
	AlertStatusActive    AlertStatus = "active"    // 活跃
	AlertStatusResolved  AlertStatus = "resolved"  // 已解决
	AlertStatusAcknowledged AlertStatus = "acknowledged" // 已确认
	AlertStatusSuppressed AlertStatus = "suppressed" // 已抑制
)

// AlertRule 告警规则
type AlertRule struct {
	Name        string            // 规则名称
	Description string            // 规则描述
	Level       AlertLevel        // 告警级别
	Condition   string            // 告警条件
	Threshold   float64           // 告警阈值
	Duration    time.Duration     // 持续时间
	Labels      map[string]string // 标签
}

// Alert 告警
type Alert struct {
	ID          string            // 告警ID
	Rule        *AlertRule        // 告警规则
	Status      AlertStatus       // 告警状态
	Value       float64           // 告警值
	Message     string            // 告警消息
	Labels      map[string]string // 标签
	StartTime   time.Time         // 开始时间
	EndTime     time.Time         // 结束时间
	UpdateTime  time.Time         // 更新时间
	ResolveTime time.Time         // 解决时间
}

// AlertConfig 告警配置
type AlertConfig struct {
	CheckInterval    time.Duration // 检查间隔
	RetentionPeriod  time.Duration // 保留期限
	MaxAlerts        int           // 最大告警数
	EnableDeduplication bool       // 启用去重
	DeduplicationWindow time.Duration // 去重窗口
	EnableNotification bool        // 启用通知
	NotificationChannels []string  // 通知渠道
}

// AlertService 告警服务
type AlertService struct {
	config     *AlertConfig
	logger     *Logger
	db         *Database
	mail       *Mail
	sms        *SMS
	rules      map[string]*AlertRule
	alerts     map[string]*Alert
	mu         sync.RWMutex
	stopCh     chan struct{}
}

// NewAlertService 创建告警服务实例
func NewAlertService(config *AlertConfig, logger *Logger, db *Database, mail *Mail, sms *SMS) *AlertService {
	service := &AlertService{
		config:  config,
		logger:  logger,
		db:      db,
		mail:    mail,
		sms:     sms,
		rules:   make(map[string]*AlertRule),
		alerts:  make(map[string]*Alert),
		stopCh:  make(chan struct{}),
	}

	// 启动检查
	go service.check()

	return service
}

// AddRule 添加告警规则
func (s *AlertService) AddRule(rule *AlertRule) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.rules[rule.Name]; exists {
		return fmt.Errorf("rule %s already exists", rule.Name)
	}

	s.rules[rule.Name] = rule
	return nil
}

// RemoveRule 移除告警规则
func (s *AlertService) RemoveRule(name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.rules[name]; !exists {
		return fmt.Errorf("rule %s not found", name)
	}

	delete(s.rules, name)
	return nil
}

// GetRule 获取告警规则
func (s *AlertService) GetRule(name string) *AlertRule {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.rules[name]
}

// ListRules 列出所有告警规则
func (s *AlertService) ListRules() []*AlertRule {
	s.mu.RLock()
	defer s.mu.RUnlock()

	rules := make([]*AlertRule, 0, len(s.rules))
	for _, rule := range s.rules {
		rules = append(rules, rule)
	}
	return rules
}

// GetAlert 获取告警
func (s *AlertService) GetAlert(id string) *Alert {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.alerts[id]
}

// ListAlerts 列出所有告警
func (s *AlertService) ListAlerts() []*Alert {
	s.mu.RLock()
	defer s.mu.RUnlock()

	alerts := make([]*Alert, 0, len(s.alerts))
	for _, alert := range s.alerts {
		alerts = append(alerts, alert)
	}
	return alerts
}

// AcknowledgeAlert 确认告警
func (s *AlertService) AcknowledgeAlert(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	alert, exists := s.alerts[id]
	if !exists {
		return fmt.Errorf("alert %s not found", id)
	}

	alert.Status = AlertStatusAcknowledged
	alert.UpdateTime = time.Now()
	return nil
}

// ResolveAlert 解决告警
func (s *AlertService) ResolveAlert(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	alert, exists := s.alerts[id]
	if !exists {
		return fmt.Errorf("alert %s not found", id)
	}

	alert.Status = AlertStatusResolved
	alert.ResolveTime = time.Now()
	alert.UpdateTime = time.Now()
	return nil
}

// check 检查告警
func (s *AlertService) check() {
	ticker := time.NewTicker(s.config.CheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-s.stopCh:
			return
		case <-ticker.C:
			s.checkRules()
			s.cleanup()
		}
	}
}

// checkRules 检查告警规则
func (s *AlertService) checkRules() {
	s.mu.RLock()
	rules := s.rules
	s.mu.RUnlock()

	for _, rule := range rules {
		// TODO: 实现规则检查
		// - 获取指标值
		// - 检查条件
		// - 创建告警
		// - 发送通知
	}
}

// createAlert 创建告警
func (s *AlertService) createAlert(rule *AlertRule, value float64, message string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 检查去重
	if s.config.EnableDeduplication {
		if s.isDuplicate(rule, value) {
			return
		}
	}

	// 创建告警
	alert := &Alert{
		ID:         fmt.Sprintf("%d", time.Now().UnixNano()),
		Rule:       rule,
		Status:     AlertStatusActive,
		Value:      value,
		Message:    message,
		Labels:     rule.Labels,
		StartTime:  time.Now(),
		UpdateTime: time.Now(),
	}

	// 保存告警
	s.alerts[alert.ID] = alert

	// 发送通知
	if s.config.EnableNotification {
		s.sendNotification(alert)
	}
}

// isDuplicate 检查是否重复告警
func (s *AlertService) isDuplicate(rule *AlertRule, value float64) bool {
	now := time.Now()
	for _, alert := range s.alerts {
		if alert.Rule.Name == rule.Name &&
			alert.Status == AlertStatusActive &&
			now.Sub(alert.StartTime) < s.config.DeduplicationWindow {
			return true
		}
	}
	return false
}

// sendNotification 发送通知
func (s *AlertService) sendNotification(alert *Alert) {
	// 构建通知内容
	subject := fmt.Sprintf("[%s] %s", alert.Rule.Level, alert.Rule.Name)
	content := fmt.Sprintf("告警规则: %s\n告警级别: %s\n告警消息: %s\n告警值: %f\n开始时间: %s",
		alert.Rule.Name,
		alert.Rule.Level,
		alert.Message,
		alert.Value,
		alert.StartTime.Format("2006-01-02 15:04:05"),
	)

	// 发送通知
	for _, channel := range s.config.NotificationChannels {
		switch channel {
		case "email":
			s.mail.SendNotificationMail(subject, content)
		case "sms":
			s.sms.SendNotificationSMS(content)
		}
	}
}

// cleanup 清理过期告警
func (s *AlertService) cleanup() {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	for id, alert := range s.alerts {
		if now.Sub(alert.UpdateTime) > s.config.RetentionPeriod {
			delete(s.alerts, id)
		}
	}
}

// Stop 停止告警服务
func (s *AlertService) Stop() {
	close(s.stopCh)
} 