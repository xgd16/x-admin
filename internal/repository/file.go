package repository

import (
	"context"
	"strings"

	"gorm.io/gorm"

	"x-admin/internal/model"
)

type FileRepo struct {
	*Base[model.File]
	db *gorm.DB
}

func NewFileRepo(db *gorm.DB) *FileRepo {
	return &FileRepo{Base: NewBase[model.File](db), db: db}
}

func (r *FileRepo) Get(ctx context.Context, id uint64) (*model.File, error) {
	return r.FindByID(ctx, id)
}

func (r *FileRepo) List(ctx context.Context, f model.FileFilter) ([]model.File, int64, error) {
	q := r.db.WithContext(ctx).Model(&model.File{})
	if f.Keyword != "" {
		kw := like(f.Keyword)
		q = q.Where("(name LIKE ? OR mime LIKE ?)", kw, kw)
	}
	if f.MIME != "" {
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

func (r *FileRepo) TotalSize(ctx context.Context) (int64, error) {
	var total int64
	err := r.db.WithContext(ctx).Model(&model.File{}).Select("COALESCE(SUM(size),0)").Scan(&total).Error
	return total, err
}

func (r *FileRepo) CountAll(ctx context.Context) (int64, error) {
	return r.Count(ctx, "")
}

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

func sqlList(values ...string) string {
	quoted := make([]string, len(values))
	for i, value := range values {
		quoted[i] = "'" + value + "'"
	}
	return strings.Join(quoted, ",")
}

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
		return "NOT " + "(" + strings.Join([]string{imageClause, videoClause, audioClause, documentClause, archiveClause}, " OR ") + ")", true
	default:
		return "", false
	}
}
