package service

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// SpanKind 跨度类型
type SpanKind string

const (
	SpanKindClient   SpanKind = "client"   // 客户端
	SpanKindServer   SpanKind = "server"   // 服务端
	SpanKindProducer SpanKind = "producer" // 生产者
	SpanKindConsumer SpanKind = "consumer" // 消费者
	SpanKindInternal SpanKind = "internal" // 内部
)

// SpanStatus 跨度状态
type SpanStatus string

const (
	SpanStatusOK    SpanStatus = "ok"     // 成功
	SpanStatusError SpanStatus = "error"  // 错误
)

// Span 跨度
type Span struct {
	ID            string            // 跨度ID
	TraceID       string            // 追踪ID
	ParentID      string            // 父跨度ID
	Name          string            // 跨度名称
	Kind          SpanKind          // 跨度类型
	Status        SpanStatus        // 跨度状态
	StartTime     time.Time         // 开始时间
	EndTime       time.Time         // 结束时间
	Duration      time.Duration     // 持续时间
	Attributes    map[string]string // 属性
	Events        []SpanEvent       // 事件
	Links         []SpanLink        // 链接
	Error         error             // 错误
}

// SpanEvent 跨度事件
type SpanEvent struct {
	Name       string            // 事件名称
	Time       time.Time         // 事件时间
	Attributes map[string]string // 事件属性
}

// SpanLink 跨度链接
type SpanLink struct {
	TraceID    string            // 追踪ID
	SpanID     string            // 跨度ID
	Attributes map[string]string // 链接属性
}

// TraceConfig 追踪配置
type TraceConfig struct {
	ServiceName      string        // 服务名称
	ServiceVersion   string        // 服务版本
	Environment      string        // 环境
	SamplingRate     float64       // 采样率
	MaxSpansPerTrace int           // 每个追踪的最大跨度数
	MaxTraceDuration time.Duration // 最大追踪持续时间
	ExportInterval   time.Duration // 导出间隔
	EnableMetrics    bool          // 启用指标
	EnableLogging    bool          // 启用日志
}

// TraceService 追踪服务
type TraceService struct {
	config     *TraceConfig
	logger     *Logger
	spans      map[string]*Span
	mu         sync.RWMutex
	stopCh     chan struct{}
}

// NewTraceService 创建追踪服务实例
func NewTraceService(config *TraceConfig, logger *Logger) *TraceService {
	service := &TraceService{
		config: config,
		logger: logger,
		spans:  make(map[string]*Span),
		stopCh: make(chan struct{}),
	}

	// 启动导出
	go service.export()

	return service
}

// StartSpan 开始跨度
func (s *TraceService) StartSpan(ctx context.Context, name string, kind SpanKind) (*Span, context.Context) {
	// 创建跨度
	span := &Span{
		ID:         generateID(),
		TraceID:    getTraceID(ctx),
		ParentID:   getParentID(ctx),
		Name:       name,
		Kind:       kind,
		Status:     SpanStatusOK,
		StartTime:  time.Now(),
		Attributes: make(map[string]string),
		Events:     make([]SpanEvent, 0),
		Links:      make([]SpanLink, 0),
	}

	// 保存跨度
	s.mu.Lock()
	s.spans[span.ID] = span
	s.mu.Unlock()

	// 创建上下文
	newCtx := context.WithValue(ctx, traceKey{}, span)

	return span, newCtx
}

// EndSpan 结束跨度
func (s *TraceService) EndSpan(span *Span, err error) {
	// 更新跨度
	span.EndTime = time.Now()
	span.Duration = span.EndTime.Sub(span.StartTime)

	if err != nil {
		span.Status = SpanStatusError
		span.Error = err
	}

	// 记录日志
	if s.config.EnableLogging {
		s.logger.Info("Span ended",
			"trace_id", span.TraceID,
			"span_id", span.ID,
			"name", span.Name,
			"kind", span.Kind,
			"status", span.Status,
			"duration", span.Duration,
		)
	}
}

// AddEvent 添加事件
func (s *TraceService) AddEvent(span *Span, name string, attributes map[string]string) {
	event := SpanEvent{
		Name:       name,
		Time:       time.Now(),
		Attributes: attributes,
	}

	span.Events = append(span.Events, event)
}

// AddLink 添加链接
func (s *TraceService) AddLink(span *Span, traceID, spanID string, attributes map[string]string) {
	link := SpanLink{
		TraceID:    traceID,
		SpanID:     spanID,
		Attributes: attributes,
	}

	span.Links = append(span.Links, link)
}

// SetAttribute 设置属性
func (s *TraceService) SetAttribute(span *Span, key, value string) {
	span.Attributes[key] = value
}

// GetSpan 获取跨度
func (s *TraceService) GetSpan(id string) *Span {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.spans[id]
}

// GetTrace 获取追踪
func (s *TraceService) GetTrace(traceID string) []*Span {
	s.mu.RLock()
	defer s.mu.RUnlock()

	spans := make([]*Span, 0)
	for _, span := range s.spans {
		if span.TraceID == traceID {
			spans = append(spans, span)
		}
	}
	return spans
}

// export 导出追踪
func (s *TraceService) export() {
	ticker := time.NewTicker(s.config.ExportInterval)
	defer ticker.Stop()

	for {
		select {
		case <-s.stopCh:
			return
		case <-ticker.C:
			s.exportSpans()
		}
	}
}

// exportSpans 导出跨度
func (s *TraceService) exportSpans() {
	s.mu.Lock()
	defer s.mu.Unlock()

	// TODO: 实现跨度导出
	// - 导出到追踪系统
	// - 清理过期跨度
	// - 更新指标
}

// Stop 停止追踪服务
func (s *TraceService) Stop() {
	close(s.stopCh)
}

// 上下文键
type traceKey struct{}

// 从上下文获取追踪ID
func getTraceID(ctx context.Context) string {
	if span, ok := ctx.Value(traceKey{}).(*Span); ok {
		return span.TraceID
	}
	return generateID()
}

// 从上下文获取父跨度ID
func getParentID(ctx context.Context) string {
	if span, ok := ctx.Value(traceKey{}).(*Span); ok {
		return span.ID
	}
	return ""
}

// 生成ID
func generateID() string {
	return fmt.Sprintf("%d", time.Now().UnixNano())
} 