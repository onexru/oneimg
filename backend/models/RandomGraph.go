package models

type RandomGraph struct {
	ID      int   `json:"id"  gorm:"type:integer;primaryKey;autoIncrement"`
	UserIds []int `json:"user_ids" gorm:"serializer:json"` // 随机图用户ID列表
	TagIds  []int `json:"tag_ids" gorm:"serializer:json"`  // 随机图标签ID列表
}
