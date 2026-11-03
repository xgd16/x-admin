package handler

import (
	"github.com/gin-gonic/gin"

	"x-admin/internal/pkg/apperr"
	"x-admin/internal/pkg/pagination"
	"x-admin/internal/pkg/response"
	"x-admin/internal/service"
)

type JobHandler struct {
	jobs *service.JobService
}

func NewJobHandler(jobs *service.JobService) *JobHandler { return &JobHandler{jobs: jobs} }

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
