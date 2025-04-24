package model

import (
	"time"

	"gorm.io/gorm"
)

// Member 家族成员模型
type Member struct {
	gorm.Model
	Name        string    `gorm:"size:100;not null" json:"name"`
	Gender      string    `gorm:"size:10;not null" json:"gender"`
	BirthDate   time.Time `json:"birth_date"`
	DeathDate   *time.Time `json:"death_date,omitempty"`
	BirthPlace  string    `gorm:"size:200" json:"birth_place"`
	CurrentPlace string   `gorm:"size:200" json:"current_place"`
	Education   string    `gorm:"size:100" json:"education"`
	Occupation  string    `gorm:"size:100" json:"occupation"`
	Description string    `gorm:"type:text" json:"description"`
	
	// 关系字段
	FatherID    *uint     `json:"father_id"`
	Father      *Member   `gorm:"foreignKey:FatherID" json:"father,omitempty"`
	MotherID    *uint     `json:"mother_id"`
	Mother      *Member   `gorm:"foreignKey:MotherID" json:"mother,omitempty"`
	SpouseID    *uint     `json:"spouse_id"`
	Spouse      *Member   `gorm:"foreignKey:SpouseID" json:"spouse,omitempty"`
	
	// 其他信息
	Photos      []Photo   `gorm:"foreignKey:MemberID" json:"photos,omitempty"`
	Events      []Event   `gorm:"foreignKey:MemberID" json:"events,omitempty"`
}

// Photo 照片模型
type Photo struct {
	gorm.Model
	MemberID    uint      `json:"member_id"`
	URL         string    `gorm:"size:500;not null" json:"url"`
	Description string    `gorm:"size:200" json:"description"`
	TakeDate    time.Time `json:"take_date"`
}

// Event 事件模型
type Event struct {
	gorm.Model
	MemberID    uint      `json:"member_id"`
	Title       string    `gorm:"size:200;not null" json:"title"`
	Description string    `gorm:"type:text" json:"description"`
	EventDate   time.Time `json:"event_date"`
	EventType   string    `gorm:"size:50" json:"event_type"` // 如：出生、结婚、去世等
	Location    string    `gorm:"size:200" json:"location"`
} 