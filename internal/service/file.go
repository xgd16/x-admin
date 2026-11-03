package service

import (
	"context"
	"io"
	"path/filepath"
	"strings"

	"x-admin/internal/model"
	"x-admin/internal/pkg/apperr"
	"x-admin/internal/pkg/storage"
)

type FileService struct {
	files    FileRepository
	store    storage.Store
	maxBytes int64
}

func NewFileService(files FileRepository, store storage.Store, maxUploadMB int64) *FileService {
	if maxUploadMB <= 0 {
		maxUploadMB = 20
	}
	return &FileService{files: files, store: store, maxBytes: maxUploadMB * 1024 * 1024}
}

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

func (s *FileService) List(ctx context.Context, f model.FileFilter) ([]model.File, int64, error) {
	f.Normalize()
	return s.files.List(ctx, f)
}

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
	return s.store.Delete(ctx, file.Key)
}
