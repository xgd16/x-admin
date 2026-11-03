package security

import "golang.org/x/crypto/bcrypt"

// PasswordHasher 抽象密码哈希算法，便于测试替换。
type PasswordHasher interface {
	Hash(plain string) (string, error)
	Verify(hash, plain string) bool
}

// BcryptHasher 是生产环境使用的 bcrypt 实现。
type BcryptHasher struct {
	Cost int
}

// NewBcryptHasher 创建 bcrypt 哈希器；成本超出允许范围时使用默认成本。
func NewBcryptHasher(cost int) *BcryptHasher {
	if cost < bcrypt.MinCost || cost > bcrypt.MaxCost {
		cost = bcrypt.DefaultCost
	}
	return &BcryptHasher{Cost: cost}
}

// Hash 计算密码摘要。
func (h *BcryptHasher) Hash(plain string) (string, error) {
	b, err := bcrypt.GenerateFromPassword([]byte(plain), h.Cost)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// Verify 校验明文密码是否与摘要匹配。
func (h *BcryptHasher) Verify(hash, plain string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(plain)) == nil
}
