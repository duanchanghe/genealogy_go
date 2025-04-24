package service

import (
	"crypto/md5"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// FileConfig 文件配置
type FileConfig struct {
	UploadDir    string   // 上传目录
	MaxFileSize  int64    // 最大文件大小（字节）
	AllowedTypes []string // 允许的文件类型
	MaxFiles     int      // 每个用户最大文件数
}

// FileInfo 文件信息
type FileInfo struct {
	ID        uint      `json:"id" gorm:"primaryKey"`
	UserID    uint      `json:"user_id"`
	Filename  string    `json:"filename"`
	Path      string    `json:"path"`
	Size      int64     `json:"size"`
	Type      string    `json:"type"`
	Hash      string    `json:"hash"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// File 文件服务
type File struct {
	config *FileConfig
	db     *Database
	logger *Logger
}

// NewFile 创建文件服务实例
func NewFile(config *FileConfig, db *Database, logger *Logger) (*File, error) {
	// 确保上传目录存在
	if err := os.MkdirAll(config.UploadDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create upload directory: %v", err)
	}

	return &File{
		config: config,
		db:     db,
		logger: logger,
	}, nil
}

// Upload 上传文件
func (f *File) Upload(userID uint, file *multipart.FileHeader) (*FileInfo, error) {
	// 检查文件大小
	if file.Size > f.config.MaxFileSize {
		return nil, fmt.Errorf("file size exceeds maximum allowed size of %d bytes", f.config.MaxFileSize)
	}

	// 检查文件类型
	ext := strings.ToLower(filepath.Ext(file.Filename))
	allowed := false
	for _, t := range f.config.AllowedTypes {
		if t == ext {
			allowed = true
			break
		}
	}
	if !allowed {
		return nil, fmt.Errorf("file type %s is not allowed", ext)
	}

	// 检查用户文件数
	var count int64
	if err := f.db.Model(&FileInfo{}).Where("user_id = ?", userID).Count(&count).Error; err != nil {
		return nil, err
	}
	if count >= int64(f.config.MaxFiles) {
		return nil, fmt.Errorf("maximum number of files (%d) reached", f.config.MaxFiles)
	}

	// 计算文件哈希
	src, err := file.Open()
	if err != nil {
		return nil, err
	}
	defer src.Close()

	hash := md5.New()
	if _, err := io.Copy(hash, src); err != nil {
		return nil, err
	}
	fileHash := hex.EncodeToString(hash.Sum(nil))

	// 生成存储路径
	storagePath := filepath.Join(f.config.UploadDir, fmt.Sprintf("%d_%s%s", userID, fileHash, ext))

	// 保存文件
	if err := f.saveFile(file, storagePath); err != nil {
		return nil, err
	}

	// 创建文件记录
	fileInfo := &FileInfo{
		UserID:    userID,
		Filename:  file.Filename,
		Path:      storagePath,
		Size:      file.Size,
		Type:      ext,
		Hash:      fileHash,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	if err := f.db.Create(fileInfo).Error; err != nil {
		// 删除已上传的文件
		os.Remove(storagePath)
		return nil, err
	}

	return fileInfo, nil
}

// Download 下载文件
func (f *File) Download(fileID uint, userID uint) (*FileInfo, io.ReadCloser, error) {
	// 查询文件信息
	var fileInfo FileInfo
	if err := f.db.First(&fileInfo, fileID).Error; err != nil {
		return nil, nil, err
	}

	// 检查权限
	if fileInfo.UserID != userID {
		return nil, nil, errors.New("permission denied")
	}

	// 打开文件
	file, err := os.Open(fileInfo.Path)
	if err != nil {
		return nil, nil, err
	}

	return &fileInfo, file, nil
}

// Delete 删除文件
func (f *File) Delete(fileID uint, userID uint) error {
	// 查询文件信息
	var fileInfo FileInfo
	if err := f.db.First(&fileInfo, fileID).Error; err != nil {
		return err
	}

	// 检查权限
	if fileInfo.UserID != userID {
		return errors.New("permission denied")
	}

	// 删除文件
	if err := os.Remove(fileInfo.Path); err != nil {
		return err
	}

	// 删除记录
	return f.db.Delete(&fileInfo).Error
}

// List 列出用户的文件
func (f *File) List(userID uint, page, pageSize int) ([]*FileInfo, int64, error) {
	var files []*FileInfo
	var total int64

	// 获取总数
	if err := f.db.Model(&FileInfo{}).Where("user_id = ?", userID).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// 获取分页数据
	offset := (page - 1) * pageSize
	if err := f.db.Where("user_id = ?", userID).
		Order("created_at DESC").
		Offset(offset).
		Limit(pageSize).
		Find(&files).Error; err != nil {
		return nil, 0, err
	}

	return files, total, nil
}

// GetFileInfo 获取文件信息
func (f *File) GetFileInfo(fileID uint) (*FileInfo, error) {
	var fileInfo FileInfo
	if err := f.db.First(&fileInfo, fileID).Error; err != nil {
		return nil, err
	}
	return &fileInfo, nil
}

// saveFile 保存文件到磁盘
func (f *File) saveFile(file *multipart.FileHeader, dst string) error {
	src, err := file.Open()
	if err != nil {
		return err
	}
	defer src.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, src)
	return err
}

// GetFilePath 获取文件路径
func (f *File) GetFilePath(fileID uint) (string, error) {
	var fileInfo FileInfo
	if err := f.db.First(&fileInfo, fileID).Error; err != nil {
		return "", err
	}
	return fileInfo.Path, nil
}

// IsFileExists 检查文件是否存在
func (f *File) IsFileExists(fileID uint) bool {
	var count int64
	f.db.Model(&FileInfo{}).Where("id = ?", fileID).Count(&count)
	return count > 0
}

// GetFileSize 获取文件大小
func (f *File) GetFileSize(fileID uint) (int64, error) {
	var fileInfo FileInfo
	if err := f.db.First(&fileInfo, fileID).Error; err != nil {
		return 0, err
	}
	return fileInfo.Size, nil
}

// GetFileType 获取文件类型
func (f *File) GetFileType(fileID uint) (string, error) {
	var fileInfo FileInfo
	if err := f.db.First(&fileInfo, fileID).Error; err != nil {
		return "", err
	}
	return fileInfo.Type, nil
}

// GetFileHash 获取文件哈希
func (f *File) GetFileHash(fileID uint) (string, error) {
	var fileInfo FileInfo
	if err := f.db.First(&fileInfo, fileID).Error; err != nil {
		return "", err
	}
	return fileInfo.Hash, nil
} 