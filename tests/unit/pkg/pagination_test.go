package pkg_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"x-admin/internal/pkg/pagination"
)

func TestPaginationNormalize(t *testing.T) {
	q := pagination.Query{}
	q.Normalize()
	assert.Equal(t, 1, q.Page)
	assert.Equal(t, 20, q.PageSize)
	assert.Equal(t, 0, q.Offset())

	q = pagination.Query{Page: 3, PageSize: 10}
	q.Normalize()
	assert.Equal(t, 3, q.Page)
	assert.Equal(t, 10, q.PageSize)
	assert.Equal(t, 20, q.Offset())

	q = pagination.Query{Page: -2, PageSize: 1000}
	q.Normalize()
	assert.Equal(t, 1, q.Page)
	assert.Equal(t, pagination.MaxPageSize, q.PageSize)

	assert.Equal(t, 0, pagination.Query{Page: 0, PageSize: 20}.Offset())
}
