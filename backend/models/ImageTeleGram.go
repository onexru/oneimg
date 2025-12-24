package models

type ImageTeleGram struct {
	Id                   int    `gorm:"type:integer;primaryKey;autoIncrement" json:"id"`
	TGFileId             string `gorm:"not null, default:''" json:"tg_file_id"`
	TGThumbnailFileId    string `gorm:"not null, default:''" json:"tg_thumbnail_file_id"`
	TGMessageId          int    `gorm:"not null, default:0" json:"tg_message_id"`
	TGThumbnailMessageId int    `gorm:"not null, default:0" json:"tg_thumbnail_message_id"`
	FileName             string `gorm:"not null, default:''" json:"file_name"`
}
