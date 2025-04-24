package service

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"time"
)

// LogLevel 日志级别
type LogLevel string

const (
	LogLevelDebug   LogLevel = "debug"   // 调试
	LogLevelInfo    LogLevel = "info"    // 信息
	LogLevelWarn    LogLevel = "warn"    // 警告
	LogLevelError   LogLevel = "error"   // 错误
	LogLevelFatal   LogLevel = "fatal"   // 致命
)

// LogFormat 日志格式
type LogFormat string

const (
	LogFormatText LogFormat = "text" // 文本格式
	LogFormatJSON LogFormat = "json" // JSON格式
)

// LogEntry 日志条目
type LogEntry struct {
	Level     LogLevel          // 日志级别
	Message   string           // 日志消息
	Time      time.Time        // 时间戳
	File      string           // 文件名
	Line      int              // 行号
	Function  string           // 函数名
	TraceID   string           // 追踪ID
	Labels    map[string]string // 标签
	Data      interface{}      // 数据
}

// LoggerConfig 日志配置
type LoggerConfig struct {
	Level           LogLevel         // 日志级别
	Format          LogFormat        // 日志格式
	Output          []string         // 输出目标
	FilePath        string           // 文件路径
	MaxSize         int64            // 最大大小
	MaxBackups      int              // 最大备份数
	MaxAge          int              // 最大保留天数
	Compress        bool             // 是否压缩
	EnableCaller    bool             // 启用调用者信息
	EnableStack     bool             // 启用堆栈信息
	EnableColors    bool             // 启用颜色
	TimeFormat      string           // 时间格式
	EnableAsync     bool             // 启用异步
	BufferSize      int              // 缓冲区大小
	FlushInterval   time.Duration    // 刷新间隔
}

// Logger 日志器
type Logger struct {
	config     *LoggerConfig
	outputs    []io.Writer
	mu         sync.RWMutex
	entryCh    chan *LogEntry
	stopCh     chan struct{}
}

// NewLogger 创建日志器实例
func NewLogger(config *LoggerConfig) *Logger {
	logger := &Logger{
		config:  config,
		outputs: make([]io.Writer, 0),
		stopCh:  make(chan struct{}),
	}

	// 初始化输出
	logger.initOutputs()

	// 启动异步处理
	if config.EnableAsync {
		logger.entryCh = make(chan *LogEntry, config.BufferSize)
		go logger.process()
	}

	return logger
}

// initOutputs 初始化输出
func (l *Logger) initOutputs() {
	// 添加标准输出
	if len(l.config.Output) == 0 {
		l.outputs = append(l.outputs, os.Stdout)
		return
	}

	// 添加文件输出
	for _, output := range l.config.Output {
		switch output {
		case "stdout":
			l.outputs = append(l.outputs, os.Stdout)
		case "stderr":
			l.outputs = append(l.outputs, os.Stderr)
		case "file":
			if l.config.FilePath != "" {
				file, err := os.OpenFile(l.config.FilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
				if err != nil {
					fmt.Printf("Failed to open log file: %v\n", err)
					continue
				}
				l.outputs = append(l.outputs, file)
			}
		}
	}
}

// process 处理日志
func (l *Logger) process() {
	ticker := time.NewTicker(l.config.FlushInterval)
	defer ticker.Stop()

	for {
		select {
		case <-l.stopCh:
			return
		case entry := <-l.entryCh:
			l.write(entry)
		case <-ticker.C:
			// 定期刷新
		}
	}
}

// write 写入日志
func (l *Logger) write(entry *LogEntry) {
	// 格式化日志
	var output string
	switch l.config.Format {
	case LogFormatText:
		output = l.formatText(entry)
	case LogFormatJSON:
		output = l.formatJSON(entry)
	}

	// 写入输出
	for _, writer := range l.outputs {
		writer.Write([]byte(output + "\n"))
	}
}

// formatText 格式化文本日志
func (l *Logger) formatText(entry *LogEntry) string {
	// 构建基本格式
	format := "%s [%s] %s"
	args := []interface{}{
		entry.Time.Format(l.config.TimeFormat),
		entry.Level,
		entry.Message,
	}

	// 添加调用者信息
	if l.config.EnableCaller {
		format += " %s:%d %s"
		args = append(args, entry.File, entry.Line, entry.Function)
	}

	// 添加追踪ID
	if entry.TraceID != "" {
		format += " trace_id=%s"
		args = append(args, entry.TraceID)
	}

	// 添加标签
	if len(entry.Labels) > 0 {
		for k, v := range entry.Labels {
			format += " %s=%s"
			args = append(args, k, v)
		}
	}

	// 添加数据
	if entry.Data != nil {
		format += " data=%v"
		args = append(args, entry.Data)
	}

	return fmt.Sprintf(format, args...)
}

// formatJSON 格式化JSON日志
func (l *Logger) formatJSON(entry *LogEntry) string {
	// TODO: 实现JSON格式化
	return ""
}

// log 记录日志
func (l *Logger) log(level LogLevel, message string, args ...interface{}) {
	// 检查日志级别
	if !l.isLevelEnabled(level) {
		return
	}

	// 创建日志条目
	entry := &LogEntry{
		Level:   level,
		Message: fmt.Sprintf(message, args...),
		Time:    time.Now(),
		Labels:  make(map[string]string),
	}

	// 添加调用者信息
	if l.config.EnableCaller {
		if pc, file, line, ok := runtime.Caller(2); ok {
			entry.File = filepath.Base(file)
			entry.Line = line
			entry.Function = runtime.FuncForPC(pc).Name()
		}
	}

	// 异步写入
	if l.config.EnableAsync {
		select {
		case l.entryCh <- entry:
		default:
			// 缓冲区已满，直接写入
			l.write(entry)
		}
	} else {
		l.write(entry)
	}
}

// isLevelEnabled 检查日志级别是否启用
func (l *Logger) isLevelEnabled(level LogLevel) bool {
	levels := map[LogLevel]int{
		LogLevelDebug: 0,
		LogLevelInfo:  1,
		LogLevelWarn:  2,
		LogLevelError: 3,
		LogLevelFatal: 4,
	}

	return levels[level] >= levels[l.config.Level]
}

// Debug 记录调试日志
func (l *Logger) Debug(message string, args ...interface{}) {
	l.log(LogLevelDebug, message, args...)
}

// Info 记录信息日志
func (l *Logger) Info(message string, args ...interface{}) {
	l.log(LogLevelInfo, message, args...)
}

// Warn 记录警告日志
func (l *Logger) Warn(message string, args ...interface{}) {
	l.log(LogLevelWarn, message, args...)
}

// Error 记录错误日志
func (l *Logger) Error(message string, args ...interface{}) {
	l.log(LogLevelError, message, args...)
}

// Fatal 记录致命日志
func (l *Logger) Fatal(message string, args ...interface{}) {
	l.log(LogLevelFatal, message, args...)
	os.Exit(1)
}

// WithContext 创建带上下文的日志器
func (l *Logger) WithContext(ctx context.Context) *Logger {
	// TODO: 实现上下文日志器
	return l
}

// WithFields 创建带字段的日志器
func (l *Logger) WithFields(fields map[string]string) *Logger {
	// TODO: 实现字段日志器
	return l
}

// Stop 停止日志器
func (l *Logger) Stop() {
	if l.config.EnableAsync {
		close(l.entryCh)
	}
	close(l.stopCh)
} 