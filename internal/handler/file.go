package handler

import (
	"net/http"
	"net/url"

	"github.com/gin-gonic/gin"

	"x-admin/internal/model"
	"x-admin/internal/pkg/apperr"
	"x-admin/internal/pkg/response"
	"x-admin/internal/service"
)

// FileHandler 处理文件上传、列表、下载与删除接口。
type FileHandler struct {
	files *service.FileService
}

// NewFileHandler 创建文件处理器。
func NewFileHandler(files *service.FileService) *FileHandler { return &FileHandler{files: files} }

// List 分页查询文件，支持关键字、MIME 前缀和文件大类过滤。
func (h *FileHandler) List(c *gin.Context) {
	var f model.FileFilter
	if err := c.ShouldBindQuery(&f); err != nil {
		response.Error(c, apperr.Wrap(err, apperr.CodeInvalidArgument, "查询参数不合法"))
		return
	}
	f.Normalize()
	list, total, err := h.files.List(c.Request.Context(), f)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Page(c, list, total, f.Page, f.PageSize)
}

// Upload 接收 multipart 文件并保存，请求体硬上限为 64MB。
// 业务层的单文件大小限制由 FileService 按配置再次校验。
func (h *FileHandler) Upload(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 64<<20)
	header, err := c.FormFile("file")
	if err != nil {
		response.Error(c, apperr.Wrap(err, apperr.CodeInvalidArgument, "请选择要上传的文件"))
		return
	}
	file, err := header.Open()
	if err != nil {
		response.Error(c, apperr.Wrap(err, apperr.CodeInvalidArgument, "无法读取文件"))
		return
	}
	defer file.Close()
	record, err := h.files.Upload(c.Request.Context(), header.Filename, file, CurrentUserID(c))
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Created(c, record)
}

// Download 以流式方式返回文件内容，并设置中文兼容的下载文件名。
func (h *FileHandler) Download(c *gin.Context) {
	id, ok := PathID(c)
	if !ok {
		return
	}
	record, rc, err := h.files.Download(c.Request.Context(), id)
	if err != nil {
		response.Error(c, err)
		return
	}
	defer rc.Close()
	disposition := `attachment; filename="` + record.Name + `"; filename*=UTF-8''` + url.QueryEscape(record.Name)
	c.DataFromReader(http.StatusOK, record.Size, record.MIME, rc, map[string]string{
		"Content-Disposition": disposition,
	})
}

// Delete 删除文件记录及磁盘上的物理文件。
func (h *FileHandler) Delete(c *gin.Context) {
	id, ok := PathID(c)
	if !ok {
		return
	}
	if err := h.files.Delete(c.Request.Context(), id); err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, nil)
}
