package service

import (
	"context"
	"io"
	"path/filepath"
	"strings"
	"time"

	"x-admin/internal/model"
	"x-admin/internal/pkg/apperr"
	"x-admin/internal/pkg/security"
	"x-admin/internal/pkg/storage"
)

// FileService 编排文件存储与元数据记录。
type FileService struct {
	files    FileRepository
	store    storage.Store
	maxBytes int64
}

// NewFileService 创建文件服务。maxUploadMB <= 0 时默认 20MB。
func NewFileService(files FileRepository, store storage.Store, maxUploadMB int64) *FileService {
	if maxUploadMB <= 0 {
		maxUploadMB = 20
	}
	return &FileService{files: files, store: store, maxBytes: maxUploadMB * 1024 * 1024}
}

// Upload 保存文件并写入元数据。
// 通过 LimitReader 多读 1 字节来检测超限；超限或落库失败时会删除已保存的物理文件。
func (s *FileService) Upload(ctx context.Context, filename string, r io.Reader, uploaderID uint64) (*model.File, error) {
	filename = filepath.Base(strings.TrimSpace(filename))
	if filename == "" || filename == "." {
		return nil, apperr.New(apperr.CodeInvalidArgument, "文件名不能为空")
	}
	info, err := s.store.Save(ctx, filename, io.LimitReader(r, s.maxBytes+1))
	if err != nil {
		return nil, apperr.Wrap(err, apperr.CodeInternal, "文件保存失败")
	}
	if info.Size > s.maxBytes {
		_ = s.store.Delete(ctx, info.Key)
		return nil, apperr.Newf(apperr.CodeInvalidArgument, "文件不能超过 %d MB", s.maxBytes/1024/1024)
	}
	file := &model.File{
		Name: info.Name, Key: info.Key, URL: info.URL, MIME: info.MIME,
		Ext: strings.TrimPrefix(filepath.Ext(info.Name), "."), Size: info.Size, UploaderID: uploaderID,
	}
	if err := s.files.Create(ctx, file); err != nil {
		_ = s.store.Delete(ctx, info.Key)
		return nil, err
	}
	return file, nil
}

// List 分页查询文件元数据。
func (s *FileService) List(ctx context.Context, f model.FileFilter) ([]model.File, int64, error) {
	f.Normalize()
	return s.files.List(ctx, f)
}

// Get 返回文件元数据。
func (s *FileService) Get(ctx context.Context, id uint64) (*model.File, error) {
	file, err := s.files.Get(ctx, id)
	if err != nil {
		if IsNotFound(err) {
			return nil, apperr.ErrNotFound
		}
		return nil, err
	}
	return file, nil
}

// Download 返回文件元数据与内容流，调用方负责关闭 ReadCloser。
func (s *FileService) Download(ctx context.Context, id uint64) (*model.File, io.ReadCloser, error) {
	file, err := s.files.Get(ctx, id)
	if err != nil {
		if IsNotFound(err) {
			return nil, nil, apperr.ErrNotFound
		}
		return nil, nil, err
	}
	rc, err := s.store.Open(ctx, file.Key)
	if err != nil {
		return nil, nil, apperr.Wrap(err, apperr.CodeNotFound, "文件已丢失")
	}
	return file, rc, nil
}

// CreateShare 为文件创建可撤销的分享链接，返回记录与明文令牌。
// 明文令牌只返回一次，数据库仅保存 SHA-256 摘要。
func (s *FileService) CreateShare(ctx context.Context, fileID, createdBy uint64, ttl time.Duration) (*model.FileShare, string, error) {
	file, err := s.Get(ctx, fileID)
	if err != nil {
		return nil, "", err
	}
	if !strings.HasPrefix(file.URL, "/uploads/") {
		return nil, "", apperr.New(apperr.CodeInvalidArgument, "该文件不支持生成分享链接")
	}
	if ttl < time.Minute {
		ttl = time.Minute
	}
	if ttl > 7*24*time.Hour {
		ttl = 7 * 24 * time.Hour
	}
	token, err := security.RandomToken()
	if err != nil {
		return nil, "", apperr.Wrap(err, apperr.CodeInternal, "生成分享链接失败")
	}
	share := &model.FileShare{
		FileID: fileID, TokenHash: security.HashToken(token),
		CreatedBy: createdBy, ExpiresAt: time.Now().Add(ttl),
	}
	if err := s.files.CreateShare(ctx, share); err != nil {
		return nil, "", err
	}
	// 清理过期或撤销超过 30 天的记录，避免分享表无限增长。
	_ = s.files.PruneShares(ctx, time.Now().AddDate(0, 0, -30))
	return share, token, nil
}

// Shares 返回某个文件的全部分享记录（不含令牌明文）。
func (s *FileService) Shares(ctx context.Context, fileID uint64) ([]model.FileShare, error) {
	if _, err := s.Get(ctx, fileID); err != nil {
		return nil, err
	}
	rows, err := s.files.ListShares(ctx, fileID)
	if err != nil {
		return nil, err
	}
	if rows == nil {
		rows = []model.FileShare{}
	}
	return rows, nil
}

// RevokeShare 撤销一条分享链接，撤销后立即无法访问。
func (s *FileService) RevokeShare(ctx context.Context, fileID, id uint64) error {
	if err := s.files.RevokeShare(ctx, fileID, id); err != nil {
		if IsNotFound(err) {
			return apperr.ErrNotFound
		}
		return err
	}
	return nil
}

// ResolveShare 校验分享令牌并返回对应文件的访问路径。
func (s *FileService) ResolveShare(ctx context.Context, token string) (string, error) {
	if token == "" {
		return "", apperr.New(apperr.CodeUnauthorized, "分享链接无效")
	}
	share, err := s.files.GetShareByHash(ctx, security.HashToken(token))
	if err != nil {
		if IsNotFound(err) {
			return "", apperr.New(apperr.CodeTokenExpired, "分享链接已失效")
		}
		return "", err
	}
	if share.RevokedAt != nil || !share.ExpiresAt.After(time.Now()) {
		return "", apperr.New(apperr.CodeTokenExpired, "分享链接已失效")
	}
	file, err := s.files.Get(ctx, share.FileID)
	if err != nil {
		if IsNotFound(err) {
			return "", apperr.New(apperr.CodeTokenExpired, "分享链接已失效")
		}
		return "", err
	}
	return file.URL, nil
}

// Delete 先删除元数据，再删除物理文件；物理文件不存在时视为成功。
func (s *FileService) Delete(ctx context.Context, id uint64) error {
	file, err := s.files.Get(ctx, id)
	if err != nil {
		if IsNotFound(err) {
			return apperr.ErrNotFound
		}
		return err
	}
	if err := s.files.Delete(ctx, id); err != nil {
		return err
	}
	// 文件删除后分享链接必然失效，顺手清理分享记录。
	_ = s.files.DeleteSharesByFile(ctx, id)
	return s.store.Delete(ctx, file.Key)
}
