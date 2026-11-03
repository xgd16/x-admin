package requestid

import (
	"context"
	"crypto/rand"
	"encoding/hex"
)

const Header = "X-Request-ID"

type contextKey struct{}

// New returns a 16-byte random request id.
func New() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "00000000000000000000000000000000"
	}
	return hex.EncodeToString(b[:])
}

func With(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, contextKey{}, id)
}

func FromContext(ctx context.Context) string {
	if v, ok := ctx.Value(contextKey{}).(string); ok {
		return v
	}
	return ""
}
