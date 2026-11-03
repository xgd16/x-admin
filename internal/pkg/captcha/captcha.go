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

const alphabet = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"

type Challenge struct {
	ID        string `json:"captcha_id"`
	Image     string `json:"image"`
	ExpiresIn int    `json:"expires_in"`
}

type Option func(*Generator)

func WithReader(r io.Reader) Option         { return func(g *Generator) { g.reader = r } }
func WithRand(r *mathrand.Rand) Option      { return func(g *Generator) { g.rand = r } }
func WithClock(now func() time.Time) Option { return func(g *Generator) { g.now = now } }
func WithIDFunc(fn func() string) Option    { return func(g *Generator) { g.idFn = fn } }
func WithLength(n int) Option {
	return func(g *Generator) {
		if n > 0 && n <= 8 {
			g.length = n
		}
	}
}

// Generator creates an SVG captcha and keeps the answer in a cache.Store.
type Generator struct {
	store  cache.Store
	ttl    time.Duration
	length int
	reader io.Reader
	rand   *mathrand.Rand
	now    func() time.Time
	idFn   func() string
}

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

// Verify consumes the challenge regardless of the outcome, so a code cannot be
// replayed after a failed attempt.
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

func key(id string) string { return "captcha:" + id }

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

func randomID() string {
	var b [12]byte
	if _, err := rand.Read(b[:]); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(b[:])
}
