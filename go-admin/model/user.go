package model

import "time"

// User 用户结构体（OOP 类）
type User struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Username  string    `gorm:"unique;not null" json:"username" binding:"required,min=2,max=20"`
	Password  string    `gorm:"not null" json:"-" binding:"required,min=6,max=20"` // json:"-" 隐藏密码
	Status    int       `json:"status"`                                            // 1=正常 2=禁用
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
