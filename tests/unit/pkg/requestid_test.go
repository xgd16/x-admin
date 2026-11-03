package pkg_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"x-admin/internal/pkg/requestid"
)

func TestRequestID(t *testing.T) {
	id := requestid.New()
	assert.Len(t, id, 32)

	ctx := requestid.With(context.Background(), "abc")
	assert.Equal(t, "abc", requestid.FromContext(ctx))
	assert.Equal(t, "", requestid.FromContext(context.Background()))
}
