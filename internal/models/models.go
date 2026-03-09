package models

import (
	"time"

	"gorm.io/gorm"
)

type User struct {
	ID           uint           `gorm:"primaryKey" json:"id"`
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
	DeletedAt    gorm.DeletedAt `gorm:"index" json:"-"`
	Phone        string         `gorm:"uniqueIndex" json:"phone"`         // 手机号，可为空（微信登录时）
	PasswordHash string         `gorm:"not null" json:"-"`                // 密码hash，微信登录时为空
	Nickname     string         `json:"nickname"`
	WechatOpenID string         `gorm:"uniqueIndex" json:"wechat_openid"`  // 微信openid
	Avatar       string         `json:"avatar"`                          // 头像
}

type Customer struct {
	ID        uint           `gorm:"primaryKey" json:"id"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
	UserID      uint       `gorm:"index;not null" json:"user_id"`
	Name        string     `gorm:"not null" json:"name"`
	Industry    string     `json:"industry"`
	Phone       string     `json:"phone"`
	Temp        string     `gorm:"default:温" json:"temp"` // 热 | 温 | 冷
	WinRate     int        `gorm:"default:0" json:"win_rate"` // 成单概率 0-100
	LastContact *time.Time `json:"last_contact"`
	Notes       string     `json:"notes"` // 简短备注，冗余字段
}

type FollowUpRecord struct {
	ID        uint           `gorm:"primaryKey" json:"id"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
	CustomerID  uint      `gorm:"index;not null" json:"customer_id"`
	UserID      uint      `gorm:"index;not null" json:"user_id"`
	Note        string    `gorm:"not null" json:"note"`
	ContactedAt time.Time `json:"contacted_at"`
}

type WeChatMessage struct {
	ID         uint           `gorm:"primaryKey" json:"id"`
	CreatedAt  time.Time      `json:"created_at"`
	UpdatedAt  time.Time      `json:"updated_at"`
	DeletedAt  gorm.DeletedAt `gorm:"index" json:"-"`
	CustomerID uint           `gorm:"index;not null" json:"customer_id"`
	UserID     uint           `gorm:"index;not null" json:"user_id"`
	Content    string         `gorm:"not null" json:"content"`
	MessageAt time.Time      `json:"message_at"`
}

// Response types
type ApiResponse struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data"`
}

func SuccessResponse(data interface{}) ApiResponse {
	return ApiResponse{
		Code:    0,
		Message: "success",
		Data:    data,
	}
}

func ErrorResponse(code int, message string) ApiResponse {
	return ApiResponse{
		Code:    code,
		Message: message,
		Data:    nil,
	}
}