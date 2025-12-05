package models

import "time"

// 图片模型
type Image struct {
	Id        int       `json:"id" gorm:"primaryKey"`
	Url       string    `json:"url" gorm:"not null"`
	Thumbnail string    `json:"thumbnail"`
	FileName  string    `json:"filename" gorm:"not null"`
	FileSize  int64     `json:"file_size" gorm:"not null"`
	MimeType  string    `json:"mimeType"`
	Width     int       `json:"width"`
	Height    int       `json:"height"`
	Storage   string    `json:"storage" gorm:"default:default"`
	UserId    int       `json:"user_id" gorm:"not null;default:1"`
	CreatedAt time.Time `json:"created_at"`
}
