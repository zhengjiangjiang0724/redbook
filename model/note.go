package model

import "time"

// Note 笔记模型
type Note struct {
	ID            uint64    `gorm:"primarykey" json:"id"`
	UserID        uint64    `gorm:"not null" json:"user_id"`
	Title         string    `gorm:"not null;size:100" json:"title"`
	Content       string    `gorm:"type:text" json:"content"`
	CoverURL      string    `gorm:"size:255" json:"cover_url"`
	Tags          string    `gorm:"size:255" json:"tags"`    // 逗号分隔
	Status        int       `gorm:"default:1" json:"status"` // 1-正常, 2-审核中, 3-禁用
	ViewsCount    int       `gorm:"default:0" json:"views_count"`
	LikesCount    int       `gorm:"default:0" json:"likes_count"`
	CommentsCount int       `gorm:"default:0" json:"comments_count"`
	CreatedAt     time.Time `json:"created_at"`
	User          User      `gorm:"foreignKey:UserID" json:"user,omitempty"` // 关联用户
}
