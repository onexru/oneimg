package models

type ImageToTags struct {
	ImageId int `json:"image_id" gorm:"primaryKey"`
	TagId   int `json:"tag_id" gorm:"primaryKey"`
}
