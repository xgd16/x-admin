package handler

import (
	"github.com/gin-gonic/gin"

	"x-admin/internal/pkg/captcha"
	"x-admin/internal/pkg/response"
	"x-admin/internal/service"
)

type AuthHandler struct {
	auth    *service.AuthService
	captcha *captcha.Generator
}

func NewAuthHandler(auth *service.AuthService, captcha *captcha.Generator) *AuthHandler {
	return &AuthHandler{auth: auth, captcha: captcha}
}

func (h *AuthHandler) Captcha(c *gin.Context) {
	challenge, err := h.captcha.Generate(c.Request.Context())
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, challenge)
}

func (h *AuthHandler) Login(c *gin.Context) {
	var in service.LoginInput
	if !Bind(c, &in) {
		return
	}
	pair, err := h.auth.Login(c.Request.Context(), in, service.LoginMeta{IP: c.ClientIP(), UserAgent: c.Request.UserAgent()})
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, pair)
}

func (h *AuthHandler) Refresh(c *gin.Context) {
	var in struct {
		RefreshToken string `json:"refresh_token" binding:"required"`
	}
	if !Bind(c, &in) {
		return
	}
	pair, err := h.auth.Refresh(c.Request.Context(), in.RefreshToken)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, pair)
}

func (h *AuthHandler) Logout(c *gin.Context) {
	var in struct {
		RefreshToken string `json:"refresh_token"`
	}
	if !Bind(c, &in) {
		return
	}
	if err := h.auth.Logout(c.Request.Context(), in.RefreshToken); err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, nil)
}

func (h *AuthHandler) Me(c *gin.Context) {
	me, err := h.auth.Me(c.Request.Context(), CurrentUserID(c))
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, me)
}
