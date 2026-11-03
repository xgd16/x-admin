package handler

import (
	"github.com/gin-gonic/gin"

	"x-admin/internal/pkg/response"
	"x-admin/internal/service"
)

type ProfileHandler struct {
	profile *service.ProfileService
}

func NewProfileHandler(profile *service.ProfileService) *ProfileHandler {
	return &ProfileHandler{profile: profile}
}

func (h *ProfileHandler) Get(c *gin.Context) {
	user, err := h.profile.Get(c.Request.Context(), CurrentUserID(c))
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, user)
}

func (h *ProfileHandler) Update(c *gin.Context) {
	var in service.ProfileInput
	if !Bind(c, &in) {
		return
	}
	user, err := h.profile.Update(c.Request.Context(), CurrentUserID(c), in)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, user)
}

func (h *ProfileHandler) ChangePassword(c *gin.Context) {
	var in struct {
		OldPassword string `json:"old_password" binding:"required"`
		NewPassword string `json:"new_password" binding:"required"`
	}
	if !Bind(c, &in) {
		return
	}
	if err := h.profile.ChangePassword(c.Request.Context(), CurrentUserID(c), in.OldPassword, in.NewPassword); err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, nil)
}
