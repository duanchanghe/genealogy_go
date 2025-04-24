package service

import (
	"fmt"
	"runtime"
	"strings"
)

// ErrorCode 错误码类型
type ErrorCode int

const (
	// 系统级错误码 (1-999)
	ErrSystem ErrorCode = iota + 1
	ErrConfig
	ErrDatabase
	ErrValidation
	ErrAuthentication
	ErrAuthorization
	ErrNotFound
	ErrDuplicate
	ErrInvalidInput
	ErrInternal

	// 业务级错误码 (1000-9999)
	ErrUserNotFound ErrorCode = iota + 1000
	ErrUserExists
	ErrInvalidPassword
	ErrInvalidToken
	ErrTokenExpired
	ErrInsufficientPermissions
)

// AppError 应用程序错误
type AppError struct {
	Code    ErrorCode   // 错误码
	Message string      // 错误消息
	Err     error      // 原始错误
	Stack   string     // 堆栈信息
	Context map[string]interface{} // 上下文信息
}

// Error 实现error接口
func (e *AppError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("[%d] %s: %v", e.Code, e.Message, e.Err)
	}
	return fmt.Sprintf("[%d] %s", e.Code, e.Message)
}

// Unwrap 实现errors.Unwrap接口
func (e *AppError) Unwrap() error {
	return e.Err
}

// NewError 创建新的应用程序错误
func NewError(code ErrorCode, message string, err error) *AppError {
	return &AppError{
		Code:    code,
		Message: message,
		Err:     err,
		Stack:   getStack(),
		Context: make(map[string]interface{}),
	}
}

// WithContext 添加上下文信息
func (e *AppError) WithContext(key string, value interface{}) *AppError {
	e.Context[key] = value
	return e
}

// Is 检查错误码是否匹配
func (e *AppError) Is(code ErrorCode) bool {
	return e.Code == code
}

// getStack 获取当前goroutine的堆栈信息
func getStack() string {
	const depth = 32
	var pcs [depth]uintptr
	n := runtime.Callers(3, pcs[:])
	frames := runtime.CallersFrames(pcs[:n])
	
	var sb strings.Builder
	for {
		frame, more := frames.Next()
		sb.WriteString(fmt.Sprintf("%s:%d\n", frame.File, frame.Line))
		if !more {
			break
		}
	}
	return sb.String()
}

// ErrorHandler 错误处理服务
type ErrorHandler struct {
	logger *Logger
}

// NewErrorHandler 创建错误处理服务实例
func NewErrorHandler(logger *Logger) *ErrorHandler {
	return &ErrorHandler{
		logger: logger,
	}
}

// Handle 处理错误
func (h *ErrorHandler) Handle(err error) {
	if err == nil {
		return
	}

	// 转换为AppError
	var appErr *AppError
	if !strings.Contains(err.Error(), "[") {
		appErr = NewError(ErrInternal, "Internal Server Error", err)
	} else {
		// 尝试从错误消息中提取错误码
		var code ErrorCode
		fmt.Sscanf(err.Error(), "[%d]", &code)
		appErr = NewError(code, err.Error(), nil)
	}

	// 记录错误日志
	h.logger.Error("Error occurred: %v\nStack trace:\n%s", appErr, appErr.Stack)

	// 根据错误类型进行不同处理
	switch appErr.Code {
	case ErrValidation:
		// 处理验证错误
		h.handleValidationError(appErr)
	case ErrAuthentication, ErrAuthorization:
		// 处理认证授权错误
		h.handleAuthError(appErr)
	case ErrDatabase:
		// 处理数据库错误
		h.handleDatabaseError(appErr)
	default:
		// 处理其他错误
		h.handleGenericError(appErr)
	}
}

// handleValidationError 处理验证错误
func (h *ErrorHandler) handleValidationError(err *AppError) {
	// TODO: 实现验证错误的具体处理逻辑
}

// handleAuthError 处理认证授权错误
func (h *ErrorHandler) handleAuthError(err *AppError) {
	// TODO: 实现认证授权错误的具体处理逻辑
}

// handleDatabaseError 处理数据库错误
func (h *ErrorHandler) handleDatabaseError(err *AppError) {
	// TODO: 实现数据库错误的具体处理逻辑
}

// handleGenericError 处理通用错误
func (h *ErrorHandler) handleGenericError(err *AppError) {
	// TODO: 实现通用错误的具体处理逻辑
} 