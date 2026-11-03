// Package captcha 生成并校验 SVG 图形验证码。
//
// 答案保存在 cache.Store 中，校验一次后立即删除，防止重放。
package captcha

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
	"math/big"
	mathrand "math/rand"
	"strings"
	"time"

	"x-admin/internal/pkg/cache"
)

// alphabet 去掉了容易混淆的 I、O、0、1。
const alphabet = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"

// Challenge 是生成验证码的响应。
type Challenge struct {
	ID        string `json:"captcha_id"`
	Image     string `json:"image"`      // SVG data URL
	ExpiresIn int    `json:"expires_in"` // 有效秒数
}

// Option 用于在测试中替换随机源、时钟和 ID 生成器。
type Option func(*Generator)

// WithReader 替换密码学随机源。
func WithReader(r io.Reader) Option { return func(g *Generator) { g.reader = r } }

// WithRand 替换用于绘制干扰线的伪随机源。
func WithRand(r *mathrand.Rand) Option { return func(g *Generator) { g.rand = r } }

// WithClock 替换时钟。
func WithClock(now func() time.Time) Option { return func(g *Generator) { g.now = now } }

// WithIDFunc 替换挑战 ID 生成函数。
func WithIDFunc(fn func() string) Option { return func(g *Generator) { g.idFn = fn } }

// WithLength 设置验证码长度，仅接受 1-8。
func WithLength(n int) Option {
	return func(g *Generator) {
		if n > 0 && n <= 8 {
			g.length = n
		}
	}
}

// Generator 生成 SVG 验证码并把答案保存在 cache.Store。
type Generator struct {
	store  cache.Store
	ttl    time.Duration
	length int
	reader io.Reader
	rand   *mathrand.Rand
	now    func() time.Time
	idFn   func() string
}

// New 创建验证码生成器。默认长度 4，使用密码学随机源和进程级伪随机源。
func New(store cache.Store, ttl time.Duration, opts ...Option) *Generator {
	g := &Generator{
		store:  store,
		ttl:    ttl,
		length: 4,
		reader: rand.Reader,
		rand:   mathrand.New(mathrand.NewSource(time.Now().UnixNano())),
		now:    time.Now,
		idFn:   randomID,
	}
	for _, opt := range opts {
		opt(g)
	}
	return g
}

// Generate 生成验证码，返回挑战 ID 和可直接用于 <img src> 的 data URL。
func (g *Generator) Generate(ctx context.Context) (*Challenge, error) {
	code, err := g.randomString(g.length)
	if err != nil {
		return nil, err
	}
	id := g.idFn()
	if err := g.store.Set(ctx, key(id), code, g.ttl); err != nil {
		return nil, err
	}
	svg := g.svg(code)
	return &Challenge{
		ID:        id,
		Image:     "data:image/svg+xml;base64," + base64.StdEncoding.EncodeToString([]byte(svg)),
		ExpiresIn: int(g.ttl.Seconds()),
	}, nil
}

// Verify 校验验证码。无论成功与否都会消费挑战，失败后不能重试同一张图。
func (g *Generator) Verify(ctx context.Context, id, answer string) bool {
	if id == "" || answer == "" {
		return false
	}
	expected, err := g.store.Get(ctx, key(id))
	_ = g.store.Del(ctx, key(id))
	if err != nil {
		return false
	}
	return strings.EqualFold(strings.TrimSpace(answer), expected)
}

// key 生成验证码在缓存中的键。
func key(id string) string { return "captcha:" + id }

// randomString 从 alphabet 中均匀抽取 n 个字符。
func (g *Generator) randomString(n int) (string, error) {
	out := make([]byte, n)
	for i := range out {
		v, err := rand.Int(g.reader, big.NewInt(int64(len(alphabet))))
		if err != nil {
			return "", err
		}
		out[i] = alphabet[v.Int64()]
	}
	return string(out), nil
}

// svg 绘制带渐变、干扰曲线和噪点的验证码图片。
func (g *Generator) svg(code string) string {
	var b strings.Builder
	b.WriteString(`<svg xmlns="http://www.w3.org/2000/svg" width="120" height="40" viewBox="0 0 120 40">`)
	b.WriteString(`<defs><linearGradient id="g" x1="0" y1="0" x2="1" y2="1">`)
	b.WriteString(`<stop offset="0" stop-color="#f6fbff"/><stop offset="1" stop-color="#dcecfb"/></linearGradient></defs>`)
	b.WriteString(`<rect width="120" height="40" rx="10" fill="url(#g)"/>`)
	for i := 0; i < 5; i++ {
		x1 := g.rand.Intn(120)
		y1 := g.rand.Intn(40)
		x2 := g.rand.Intn(120)
		y2 := g.rand.Intn(40)
		opacity := 0.18 + g.rand.Float64()*0.2
		fmt.Fprintf(&b, `<path d="M%d %d Q%d %d %d %d" stroke="#7aa7c7" stroke-width="1" fill="none" opacity="%.2f"/>`, x1, y1, (x1+x2)/2, g.rand.Intn(40), x2, y2, opacity)
	}
	colors := []string{"#e2542b", "#2f6f9f", "#6b4fbb", "#187a5a"}
	for i, ch := range code {
		x := 14 + i*26
		y := 27 + g.rand.Intn(5) - 2
		rotate := g.rand.Intn(24) - 12
		fmt.Fprintf(&b, `<text x="%d" y="%d" fill="%s" font-family="Menlo,monospace" font-size="22" font-weight="700" transform="rotate(%d %d %d)">%c</text>`, x, y, colors[i%len(colors)], rotate, x, y, ch)
	}
	for i := 0; i < 18; i++ {
		fmt.Fprintf(&b, `<circle cx="%d" cy="%d" r="%.1f" fill="#7aa7c7" opacity=".28"/>`, g.rand.Intn(120), g.rand.Intn(40), 0.5+g.rand.Float64())
	}
	b.WriteString(`</svg>`)
	return b.String()
}

// randomID 生成 12 字节随机挑战 ID，失败时退回时间戳。
func randomID() string {
	var b [12]byte
	if _, err := rand.Read(b[:]); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(b[:])
}
