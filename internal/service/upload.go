package service

import (
	"fmt"
	"io"
	"mime/multipart"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
)

// UploadService 文件上传服务
type UploadService struct {
	uploadDir string
}

// NewUploadService 创建上传服务实例
func NewUploadService(uploadDir string) (*UploadService, error) {
	if err := os.MkdirAll(uploadDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create upload directory: %v", err)
	}
	return &UploadService{uploadDir: uploadDir}, nil
}

// UploadFile 上传文件
func (s *UploadService) UploadFile(file *multipart.FileHeader) (string, error) {
	// 生成唯一文件名
	ext := filepath.Ext(file.Filename)
	filename := fmt.Sprintf("%s%s", uuid.New().String(), ext)

	// 创建目标文件
	dst, err := os.Create(filepath.Join(s.uploadDir, filename))
	if err != nil {
		return "", fmt.Errorf("failed to create file: %v", err)
	}
	defer dst.Close()

	// 打开源文件
	src, err := file.Open()
	if err != nil {
		return "", fmt.Errorf("failed to open file: %v", err)
	}
	defer src.Close()

	// 复制文件内容
	if _, err = io.Copy(dst, src); err != nil {
		return "", fmt.Errorf("failed to copy file: %v", err)
	}

	// 返回文件URL
	return fmt.Sprintf("/uploads/%s", filename), nil
}

// DeleteFile 删除文件
func (s *UploadService) DeleteFile(url string) error {
	filename := filepath.Base(url)
	return os.Remove(filepath.Join(s.uploadDir, filename))
}

// GetFileURL 获取文件URL
func (s *UploadService) GetFileURL(filename string) string {
	return fmt.Sprintf("/uploads/%s", filename)
} 