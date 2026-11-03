package service

import (
	"context"

	"x-admin/internal/model"
	"x-admin/internal/pkg/apperr"
)

type RoleInput struct {
	Name    string   `json:"name"`
	Code    string   `json:"code"`
	Status  int8     `json:"status"`
	Sort    int      `json:"sort"`
	Remark  string   `json:"remark"`
	MenuIDs []uint64 `json:"menu_ids"`
}

type RoleService struct {
	roles RoleRepository
	menus MenuRepository
}

func NewRoleService(roles RoleRepository, menus MenuRepository) *RoleService {
	return &RoleService{roles: roles, menus: menus}
}

func (s *RoleService) List(ctx context.Context, f model.RoleFilter) ([]model.Role, int64, error) {
	f.Normalize()
	return s.roles.List(ctx, f)
}

func (s *RoleService) All(ctx context.Context) ([]model.Role, error) {
	return s.roles.All(ctx)
}

func (s *RoleService) Create(ctx context.Context, in RoleInput) (*model.Role, error) {
	if err := s.validate(ctx, in, 0); err != nil {
		return nil, err
	}
	role := &model.Role{
		Name: trim(in.Name), Code: trim(in.Code), Status: in.Status,
		Sort: in.Sort, Remark: trim(in.Remark),
	}
	if err := s.roles.Create(ctx, role); err != nil {
		return nil, err
	}
	if len(in.MenuIDs) > 0 {
		if err := s.roles.SetMenus(ctx, role.ID, in.MenuIDs); err != nil {
			return nil, err
		}
	}
	return role, nil
}

func (s *RoleService) Update(ctx context.Context, id uint64, in RoleInput) (*model.Role, error) {
	role, err := s.roles.Get(ctx, id)
	if err != nil {
		if IsNotFound(err) {
			return nil, apperr.ErrNotFound
		}
		return nil, err
	}
	if err := s.validate(ctx, in, id); err != nil {
		return nil, err
	}
	if role.Code == "super_admin" && trim(in.Code) != role.Code {
		return nil, apperr.New(apperr.CodeConflict, "内置超级管理员编码不可修改")
	}
	role.Name = trim(in.Name)
	role.Code = trim(in.Code)
	role.Status = in.Status
	role.Sort = in.Sort
	role.Remark = trim(in.Remark)
	if err := s.roles.Update(ctx, role); err != nil {
		return nil, err
	}
	if in.MenuIDs != nil {
		if err := s.roles.SetMenus(ctx, role.ID, in.MenuIDs); err != nil {
			return nil, err
		}
	}
	return role, nil
}

func (s *RoleService) Delete(ctx context.Context, id uint64) error {
	role, err := s.roles.Get(ctx, id)
	if err != nil {
		if IsNotFound(err) {
			return apperr.ErrNotFound
		}
		return err
	}
	if role.Code == "super_admin" {
		return apperr.New(apperr.CodeForbidden, "内置超级管理员不可删除")
	}
	n, err := s.roles.CountUsers(ctx, id)
	if err != nil {
		return err
	}
	if n > 0 {
		return apperr.New(apperr.CodeConflict, "角色下仍有用户，无法删除")
	}
	return s.roles.Delete(ctx, id)
}

func (s *RoleService) Menus(ctx context.Context, roleID uint64) ([]uint64, error) {
	if _, err := s.roles.Get(ctx, roleID); err != nil {
		if IsNotFound(err) {
			return nil, apperr.ErrNotFound
		}
		return nil, err
	}
	return s.menus.MenuIDsByRole(ctx, roleID)
}

func (s *RoleService) AssignMenus(ctx context.Context, roleID uint64, menuIDs []uint64) error {
	if _, err := s.roles.Get(ctx, roleID); err != nil {
		if IsNotFound(err) {
			return apperr.ErrNotFound
		}
		return err
	}
	return s.roles.SetMenus(ctx, roleID, menuIDs)
}

func (s *RoleService) validate(ctx context.Context, in RoleInput, excludeID uint64) error {
	if trim(in.Name) == "" {
		return apperr.New(apperr.CodeInvalidArgument, "角色名称不能为空")
	}
	if err := ValidateRoleCode(trim(in.Code)); err != nil {
		return err
	}
	if err := ValidateStatus(in.Status); err != nil {
		return err
	}
	exists, err := s.roles.ExistsCode(ctx, trim(in.Code), excludeID)
	if err != nil {
		return err
	}
	if exists {
		return apperr.New(apperr.CodeConflict, "角色编码已存在")
	}
	return nil
}
