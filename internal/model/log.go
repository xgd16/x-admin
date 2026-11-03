package model

// OperationLog 是写操作的审计记录，由中间件异步落库。
type OperationLog struct {
	Base
	UserID    uint64 `gorm:"index" json:"user_id"`
	Username  string `gorm:"size:64;index" json:"username"`
	Method    string `gorm:"size:16" json:"method"`
	Path      string `gorm:"size:255" json:"path"`
	IP        string `gorm:"size:64" json:"ip"`
	UserAgent string `gorm:"size:255" json:"user_agent"`
	Status    int    `json:"status"`     // HTTP 状态码
	LatencyMS int64  `json:"latency_ms"` // 处理耗时（毫秒）
	RequestID string `gorm:"size:64;index" json:"request_id"`
	Body      string `gorm:"type:text" json:"body"` // 已脱敏的请求体
	Error     string `gorm:"size:500" json:"error"`
}

// TableName 指定操作日志表名。
func (OperationLog) TableName() string { return "sys_operation_logs" }

// LoginLog 是登录审计记录，覆盖密码、二次验证和 Passkey 登录。
type LoginLog struct {
	Base
	UserID    uint64 `gorm:"index" json:"user_id"`
	Username  string `gorm:"size:64;index" json:"username"`
	IP        string `gorm:"size:64" json:"ip"`
	UserAgent string `gorm:"size:255" json:"user_agent"`
	Status    int    `json:"status"` // HTTP 语义的状态码
	Message   string `gorm:"size:255" json:"message"`
}

// TableName 指定登录日志表名。
func (LoginLog) TableName() string { return "sys_login_logs" }
