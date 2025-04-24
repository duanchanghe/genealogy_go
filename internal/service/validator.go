package service

import (
	"fmt"
	"regexp"
	"strings"
	"time"
)

// Validator 数据验证服务
type Validator struct {
	errors []string
}

// NewValidator 创建验证器实例
func NewValidator() *Validator {
	return &Validator{
		errors: make([]string, 0),
	}
}

// Validate 执行验证并返回错误
func (v *Validator) Validate() error {
	if len(v.errors) > 0 {
		return fmt.Errorf("validation errors: %s", strings.Join(v.errors, "; "))
	}
	return nil
}

// Required 必填字段验证
func (v *Validator) Required(value interface{}, fieldName string) *Validator {
	if value == nil || (isString(value) && strings.TrimSpace(value.(string)) == "") {
		v.errors = append(v.errors, fmt.Sprintf("%s is required", fieldName))
	}
	return v
}

// MinLength 最小长度验证
func (v *Validator) MinLength(value string, fieldName string, min int) *Validator {
	if len(value) < min {
		v.errors = append(v.errors, fmt.Sprintf("%s must be at least %d characters", fieldName, min))
	}
	return v
}

// MaxLength 最大长度验证
func (v *Validator) MaxLength(value string, fieldName string, max int) *Validator {
	if len(value) > max {
		v.errors = append(v.errors, fmt.Sprintf("%s must be at most %d characters", fieldName, max))
	}
	return v
}

// Email 邮箱格式验证
func (v *Validator) Email(value string, fieldName string) *Validator {
	emailRegex := regexp.MustCompile(`^[a-z0-9._%+\-]+@[a-z0-9.\-]+\.[a-z]{2,4}$`)
	if !emailRegex.MatchString(strings.ToLower(value)) {
		v.errors = append(v.errors, fmt.Sprintf("%s must be a valid email address", fieldName))
	}
	return v
}

// Date 日期格式验证
func (v *Validator) Date(value string, fieldName string) *Validator {
	_, err := time.Parse("2006-01-02", value)
	if err != nil {
		v.errors = append(v.errors, fmt.Sprintf("%s must be a valid date (YYYY-MM-DD)", fieldName))
	}
	return v
}

// DateRange 日期范围验证
func (v *Validator) DateRange(startDate, endDate string, fieldName string) *Validator {
	start, err1 := time.Parse("2006-01-02", startDate)
	end, err2 := time.Parse("2006-01-02", endDate)
	
	if err1 != nil || err2 != nil {
		v.errors = append(v.errors, fmt.Sprintf("%s must be valid dates (YYYY-MM-DD)", fieldName))
		return v
	}
	
	if end.Before(start) {
		v.errors = append(v.errors, fmt.Sprintf("%s end date must be after start date", fieldName))
	}
	return v
}

// Gender 性别验证
func (v *Validator) Gender(value string, fieldName string) *Validator {
	validGenders := map[string]bool{
		"男": true,
		"女": true,
		"male": true,
		"female": true,
	}
	
	if !validGenders[value] {
		v.errors = append(v.errors, fmt.Sprintf("%s must be either '男' or '女'", fieldName))
	}
	return v
}

// FileType 文件类型验证
func (v *Validator) FileType(filename string, fieldName string, allowedTypes []string) *Validator {
	ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(filename), "."))
	allowed := false
	for _, t := range allowedTypes {
		if t == ext {
			allowed = true
			break
		}
	}
	
	if !allowed {
		v.errors = append(v.errors, fmt.Sprintf("%s must be one of the following types: %s", 
			fieldName, strings.Join(allowedTypes, ", ")))
	}
	return v
}

// FileSize 文件大小验证
func (v *Validator) FileSize(size int64, fieldName string, maxSize int64) *Validator {
	if size > maxSize {
		v.errors = append(v.errors, fmt.Sprintf("%s must be smaller than %d bytes", fieldName, maxSize))
	}
	return v
}

// isString 判断是否为字符串类型
func isString(value interface{}) bool {
	_, ok := value.(string)
	return ok
} 