package service

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// SMSConfig 短信配置
type SMSConfig struct {
	Provider    string // 短信服务提供商
	AccessKey   string // 访问密钥
	SecretKey   string // 密钥
	SignName    string // 短信签名
	TemplateID  string // 模板ID
	Endpoint    string // API端点
	MaxRetries  int    // 最大重试次数
	RetryDelay  int    // 重试延迟（秒）
}

// SMSResponse 短信响应
type SMSResponse struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	RequestID string `json:"requestId"`
}

// SMS 短信服务
type SMS struct {
	config *SMSConfig
	logger *Logger
	client *http.Client
}

// NewSMS 创建短信服务实例
func NewSMS(config *SMSConfig, logger *Logger) *SMS {
	return &SMS{
		config: config,
		logger: logger,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// SendSMS 发送短信
func (s *SMS) SendSMS(phone, templateCode string, params map[string]string) error {
	var err error
	for i := 0; i < s.config.MaxRetries; i++ {
		err = s.send(phone, templateCode, params)
		if err == nil {
			return nil
		}
		s.logger.Warn("Failed to send SMS, retrying... (%d/%d)", i+1, s.config.MaxRetries)
		time.Sleep(time.Duration(s.config.RetryDelay) * time.Second)
	}
	return fmt.Errorf("failed to send SMS after %d retries: %v", s.config.MaxRetries, err)
}

// send 发送单条短信
func (s *SMS) send(phone, templateCode string, params map[string]string) error {
	// 构建请求参数
	values := url.Values{}
	values.Set("PhoneNumbers", phone)
	values.Set("SignName", s.config.SignName)
	values.Set("TemplateCode", templateCode)
	values.Set("TemplateParam", s.buildTemplateParam(params))

	// 构建请求
	req, err := http.NewRequest("POST", s.config.Endpoint, strings.NewReader(values.Encode()))
	if err != nil {
		return fmt.Errorf("failed to create request: %v", err)
	}

	// 设置请求头
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Authorization", s.generateAuthorization())

	// 发送请求
	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %v", err)
	}
	defer resp.Body.Close()

	// 读取响应
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %v", err)
	}

	// 解析响应
	var smsResp SMSResponse
	if err := json.Unmarshal(body, &smsResp); err != nil {
		return fmt.Errorf("failed to parse response: %v", err)
	}

	// 检查响应状态
	if smsResp.Code != "OK" {
		return fmt.Errorf("SMS API error: %s", smsResp.Message)
	}

	return nil
}

// buildTemplateParam 构建模板参数
func (s *SMS) buildTemplateParam(params map[string]string) string {
	if len(params) == 0 {
		return "{}"
	}
	jsonBytes, _ := json.Marshal(params)
	return string(jsonBytes)
}

// generateAuthorization 生成授权信息
func (s *SMS) generateAuthorization() string {
	// 这里实现具体的授权逻辑，根据不同的短信服务提供商可能有不同的实现
	return fmt.Sprintf("APPCODE %s", s.config.AccessKey)
}

// SendVerificationCode 发送验证码
func (s *SMS) SendVerificationCode(phone, code string) error {
	params := map[string]string{
		"code": code,
	}
	return s.SendSMS(phone, s.config.TemplateID, params)
}

// SendNotification 发送通知短信
func (s *SMS) SendNotification(phone, content string) error {
	params := map[string]string{
		"content": content,
	}
	return s.SendSMS(phone, s.config.TemplateID, params)
}

// SendLoginAlert 发送登录提醒
func (s *SMS) SendLoginAlert(phone, location, device string) error {
	params := map[string]string{
		"location": location,
		"device":   device,
	}
	return s.SendSMS(phone, s.config.TemplateID, params)
}

// SendPasswordReset 发送密码重置验证码
func (s *SMS) SendPasswordReset(phone, code string) error {
	params := map[string]string{
		"code": code,
	}
	return s.SendSMS(phone, s.config.TemplateID, params)
}

// SendBulkSMS 批量发送短信
func (s *SMS) SendBulkSMS(phones []string, templateCode string, params map[string]string) error {
	for _, phone := range phones {
		if err := s.SendSMS(phone, templateCode, params); err != nil {
			s.logger.Error("Failed to send SMS to %s: %v", phone, err)
			continue
		}
	}
	return nil
}

// ValidatePhoneNumber 验证手机号格式
func (s *SMS) ValidatePhoneNumber(phone string) bool {
	// 这里实现手机号验证逻辑，可以根据不同国家的手机号格式进行调整
	return len(phone) >= 11 && len(phone) <= 15
}

// GetSMSStatus 获取短信发送状态
func (s *SMS) GetSMSStatus(requestID string) (*SMSResponse, error) {
	// 构建请求参数
	values := url.Values{}
	values.Set("RequestId", requestID)

	// 构建请求
	req, err := http.NewRequest("GET", s.config.Endpoint+"/status", strings.NewReader(values.Encode()))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %v", err)
	}

	// 设置请求头
	req.Header.Set("Authorization", s.generateAuthorization())

	// 发送请求
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %v", err)
	}
	defer resp.Body.Close()

	// 读取响应
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %v", err)
	}

	// 解析响应
	var smsResp SMSResponse
	if err := json.Unmarshal(body, &smsResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %v", err)
	}

	return &smsResp, nil
} 