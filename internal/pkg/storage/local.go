package storage

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"
)

type Info struct {
	Key  string `json:"key"`
	Name string `json:"name"`
	URL  string `json:"url"`
	MIME string `json:"mime"`
	Size int64  `json:"size"`
}

type Store interface {
	Save(ctx context.Context, filename string, r io.Reader) (Info, error)
	Open(ctx context.Context, key string) (io.ReadCloser, error)
	Delete(ctx context.Context, key string) error
	URL(key string) string
}

type Option func(*Local)

func WithClock(now func() time.Time) Option { return func(l *Local) { l.now = now } }
func WithIDFunc(fn func() string) Option    { return func(l *Local) { l.idFn = fn } }

// Local stores files under Root in YYYY/MM/uuid.ext layout.
type Local struct {
	root      string
	urlPrefix string
	now       func() time.Time
	idFn      func() string
}

func NewLocal(root, urlPrefix string, opts ...Option) (*Local, error) {
	if root == "" {
		return nil, fmt.Errorf("storage: root is required")
	}
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	l := &Local{root: abs, urlPrefix: strings.TrimRight(urlPrefix, "/"), now: time.Now, idFn: newID}
	for _, opt := range opts {
		opt(l)
	}
	if err := os.MkdirAll(l.root, 0o755); err != nil {
		return nil, err
	}
	return l, nil
}

func (l *Local) Save(_ context.Context, filename string, r io.Reader) (Info, error) {
	if filename == "" {
		filename = "file"
	}
	ext := sanitizeExt(filepath.Ext(filename))
	key := path.Join(l.now().UTC().Format("2006/01"), l.idFn()+ext)

	head := make([]byte, 512)
	n, err := io.ReadFull(r, head)
	if err != nil && err != io.ErrUnexpectedEOF && err != io.EOF {
		return Info{}, err
	}
	head = head[:n]
	mime := http.DetectContentType(head)

	dst := filepath.Join(l.root, filepath.FromSlash(key))
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return Info{}, err
	}
	f, err := os.Create(dst)
	if err != nil {
		return Info{}, err
	}
	size, copyErr := io.Copy(f, io.MultiReader(strings.NewReader(string(head)), r))
	closeErr := f.Close()
	if copyErr != nil {
		_ = os.Remove(dst)
		return Info{}, copyErr
	}
	if closeErr != nil {
		_ = os.Remove(dst)
		return Info{}, closeErr
	}
	return Info{
		Key:  key,
		Name: filepath.Base(filename),
		URL:  l.URL(key),
		MIME: mime,
		Size: size,
	}, nil
}

func (l *Local) Open(_ context.Context, key string) (io.ReadCloser, error) {
	full, err := l.resolve(key)
	if err != nil {
		return nil, err
	}
	return os.Open(full)
}

func (l *Local) Delete(_ context.Context, key string) error {
	full, err := l.resolve(key)
	if err != nil {
		return err
	}
	err = os.Remove(full)
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

func (l *Local) URL(key string) string {
	return l.urlPrefix + "/" + strings.TrimLeft(key, "/")
}

func (l *Local) resolve(key string) (string, error) {
	cleaned := path.Clean("/" + strings.ReplaceAll(key, "\\", "/"))
	if strings.Contains(cleaned, "..") || cleaned == "/" {
		return "", fmt.Errorf("storage: invalid key")
	}
	full := filepath.Join(l.root, filepath.FromSlash(strings.TrimPrefix(cleaned, "/")))
	if full != l.root && !strings.HasPrefix(full, l.root+string(os.PathSeparator)) {
		return "", fmt.Errorf("storage: invalid key")
	}
	return full, nil
}

func sanitizeExt(ext string) string {
	ext = strings.ToLower(ext)
	if len(ext) < 2 || len(ext) > 12 {
		return ".bin"
	}
	for _, r := range ext[1:] {
		if (r < 'a' || r > 'z') && (r < '0' || r > '9') {
			return ".bin"
		}
	}
	return ext
}

func newID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(b[:])
}
