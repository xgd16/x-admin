package model

import "time"

// Passkey 是一条绑定到用户的 WebAuthn 凭据。
// 一个用户可以注册多台设备，每一行代表一个独立的认证器。
type Passkey struct {
	Base
	UserID       uint64     `gorm:"not null;index" json:"-"`
	CredentialID string     `gorm:"size:512;uniqueIndex;not null" json:"-"` // base64url 编码的凭据 ID
	Name         string     `gorm:"size:64;not null" json:"name"`           // 设备名称，便于用户区分
	Credential   string     `gorm:"type:text;not null" json:"-"`            // 序列化后的 webauthn.Credential
	LastUsedAt   *time.Time `json:"last_used_at"`
}

// TableName 指定通行密钥表名。
func (Passkey) TableName() string { return "sys_passkeys" }
