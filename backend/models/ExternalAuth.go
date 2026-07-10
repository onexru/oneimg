package models

import "time"

// ExternalAuthFlow 保存短时、一次性的外部登录事务。数据库中只保存 state 的哈希。
type ExternalAuthFlow struct {
	StateHash    string    `gorm:"size:64;primaryKey;column:state_hash" json:"-"`
	Provider     string    `gorm:"size:16;not null;index;column:provider" json:"-"`
	Issuer       string    `gorm:"size:2048;column:issuer" json:"-"`
	ClientID     string    `gorm:"size:512;column:client_id" json:"-"`
	Nonce        string    `gorm:"size:128;column:nonce" json:"-"`
	CodeVerifier string    `gorm:"size:128;column:code_verifier" json:"-"`
	CallbackURL  string    `gorm:"size:2048;column:callback_url" json:"-"`
	ServiceURL   string    `gorm:"size:2048;column:service_url" json:"-"`
	ExpiresAt    time.Time `gorm:"not null;index;column:expires_at" json:"-"`
	CreatedAt    time.Time `gorm:"autoCreateTime;column:created_at" json:"-"`
}

func (ExternalAuthFlow) TableName() string { return "external_auth_flows" }

// ExternalIdentity 将外部提供方的稳定主体标识绑定到本地用户。
type ExternalIdentity struct {
	ID          int       `gorm:"type:integer;primaryKey;autoIncrement" json:"id"`
	UserID      int       `gorm:"not null;index;column:user_id" json:"user_id"`
	Provider    string    `gorm:"size:16;not null;column:provider" json:"provider"`
	Issuer      string    `gorm:"size:2048;not null;column:issuer" json:"issuer"`
	Subject     string    `gorm:"size:2048;not null;column:subject" json:"subject"`
	IdentityKey string    `gorm:"size:64;not null;uniqueIndex;column:identity_key" json:"-"`
	Disabled    bool      `gorm:"not null;default:false;index;column:disabled" json:"disabled"`
	Email       string    `gorm:"size:320;column:email" json:"email"`
	DisplayName string    `gorm:"size:255;column:display_name" json:"display_name"`
	LastLoginAt time.Time `gorm:"column:last_login_at" json:"last_login_at"`
	CreatedAt   time.Time `gorm:"autoCreateTime;column:created_at" json:"created_at"`
	UpdatedAt   time.Time `gorm:"autoUpdateTime;column:updated_at" json:"updated_at"`
}

func (ExternalIdentity) TableName() string { return "external_identities" }
