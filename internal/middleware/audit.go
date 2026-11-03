package middleware

import (
	"bytes"
	"encoding/json"
	"io"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"x-admin/internal/model"
	"x-admin/internal/pkg/authn"
	"x-admin/internal/pkg/requestid"
)

type OperationRecorder interface {
	RecordOperation(log *model.OperationLog)
}

// Audit captures mutating JSON requests and hands them to the async recorder.
// Bodies are captured while the handler reads them, never re-read.
func Audit(recorder OperationRecorder) gin.HandlerFunc {
	const maxBody = 4096
	return func(c *gin.Context) {
		if recorder == nil || !shouldAudit(c) {
			c.Next()
			return
		}
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

func shouldAudit(c *gin.Context) bool {
	switch c.Request.Method {
	case "POST", "PUT", "PATCH", "DELETE":
	default:
		return false
	}
	path := c.Request.URL.Path
	return !strings.HasPrefix(path, "/admin/v1/auth/")
}

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

func isSensitive(key string) bool {
	key = strings.ToLower(key)
	return strings.Contains(key, "password") || strings.Contains(key, "token") ||
		strings.Contains(key, "secret") || strings.Contains(key, "captcha")
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}
