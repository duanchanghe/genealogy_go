package repository

import (
	"fmt"
	"log"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"genealogy_go/internal/model"
)

// DB 数据库连接实例
type DB struct {
	*gorm.DB
}

// InitDB 初始化数据库连接
func InitDB(dsn string) (*DB, error) {
	var err error
	
	// 配置GORM
	config := &gorm.Config{
		Logger: logger.Default.LogMode(logger.Info),
	}

	// 连接数据库
	gormDB, err := gorm.Open(postgres.Open(dsn), config)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %v", err)
	}

	// 自动迁移数据库表
	err = gormDB.AutoMigrate(
		&model.Member{},
		&model.Photo{},
		&model.Event{},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to migrate database: %v", err)
	}

	log.Println("Database connected successfully")
	return &DB{gormDB}, nil
}

// GetDB 获取数据库连接
func GetDB() *gorm.DB {
	return DB.DB
} 