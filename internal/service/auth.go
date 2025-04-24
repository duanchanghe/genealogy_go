package service

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v4"
	"golang.org/x/crypto/bcrypt"
)

// AuthConfig 认证配置
type AuthConfig struct {
	SecretKey     string        // JWT密钥
	TokenDuration time.Duration // Token有效期
	SaltLength    int          // 密码盐值长度
}

// Claims JWT声明
type Claims struct {
	UserID   uint     `json:"user_id"`
	Username string   `json:"username"`
	Roles    []string `json:"roles"`
	jwt.RegisteredClaims
}

// Auth 认证服务
type Auth struct {
	config *AuthConfig
	db     *Database
	logger *Logger
}

// NewAuth 创建认证服务实例
func NewAuth(config *AuthConfig, db *Database, logger *Logger) *Auth {
	return &Auth{
		config: config,
		db:     db,
		logger: logger,
	}
}

// GenerateToken 生成JWT令牌
func (a *Auth) GenerateToken(userID uint, username string, roles []string) (string, error) {
	// 创建声明
	claims := Claims{
		UserID:   userID,
		Username: username,
		Roles:    roles,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(a.config.TokenDuration)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now()),
		},
	}

	// 创建令牌
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(a.config.SecretKey))
}

// ValidateToken 验证JWT令牌
func (a *Auth) ValidateToken(tokenString string) (*Claims, error) {
	// 解析令牌
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(a.config.SecretKey), nil
	})

	if err != nil {
		return nil, err
	}

	// 获取声明
	if claims, ok := token.Claims.(*Claims); ok && token.Valid {
		return claims, nil
	}

	return nil, errors.New("invalid token")
}

// HashPassword 哈希密码
func (a *Auth) HashPassword(password string) (string, error) {
	// 生成盐值
	salt := make([]byte, a.config.SaltLength)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}

	// 哈希密码
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}

	// 组合盐值和哈希
	return base64.StdEncoding.EncodeToString(append(salt, hash...)), nil
}

// VerifyPassword 验证密码
func (a *Auth) VerifyPassword(hashedPassword, password string) error {
	// 解码哈希密码
	decoded, err := base64.StdEncoding.DecodeString(hashedPassword)
	if err != nil {
		return err
	}

	// 验证密码
	return bcrypt.CompareHashAndPassword(decoded[a.config.SaltLength:], []byte(password))
}

// Login 用户登录
func (a *Auth) Login(username, password string) (string, error) {
	// 查询用户
	var user struct {
		ID       uint
		Username string
		Password string
		Roles    []string
	}

	err := a.db.Where("username = ?", username).First(&user).Error
	if err != nil {
		return "", errors.New("invalid username or password")
	}

	// 验证密码
	if err := a.VerifyPassword(user.Password, password); err != nil {
		return "", errors.New("invalid username or password")
	}

	// 生成令牌
	return a.GenerateToken(user.ID, user.Username, user.Roles)
}

// Register 用户注册
func (a *Auth) Register(username, password string, roles []string) error {
	// 检查用户名是否已存在
	var count int64
	if err := a.db.Model(&struct{}{}).Where("username = ?", username).Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return errors.New("username already exists")
	}

	// 哈希密码
	hashedPassword, err := a.HashPassword(password)
	if err != nil {
		return err
	}

	// 创建用户
	user := struct {
		Username string
		Password string
		Roles    []string
	}{
		Username: username,
		Password: hashedPassword,
		Roles:    roles,
	}

	return a.db.Create(&user).Error
}

// ChangePassword 修改密码
func (a *Auth) ChangePassword(userID uint, oldPassword, newPassword string) error {
	// 查询用户
	var user struct {
		Password string
	}

	err := a.db.Model(&struct{}{}).Where("id = ?", userID).Select("password").First(&user).Error
	if err != nil {
		return err
	}

	// 验证旧密码
	if err := a.VerifyPassword(user.Password, oldPassword); err != nil {
		return errors.New("invalid old password")
	}

	// 哈希新密码
	hashedPassword, err := a.HashPassword(newPassword)
	if err != nil {
		return err
	}

	// 更新密码
	return a.db.Model(&struct{}{}).Where("id = ?", userID).Update("password", hashedPassword).Error
}

// HasRole 检查用户是否具有指定角色
func (a *Auth) HasRole(claims *Claims, role string) bool {
	for _, r := range claims.Roles {
		if r == role {
			return true
		}
	}
	return false
}

// HasAnyRole 检查用户是否具有任意指定角色
func (a *Auth) HasAnyRole(claims *Claims, roles ...string) bool {
	for _, role := range roles {
		if a.HasRole(claims, role) {
			return true
		}
	}
	return false
}

// HasAllRoles 检查用户是否具有所有指定角色
func (a *Auth) HasAllRoles(claims *Claims, roles ...string) bool {
	for _, role := range roles {
		if !a.HasRole(claims, role) {
			return false
		}
	}
	return true
}

// RefreshToken 刷新令牌
func (a *Auth) RefreshToken(tokenString string) (string, error) {
	// 验证旧令牌
	claims, err := a.ValidateToken(tokenString)
	if err != nil {
		return "", err
	}

	// 生成新令牌
	return a.GenerateToken(claims.UserID, claims.Username, claims.Roles)
}

// RevokeToken 撤销令牌
func (a *Auth) RevokeToken(tokenString string) error {
	// TODO: 实现令牌撤销逻辑，例如将令牌加入黑名单
	return nil
} 