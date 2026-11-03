package service

import (
	"context"

	"x-admin/internal/model"
	"x-admin/internal/pkg/apperr"
	"x-admin/internal/pkg/security"
)

type ProfileInput struct {
	Nickname string `json:"nickname"`
	Email    string `json:"email"`
	Phone    string `json:"phone"`
	Avatar   string `json:"avatar"`
}

type ProfileService struct {
	users       UserRepository
	hasher      security.PasswordHasher
	passwordMin int
}

func NewProfileService(users UserRepository, hasher security.PasswordHasher, passwordMin int) *ProfileService {
	if passwordMin < 8 {
		passwordMin = 8
	}
	return &ProfileService{users: users, hasher: hasher, passwordMin: passwordMin}
}

func (s *ProfileService) Get(ctx context.Context, userID uint64) (*model.User, error) {
	user, err := s.users.Get(ctx, userID)
	if err != nil {
		if IsNotFound(err) {
			return nil, apperr.ErrNotFound
		}
		return nil, err
	}
	return user, nil
}

func (s *ProfileService) Update(ctx context.Context, userID uint64, in ProfileInput) (*model.User, error) {
	user, err := s.users.Get(ctx, userID)
	if err != nil {
		if IsNotFound(err) {
			return nil, apperr.ErrNotFound
		}
		return nil, err
	}
	if err := ValidateEmail(trim(in.Email)); err != nil {
		return nil, err
	}
	if err := ValidatePhone(trim(in.Phone)); err != nil {
		return nil, err
	}
	user.Nickname = trim(in.Nickname)
	user.Email = trim(in.Email)
	user.Phone = trim(in.Phone)
	user.Avatar = trim(in.Avatar)
	if err := s.users.Update(ctx, user); err != nil {
		return nil, err
	}
	return s.Get(ctx, userID)
}

func (s *ProfileService) ChangePassword(ctx context.Context, userID uint64, oldPassword, newPassword string) error {
	user, err := s.users.Get(ctx, userID)
	if err != nil {
		if IsNotFound(err) {
			return apperr.ErrNotFound
		}
		return err
	}
	if !s.hasher.Verify(user.Password, oldPassword) {
		return apperr.New(apperr.CodeInvalidArgument, "原密码不正确")
	}
	if err := ValidatePassword(newPassword, s.passwordMin); err != nil {
		return err
	}
	if oldPassword == newPassword {
		return apperr.New(apperr.CodeInvalidArgument, "新密码不能与原密码相同")
	}
	hash, err := s.hasher.Hash(newPassword)
	if err != nil {
		return apperr.Wrap(err, apperr.CodeInternal, "密码加密失败")
	}
	return s.users.UpdateFields(ctx, userID, map[string]any{"password": hash})
}
