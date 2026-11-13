package middleware

import (
	"bytes"
	"encoding/json"
	"io"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/gin-gonic/gin"

	"x-admin/internal/model"
	"x-admin/internal/pkg/authn"
	"x-admin/internal/pkg/requestid"
)

// OperationRecorder 接收审计记录。实现方应当异步落库，不能阻塞请求。
type OperationRecorder interface {
	RecordOperation(log *model.OperationLog)
}

// Audit 捕获会修改数据的 JSON 请求，并交给异步记录器。
// 请求体在处理器读取时顺带复制，不会二次读取或改变请求语义。
func Audit(recorder OperationRecorder) gin.HandlerFunc {
	const maxBody = 4096
	return func(c *gin.Context) {
		if recorder == nil || !shouldAudit(c) {
			c.Next()
			return
		}
		// 最多保留 64KB 原始请求体，最终写入数据库时再截断到 4KB。
		capture := newBodyCapture(c.Request.Body, 64*1024)
		c.Request.Body = capture
		start := time.Now()
		c.Next()
		_ = capture.Close()

		principal, _ := authn.FromContext(c.Request.Context())
		log := &model.OperationLog{
			Method:    c.Request.Method,
			Path:      c.Request.URL.Path,
			IP:        c.ClientIP(),
			UserAgent: truncate(c.Request.UserAgent(), 255),
			Status:    c.Writer.Status(),
			LatencyMS: time.Since(start).Milliseconds(),
			RequestID: requestid.FromContext(c.Request.Context()),
			Body:      truncate(sanitizeBody(capture.Bytes()), maxBody),
			Error:     truncate(c.Errors.String(), 500),
		}
		if principal != nil {
			log.UserID = principal.UserID
			log.Username = principal.Username
		}
		recorder.RecordOperation(log)
	}
}

// shouldAudit 只审计写操作，并排除所有 API 面的认证接口，避免记录密码、验证码和令牌。
func shouldAudit(c *gin.Context) bool {
	switch c.Request.Method {
	case "POST", "PUT", "PATCH", "DELETE":
	default:
		return false
	}
	return !strings.Contains(c.Request.URL.Path, "/auth/")
}

// bodyCapture 在读取请求体的同时把内容复制到缓冲区，超过上限后停止复制。
type bodyCapture struct {
	rc     io.ReadCloser
	buf    bytes.Buffer
	limit  int
	sealed bool
}

func newBodyCapture(rc io.ReadCloser, limit int) *bodyCapture {
	return &bodyCapture{rc: rc, limit: limit}
}

func (b *bodyCapture) Read(p []byte) (int, error) {
	n, err := b.rc.Read(p)
	if n > 0 && !b.sealed {
		room := b.limit - b.buf.Len()
		if room <= 0 {
			b.sealed = true
		} else {
			if n < room {
				room = n
			}
			b.buf.Write(p[:room])
		}
	}
	return n, err
}

func (b *bodyCapture) Close() error { return b.rc.Close() }

func (b *bodyCapture) Bytes() []byte { return b.buf.Bytes() }

// sanitizeBody 把敏感字段替换为 *** 后重新序列化。
// 非 JSON 请求体不记录内容，避免把文件或二进制写进审计表。
func sanitizeBody(raw []byte) string {
	if len(bytes.TrimSpace(raw)) == 0 {
		return ""
	}
	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		return "[non-json body]"
	}
	mask(value)
	out, err := json.Marshal(value)
	if err != nil {
		return "[unserializable body]"
	}
	return string(out)
}

// mask 递归遍历 JSON 结构，替换敏感键的值。
func mask(value any) {
	switch v := value.(type) {
	case map[string]any:
		for key, child := range v {
			if isSensitive(key) {
				v[key] = "***"
				continue
			}
			mask(child)
		}
	case []any:
		for _, child := range v {
			mask(child)
		}
	}
}

// isSensitive 判断字段名是否包含密码、令牌、密钥或验证码等敏感信息。
func isSensitive(key string) bool {
	key = strings.ToLower(key)
	if key == "otp_code" {
		return true
	}
	return strings.Contains(key, "password") || strings.Contains(key, "token") ||
		strings.Contains(key, "secret") || strings.Contains(key, "captcha")
}

// truncate 按字节上限截断，并回退到合法 UTF-8 边界，避免写入 MySQL 时报编码错误。
func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	for n > 0 && !utf8.RuneStart(s[n]) {
		n--
	}
	return s[:n]
}
