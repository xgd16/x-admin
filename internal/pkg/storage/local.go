// Package storage 定义文件存储抽象，并提供本地磁盘实现。
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

// Info 是一次保存操作返回的文件元信息。
type Info struct {
	Key  string `json:"key"`  // 存储层相对路径
	Name string `json:"name"` // 原始文件名
	URL  string `json:"url"`  // 对外访问地址
	MIME string `json:"mime"`
	Size int64  `json:"size"`
}

// Store 抽象文件存储，便于后续替换为对象存储。
type Store interface {
	Save(ctx context.Context, filename string, r io.Reader) (Info, error)
	Open(ctx context.Context, key string) (io.ReadCloser, error)
	Delete(ctx context.Context, key string) error
	URL(key string) string
}

// Option 用于在测试中替换时钟和 ID 生成器。
type Option func(*Local)

// WithClock 替换时钟。
func WithClock(now func() time.Time) Option { return func(l *Local) { l.now = now } }

// WithIDFunc 替换文件名 ID 生成器。
func WithIDFunc(fn func() string) Option { return func(l *Local) { l.idFn = fn } }

// Local 把文件存放在 Root 下，布局为 YYYY/MM/<random>.<ext>。
type Local struct {
	root      string
	urlPrefix string
	now       func() time.Time
	idFn      func() string
}

// NewLocal 创建本地存储并确保根目录存在。
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

// Save 写入文件。
// 先读取前 512 字节用于嗅探 MIME，再把头部与剩余内容一起写入磁盘；
// 文件名使用随机 ID 并保留经过清洗的扩展名，避免路径穿越和重名。
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

// Open 打开文件读取流。
func (l *Local) Open(_ context.Context, key string) (io.ReadCloser, error) {
	full, err := l.resolve(key)
	if err != nil {
		return nil, err
	}
	return os.Open(full)
}

// Delete 删除文件；文件不存在时视为成功。
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

// URL 返回文件的对外访问地址。
func (l *Local) URL(key string) string {
	return l.urlPrefix + "/" + strings.TrimLeft(key, "/")
}

// resolve 把存储键解析为绝对路径，并阻止路径穿越到 Root 之外。
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

// sanitizeExt 只允许 1-11 位小写字母或数字的扩展名，否则替换为 .bin。
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

// newID 生成 16 字节随机文件名；随机源故障时退回时间戳。
func newID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(b[:])
}
