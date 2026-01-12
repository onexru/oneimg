package models

type Buckets struct {
	Id       int            `json:"id" gorm:"type:integer;primaryKey;autoIncrement"`
	Name     string         `json:"name" gorm:"not null;unique"`                      // 存储名称，唯一
	Type     string         `json:"type" gorm:"not null"`                             // 存储类型
	Capacity uint64         `json:"capacity" gorm:"not null"`                         // 容量
	Config   map[string]any `json:"config" gorm:"type:text;not null;serializer:json"` // 配置
	Usage    uint64         `json:"usage" gorm:"not null"`                            // 已使用容量
}

// 定义每个bucket的存储类型

// S3 存储
type S3Bucket struct {
	S3Endpoint  string `json:"s3_endpoint"`
	S3AccessKey string `json:"s3_access_key"`
	S3SecretKey string `json:"s3_secret_key"`
	S3Bucket    string `json:"s3_bucket"`
}

// R2 兼容S3协议
type R2Bucket struct {
	R2Endpoint  string `json:"r2_endpoint"`
	R2AccessKey string `json:"r2_access_key"`
	R2SecretKey string `json:"r2_secret_key"`
	R2Bucket    string `json:"r2_bucket"`
}

// FTP 存储
type FTPBucket struct {
	FTPHost string `json:"ftp_host"`
	FTPUser string `json:"ftp_user"`
	FTPPass string `json:"ftp_pass"`
	FTPPort int    `json:"ftp_port"`
}

// WebDav 存储
type WebDavBucket struct {
	WebdavURL  string `json:"webdav_url"`
	WebdavUser string `json:"webdav_user"`
	WebdavPass string `json:"webdav_pass"`
}

// Telegram 存储
type TelegramBucket struct {
	TGBotToken  string `json:"tg_bot_token"`
	TGReceivers string `json:"tg_receivers"`
}
