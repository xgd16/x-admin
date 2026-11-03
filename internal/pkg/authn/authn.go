package authn

import "context"

// Principal is the authenticated identity carried in the request context.
type Principal struct {
	UserID      uint64
	Username    string
	Nickname    string
	Super       bool
	Permissions map[string]struct{}
}

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

type contextKey struct{}

func With(ctx context.Context, p *Principal) context.Context {
	return context.WithValue(ctx, contextKey{}, p)
}

func FromContext(ctx context.Context) (*Principal, bool) {
	p, ok := ctx.Value(contextKey{}).(*Principal)
	return p, ok
}
