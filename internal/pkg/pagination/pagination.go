// Package pagination 定义列表接口共用的分页参数与边界处理。
package pagination

// 分页默认值与上限，避免客户端请求过大的页。
const (
	DefaultPage     = 1
	DefaultPageSize = 20
	MaxPageSize     = 200
)

// Query 由列表接口内嵌，保证所有模块的分页行为一致。
type Query struct {
	Page     int    `form:"page" json:"page"`
	PageSize int    `form:"page_size" json:"page_size"`
	Keyword  string `form:"keyword" json:"keyword"`
}

// Normalize 修正非法分页参数：页码至少为 1，页大小限制在 1-200。
func (q *Query) Normalize() {
	if q.Page < 1 {
		q.Page = DefaultPage
	}
	if q.PageSize < 1 {
		q.PageSize = DefaultPageSize
	}
	if q.PageSize > MaxPageSize {
		q.PageSize = MaxPageSize
	}
}

// Offset 返回 SQL 的 OFFSET 值。
func (q Query) Offset() int {
	if q.Page < 1 {
		return 0
	}
	return (q.Page - 1) * q.PageSize
}
