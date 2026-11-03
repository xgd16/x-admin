package model

type OperationLog struct {
	Base
	UserID    uint64 `gorm:"index" json:"user_id"`
	Username  string `gorm:"size:64;index" json:"username"`
	Method    string `gorm:"size:16" json:"method"`
	Path      string `gorm:"size:255" json:"path"`
	IP        string `gorm:"size:64" json:"ip"`
	UserAgent string `gorm:"size:255" json:"user_agent"`
	Status    int    `json:"status"`
	LatencyMS int64  `json:"latency_ms"`
	RequestID string `gorm:"size:64;index" json:"request_id"`
	Body      string `gorm:"type:text" json:"body"`
	Error     string `gorm:"size:500" json:"error"`
}

func (OperationLog) TableName() string { return "sys_operation_logs" }

type LoginLog struct {
	Base
	UserID    uint64 `gorm:"index" json:"user_id"`
	Username  string `gorm:"size:64;index" json:"username"`
	IP        string `gorm:"size:64" json:"ip"`
	UserAgent string `gorm:"size:255" json:"user_agent"`
	Status    int    `json:"status"`
	Message   string `gorm:"size:255" json:"message"`
}

func (LoginLog) TableName() string { return "sys_login_logs" }
