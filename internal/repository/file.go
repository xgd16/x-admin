package repository

import (
	"context"
	"strings"
	"time"

	"gorm.io/gorm"

	"x-admin/internal/model"
)

// FileRepo 提供上传文件元数据的数据访问。
type FileRepo struct {
	*Base[model.File]
	db *gorm.DB
}

// NewFileRepo 创建文件仓储。
func NewFileRepo(db *gorm.DB) *FileRepo {
	return &FileRepo{Base: NewBase[model.File](db), db: db}
}

// Get 按 ID 查询文件记录。
func (r *FileRepo) Get(ctx context.Context, id uint64) (*model.File, error) {
	return r.FindByID(ctx, id)
}

// List 分页查询文件，支持文件名/MIME 关键字、MIME 前缀和文件大类过滤。
func (r *FileRepo) List(ctx context.Context, f model.FileFilter) ([]model.File, int64, error) {
	q := r.db.WithContext(ctx).Model(&model.File{})
	if f.Keyword != "" {
		kw := like(f.Keyword)
		q = q.Where("(name LIKE ? OR mime LIKE ?)", kw, kw)
	}
	if f.MIME != "" {
		// 前缀匹配，例如 mime=image/ 可以筛出所有图片。
		q = q.Where("mime LIKE ?", f.MIME+"%")
	}
	if where := fileKindWhere(f.Kind); where != "" {
		q = q.Where(where)
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	q = paginate(q.Order("id DESC"), f.Page, f.PageSize)
	var rows []model.File
	if err := q.Find(&rows).Error; err != nil {
		return nil, 0, err
	}
	return rows, total, nil
}

// TotalSize 返回全部文件占用的总字节数。
func (r *FileRepo) TotalSize(ctx context.Context) (int64, error) {
	var total int64
	err := r.db.WithContext(ctx).Model(&model.File{}).Select("COALESCE(SUM(size),0)").Scan(&total).Error
	return total, err
}

// CountAll 统计文件数量。
func (r *FileRepo) CountAll(ctx context.Context) (int64, error) {
	return r.Count(ctx, "")
}

// 文件大类判定条件：优先按扩展名，其次按 MIME。
// 这些常量在包初始化时拼接成 SQL 片段，输入均为固定字面量，不含用户数据。
var (
	imageExts      = sqlList("jpg", "jpeg", "png", "gif", "webp", "svg", "bmp", "ico", "avif", "heic", "heif", "tif", "tiff")
	videoExts      = sqlList("mp4", "webm", "mov", "m4v", "avi", "mkv", "flv", "wmv", "mpeg", "mpg")
	audioExts      = sqlList("mp3", "wav", "ogg", "flac", "aac", "m4a", "wma", "opus")
	documentExts   = sqlList("pdf", "doc", "docx", "xls", "xlsx", "ppt", "pptx", "txt", "md", "csv", "json", "xml", "yaml", "yml", "html", "htm", "rtf")
	archiveExts    = sqlList("zip", "rar", "7z", "tar", "gz", "bz2", "xz")
	imageClause    = "(LOWER(ext) IN (" + imageExts + ") OR mime LIKE 'image/%')"
	videoClause    = "(LOWER(ext) IN (" + videoExts + ") OR mime LIKE 'video/%')"
	audioClause    = "(LOWER(ext) IN (" + audioExts + ") OR mime LIKE 'audio/%')"
	documentClause = "(LOWER(ext) IN (" + documentExts + ") OR mime LIKE 'text/%' OR mime = 'application/pdf' OR mime LIKE 'application/vnd.ms-%' OR mime LIKE 'application/vnd.openxmlformats-officedocument.%' OR mime IN ('application/json', 'application/xml', 'application/rtf'))"
	archiveClause  = "(LOWER(ext) IN (" + archiveExts + "))"
)

// sqlList 把字符串列表拼成 SQL 的 IN 值列表（仅用于固定字面量）。
func sqlList(values ...string) string {
	quoted := make([]string, len(values))
	for i, value := range values {
		quoted[i] = "'" + value + "'"
	}
	return strings.Join(quoted, ",")
}

// fileKindWhere 把逗号分隔的文件大类转换成 OR 连接的 SQL 条件。
func fileKindWhere(value string) string {
	seen := make(map[string]bool)
	clauses := make([]string, 0, 5)
	for _, raw := range strings.Split(value, ",") {
		kind := strings.ToLower(strings.TrimSpace(raw))
		if kind == "" || kind == "all" || seen[kind] {
			continue
		}
		seen[kind] = true
		if clause, ok := fileKindClause(kind); ok {
			clauses = append(clauses, clause)
		}
	}
	if len(clauses) == 0 {
		return ""
	}
	return "(" + strings.Join(clauses, " OR ") + ")"
}

// fileKindClause 返回单个文件大类的判定条件；未知类型返回 false。
func fileKindClause(kind string) (string, bool) {
	switch kind {
	case "image":
		return imageClause, true
	case "video":
		return videoClause, true
	case "audio":
		return audioClause, true
	case "document":
		return documentClause, true
	case "archive":
		return archiveClause, true
	case "other":
		// other = 不属于以上任何一类。
		return "NOT " + "(" + strings.Join([]string{imageClause, videoClause, audioClause, documentClause, archiveClause}, " OR ") + ")", true
	default:
		return "", false
	}
}

// CreateShare 写入一条文件分享记录，只保存令牌摘要。
func (r *FileRepo) CreateShare(ctx context.Context, share *model.FileShare) error {
	return r.db.WithContext(ctx).Create(share).Error
}

// GetShareByHash 按令牌摘要查询分享记录。
func (r *FileRepo) GetShareByHash(ctx context.Context, hash string) (*model.FileShare, error) {
	var share model.FileShare
	if err := r.db.WithContext(ctx).Where("token_hash = ?", hash).First(&share).Error; err != nil {
		return nil, err
	}
	return &share, nil
}

// ListShares 返回某个文件的分享记录，按创建时间倒序。
func (r *FileRepo) ListShares(ctx context.Context, fileID uint64) ([]model.FileShare, error) {
	var rows []model.FileShare
	err := r.db.WithContext(ctx).Where("file_id = ?", fileID).
		Order("created_at DESC, id DESC").Find(&rows).Error
	return rows, err
}

// RevokeShare 标记分享链接为已撤销。
func (r *FileRepo) RevokeShare(ctx context.Context, fileID, id uint64) error {
	res := r.db.WithContext(ctx).Model(&model.FileShare{}).
		Where("id = ? AND file_id = ? AND revoked_at IS NULL", id, fileID).
		Update("revoked_at", time.Now())
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

// DeleteSharesByFile 在删除文件时物理删除其分享记录。
func (r *FileRepo) DeleteSharesByFile(ctx context.Context, fileID uint64) error {
	return r.db.WithContext(ctx).Unscoped().
		Where("file_id = ?", fileID).Delete(&model.FileShare{}).Error
}

// PruneShares 物理删除过期或撤销超过保留期的分享记录。
func (r *FileRepo) PruneShares(ctx context.Context, before time.Time) error {
	return r.db.WithContext(ctx).Unscoped().
		Where("expires_at < ? OR revoked_at < ?", before, before).
		Delete(&model.FileShare{}).Error
}
