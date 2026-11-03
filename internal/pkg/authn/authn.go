// Package authn 定义请求上下文中的认证身份。
package authn

import "context"

// Principal 是随请求传递的已认证身份。
type Principal struct {
	UserID      uint64
	Username    string
	Nickname    string
	Super       bool                // 是否超级管理员
	Permissions map[string]struct{} // 权限码集合
}

// Has 判断是否拥有指定权限；超级管理员对任何权限都返回 true。
func (p *Principal) Has(permission string) bool {
	if p == nil {
		return false
	}
	if p.Super {
		return true
	}
	_, ok := p.Permissions[permission]
	return ok
}

// contextKey 是私有类型，避免与其他包的上下文键冲突。
type contextKey struct{}

// With 把 Principal 写入上下文。
func With(ctx context.Context, p *Principal) context.Context {
	return context.WithValue(ctx, contextKey{}, p)
}

// FromContext 从上下文取出 Principal；未认证时返回 false。
func FromContext(ctx context.Context) (*Principal, bool) {
	p, ok := ctx.Value(contextKey{}).(*Principal)
	return p, ok
}
