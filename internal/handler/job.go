package handler

import (
	"github.com/gin-gonic/gin"

	"x-admin/internal/pkg/apperr"
	"x-admin/internal/pkg/pagination"
	"x-admin/internal/pkg/response"
	"x-admin/internal/service"
)

// JobHandler 处理定时任务管理接口。
type JobHandler struct {
	jobs *service.JobService
}

// NewJobHandler 创建任务处理器。
func NewJobHandler(jobs *service.JobService) *JobHandler { return &JobHandler{jobs: jobs} }

// List 分页查询定时任务。
func (h *JobHandler) List(c *gin.Context) {
	var q pagination.Query
	if err := c.ShouldBindQuery(&q); err != nil {
		response.Error(c, apperr.Wrap(err, apperr.CodeInvalidArgument, "查询参数不合法"))
		return
	}
	q.Normalize()
	list, total, err := h.jobs.List(c.Request.Context(), q)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Page(c, list, total, q.Page, q.PageSize)
}

// Runs 分页查询任务执行历史。
func (h *JobHandler) Runs(c *gin.Context) {
	id, ok := PathID(c)
	if !ok {
		return
	}
	var q pagination.Query
	if err := c.ShouldBindQuery(&q); err != nil {
		response.Error(c, apperr.Wrap(err, apperr.CodeInvalidArgument, "查询参数不合法"))
		return
	}
	list, total, err := h.jobs.Runs(c.Request.Context(), id, q)
	if err != nil {
		response.Error(c, err)
		return
	}
	q.Normalize()
	response.Page(c, list, total, q.Page, q.PageSize)
}

// Create 创建任务；启用状态下会立即注册到调度器。
func (h *JobHandler) Create(c *gin.Context) {
	var in service.JobInput
	if !Bind(c, &in) {
		return
	}
	job, err := h.jobs.Create(c.Request.Context(), in)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Created(c, job)
}

// Update 更新任务并重建调度。
func (h *JobHandler) Update(c *gin.Context) {
	id, ok := PathID(c)
	if !ok {
		return
	}
	var in service.JobInput
	if !Bind(c, &in) {
		return
	}
	job, err := h.jobs.Update(c.Request.Context(), id, in)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, job)
}

// Delete 删除任务并移除调度。
func (h *JobHandler) Delete(c *gin.Context) {
	id, ok := PathID(c)
	if !ok {
		return
	}
	if err := h.jobs.Delete(c.Request.Context(), id); err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, nil)
}

// SetStatus 启用或停用任务，同步增删调度。
func (h *JobHandler) SetStatus(c *gin.Context) {
	id, ok := PathID(c)
	if !ok {
		return
	}
	var in struct {
		Status int8 `json:"status"`
	}
	if !Bind(c, &in) {
		return
	}
	if err := h.jobs.SetStatus(c.Request.Context(), id, in.Status); err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, nil)
}

// Run 立即执行一次任务，不影响原有调度计划。
func (h *JobHandler) Run(c *gin.Context) {
	id, ok := PathID(c)
	if !ok {
		return
	}
	if err := h.jobs.RunNow(c.Request.Context(), id); err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, nil)
}
