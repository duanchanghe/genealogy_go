package service

import (
	"fmt"
	"time"

	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// DBConfig 数据库配置
type DBConfig struct {
	Type         string        // 数据库类型：mysql, postgres, sqlite
	Host         string        // 主机地址
	Port         int          // 端口
	Username     string        // 用户名
	Password     string        // 密码
	Database     string        // 数据库名
	MaxIdleConns int          // 最大空闲连接数
	MaxOpenConns int          // 最大打开连接数
	MaxLifetime  time.Duration // 连接最大生命周期
	LogLevel     logger.LogLevel // 日志级别
}

// Database 数据库服务
type Database struct {
	db     *gorm.DB
	config *DBConfig
	logger *Logger
}

// NewDatabase 创建数据库服务实例
func NewDatabase(config *DBConfig, logger *Logger) (*Database, error) {
	db := &Database{
		config: config,
		logger: logger,
	}

	// 配置GORM日志
	gormLogger := logger.New(
		logger.Config{
			SlowThreshold:             time.Second,  // 慢SQL阈值
			LogLevel:                  config.LogLevel,
			IgnoreRecordNotFoundError: true,         // 忽略记录未找到错误
			Colorful:                  true,         // 彩色输出
		},
	)

	// 连接数据库
	var err error
	switch config.Type {
	case "mysql":
		dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=True&loc=Local",
			config.Username, config.Password, config.Host, config.Port, config.Database)
		db.db, err = gorm.Open(mysql.Open(dsn), &gorm.Config{Logger: gormLogger})
	case "postgres":
		dsn := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable",
			config.Host, config.Port, config.Username, config.Password, config.Database)
		db.db, err = gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: gormLogger})
	case "sqlite":
		db.db, err = gorm.Open(sqlite.Open(config.Database), &gorm.Config{Logger: gormLogger})
	default:
		return nil, fmt.Errorf("unsupported database type: %s", config.Type)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %v", err)
	}

	// 配置连接池
	sqlDB, err := db.db.DB()
	if err != nil {
		return nil, fmt.Errorf("failed to get database instance: %v", err)
	}

	sqlDB.SetMaxIdleConns(config.MaxIdleConns)
	sqlDB.SetMaxOpenConns(config.MaxOpenConns)
	sqlDB.SetConnMaxLifetime(config.MaxLifetime)

	return db, nil
}

// DB 获取数据库实例
func (d *Database) DB() *gorm.DB {
	return d.db
}

// Close 关闭数据库连接
func (d *Database) Close() error {
	sqlDB, err := d.db.DB()
	if err != nil {
		return fmt.Errorf("failed to get database instance: %v", err)
	}
	return sqlDB.Close()
}

// AutoMigrate 自动迁移数据库结构
func (d *Database) AutoMigrate(models ...interface{}) error {
	return d.db.AutoMigrate(models...)
}

// Transaction 执行事务
func (d *Database) Transaction(fc func(tx *gorm.DB) error) error {
	return d.db.Transaction(fc)
}

// Create 创建记录
func (d *Database) Create(value interface{}) error {
	return d.db.Create(value).Error
}

// Update 更新记录
func (d *Database) Update(value interface{}) error {
	return d.db.Save(value).Error
}

// Delete 删除记录
func (d *Database) Delete(value interface{}) error {
	return d.db.Delete(value).Error
}

// First 获取第一条记录
func (d *Database) First(dest interface{}, conds ...interface{}) error {
	return d.db.First(dest, conds...).Error
}

// Find 获取多条记录
func (d *Database) Find(dest interface{}, conds ...interface{}) error {
	return d.db.Find(dest, conds...).Error
}

// Count 获取记录数
func (d *Database) Count(count *int64, conds ...interface{}) error {
	return d.db.Model(conds[0]).Count(count).Error
}

// Where 条件查询
func (d *Database) Where(query interface{}, args ...interface{}) *gorm.DB {
	return d.db.Where(query, args...)
}

// Order 排序
func (d *Database) Order(value interface{}) *gorm.DB {
	return d.db.Order(value)
}

// Limit 限制返回数量
func (d *Database) Limit(limit int) *gorm.DB {
	return d.db.Limit(limit)
}

// Offset 偏移量
func (d *Database) Offset(offset int) *gorm.DB {
	return d.db.Offset(offset)
}

// Preload 预加载关联
func (d *Database) Preload(query string, args ...interface{}) *gorm.DB {
	return d.db.Preload(query, args...)
}

// Joins 连接查询
func (d *Database) Joins(query string, args ...interface{}) *gorm.DB {
	return d.db.Joins(query, args...)
}

// Group 分组查询
func (d *Database) Group(name string) *gorm.DB {
	return d.db.Group(name)
}

// Having 分组条件
func (d *Database) Having(query interface{}, args ...interface{}) *gorm.DB {
	return d.db.Having(query, args...)
}

// Scopes 作用域
func (d *Database) Scopes(funcs ...func(*gorm.DB) *gorm.DB) *gorm.DB {
	return d.db.Scopes(funcs...)
}

// Raw 原生SQL查询
func (d *Database) Raw(sql string, values ...interface{}) *gorm.DB {
	return d.db.Raw(sql, values...)
}

// Exec 执行原生SQL
func (d *Database) Exec(sql string, values ...interface{}) error {
	return d.db.Exec(sql, values...).Error
} 