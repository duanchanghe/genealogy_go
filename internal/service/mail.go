package service

import (
	"bytes"
	"fmt"
	"html/template"
	"mime/multipart"
	"net/smtp"
	"path/filepath"
	"strings"
)

// MailConfig 邮件配置
type MailConfig struct {
	Host     string   // SMTP服务器地址
	Port     int     // SMTP服务器端口
	Username string   // 用户名
	Password string   // 密码
	From     string   // 发件人
	FromName string   // 发件人名称
}

// MailAttachment 邮件附件
type MailAttachment struct {
	Filename string
	Path     string
}

// Mail 邮件服务
type Mail struct {
	config *MailConfig
	logger *Logger
}

// NewMail 创建邮件服务实例
func NewMail(config *MailConfig, logger *Logger) *Mail {
	return &Mail{
		config: config,
		logger: logger,
	}
}

// SendMail 发送邮件
func (m *Mail) SendMail(to []string, subject, body string, attachments []*MailAttachment) error {
	// 设置认证信息
	auth := smtp.PlainAuth("", m.config.Username, m.config.Password, m.config.Host)

	// 创建邮件内容
	var buffer bytes.Buffer
	buffer.WriteString(fmt.Sprintf("From: %s <%s>\r\n", m.config.FromName, m.config.From))
	buffer.WriteString(fmt.Sprintf("To: %s\r\n", strings.Join(to, ", ")))
	buffer.WriteString(fmt.Sprintf("Subject: %s\r\n", subject))

	// 生成边界
	boundary := "boundary123"
	buffer.WriteString(fmt.Sprintf("MIME-Version: 1.0\r\n"))
	buffer.WriteString(fmt.Sprintf("Content-Type: multipart/mixed; boundary=%s\r\n", boundary))
	buffer.WriteString("\r\n")

	// 添加文本内容
	buffer.WriteString(fmt.Sprintf("--%s\r\n", boundary))
	buffer.WriteString("Content-Type: text/html; charset=UTF-8\r\n")
	buffer.WriteString("\r\n")
	buffer.WriteString(body)
	buffer.WriteString("\r\n")

	// 添加附件
	for _, attachment := range attachments {
		buffer.WriteString(fmt.Sprintf("--%s\r\n", boundary))
		buffer.WriteString(fmt.Sprintf("Content-Type: application/octet-stream\r\n"))
		buffer.WriteString(fmt.Sprintf("Content-Disposition: attachment; filename=\"%s\"\r\n", attachment.Filename))
		buffer.WriteString("Content-Transfer-Encoding: base64\r\n")
		buffer.WriteString("\r\n")

		// 读取附件内容
		content, err := os.ReadFile(attachment.Path)
		if err != nil {
			return fmt.Errorf("failed to read attachment: %v", err)
		}

		// 添加附件内容
		buffer.Write(content)
		buffer.WriteString("\r\n")
	}

	buffer.WriteString(fmt.Sprintf("--%s--\r\n", boundary))

	// 发送邮件
	addr := fmt.Sprintf("%s:%d", m.config.Host, m.config.Port)
	if err := smtp.SendMail(addr, auth, m.config.From, to, buffer.Bytes()); err != nil {
		return fmt.Errorf("failed to send mail: %v", err)
	}

	return nil
}

// SendVerificationMail 发送验证邮件
func (m *Mail) SendVerificationMail(to string, code string) error {
	subject := "验证您的邮箱"
	body := fmt.Sprintf(`
		<html>
		<body>
			<h2>邮箱验证</h2>
			<p>您的验证码是：<strong>%s</strong></p>
			<p>此验证码将在30分钟后过期。</p>
			<p>如果这不是您的操作，请忽略此邮件。</p>
		</body>
		</html>
	`, code)

	return m.SendMail([]string{to}, subject, body, nil)
}

// SendWelcomeMail 发送欢迎邮件
func (m *Mail) SendWelcomeMail(to string, username string) error {
	subject := "欢迎加入我们"
	body := fmt.Sprintf(`
		<html>
		<body>
			<h2>欢迎 %s</h2>
			<p>感谢您注册我们的服务！</p>
			<p>如果您有任何问题，请随时联系我们。</p>
		</body>
		</html>
	`, username)

	return m.SendMail([]string{to}, subject, body, nil)
}

// SendPasswordResetMail 发送密码重置邮件
func (m *Mail) SendPasswordResetMail(to string, resetLink string) error {
	subject := "密码重置"
	body := fmt.Sprintf(`
		<html>
		<body>
			<h2>密码重置</h2>
			<p>请点击以下链接重置您的密码：</p>
			<p><a href="%s">%s</a></p>
			<p>此链接将在1小时后过期。</p>
			<p>如果这不是您的操作，请忽略此邮件。</p>
		</body>
		</html>
	`, resetLink, resetLink)

	return m.SendMail([]string{to}, subject, body, nil)
}

// SendNotificationMail 发送通知邮件
func (m *Mail) SendNotificationMail(to []string, subject, content string) error {
	body := fmt.Sprintf(`
		<html>
		<body>
			<h2>%s</h2>
			<p>%s</p>
		</body>
		</html>
	`, subject, content)

	return m.SendMail(to, subject, body, nil)
}

// SendMailWithTemplate 使用模板发送邮件
func (m *Mail) SendMailWithTemplate(to []string, subject, templatePath string, data interface{}) error {
	// 读取模板文件
	tmpl, err := template.ParseFiles(templatePath)
	if err != nil {
		return fmt.Errorf("failed to parse template: %v", err)
	}

	// 渲染模板
	var body bytes.Buffer
	if err := tmpl.Execute(&body, data); err != nil {
		return fmt.Errorf("failed to execute template: %v", err)
	}

	return m.SendMail(to, subject, body.String(), nil)
}

// SendMailWithAttachments 发送带附件的邮件
func (m *Mail) SendMailWithAttachments(to []string, subject, body string, attachmentPaths []string) error {
	attachments := make([]*MailAttachment, len(attachmentPaths))
	for i, path := range attachmentPaths {
		attachments[i] = &MailAttachment{
			Filename: filepath.Base(path),
			Path:     path,
		}
	}

	return m.SendMail(to, subject, body, attachments)
}

// SendBulkMail 批量发送邮件
func (m *Mail) SendBulkMail(recipients []string, subject, body string) error {
	// 分批发送，每批最多50个收件人
	batchSize := 50
	for i := 0; i < len(recipients); i += batchSize {
		end := i + batchSize
		if end > len(recipients) {
			end = len(recipients)
		}

		if err := m.SendMail(recipients[i:end], subject, body, nil); err != nil {
			return fmt.Errorf("failed to send bulk mail: %v", err)
		}
	}

	return nil
} 