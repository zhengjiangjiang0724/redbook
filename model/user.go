package model

import "time"

// User 用户模型
type User struct {
	ID           uint64    `gorm:"primarykey" json:"id"`
	Mobile       string    `gorm:"unique;not null;size:11" json:"mobile"`
	Username     string    `gorm:"not null;size:50" json:"username"`
	Password     string    `gorm:"not null;size:100" json:"password"`
	Nickname     string    `gorm:"not null;size:100" json:"nickname"`
	PasswordHash string    `gorm:"not null;size:255" json:"-"` // 忽略JSON序列化
	AvatarURL    string    `gorm:"size:255" json:"avatar_url"`
	Bio          string    `gorm:"type:text" json:"bio"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}
