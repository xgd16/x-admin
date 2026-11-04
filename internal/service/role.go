package service

import (
	"context"

	"x-admin/internal/model"
	"x-admin/internal/pkg/apperr"
	"x-admin/internal/pkg/authn"
)

// RoleInput 是创建/更新角色的输入。MenuIDs 为 nil 表示不修改授权。
type RoleInput struct {
	Name    string   `json:"name"`
	Code    string   `json:"code"`
	Status  int8     `json:"status"`
	Sort    int      `json:"sort"`
	Remark  string   `json:"remark"`
	MenuIDs []uint64 `json:"menu_ids"`
}

// RoleService 实现角色维护与菜单授权。
type RoleService struct {
	roles RoleRepository
	menus MenuRepository
}

// NewRoleService 创建角色服务。
func NewRoleService(roles RoleRepository, menus MenuRepository) *RoleService {
	return &RoleService{roles: roles, menus: menus}
}

// List 分页查询角色。
func (s *RoleService) List(ctx context.Context, f model.RoleFilter) ([]model.Role, int64, error) {
	f.Normalize()
	return s.roles.List(ctx, f)
}

// All 返回全部启用角色。
func (s *RoleService) All(ctx context.Context) ([]model.Role, error) {
	return s.roles.All(ctx)
}

// Create 校验并创建角色，可同时写入菜单授权。
// actor 非空时，只能授予自己已经拥有的菜单权限。
func (s *RoleService) Create(ctx context.Context, in RoleInput, actor ...*authn.Principal) (*model.Role, error) {
	if err := s.validate(ctx, in, 0); err != nil {
		return nil, err
	}
	if err := s.checkMenus(ctx, firstPrincipal(actor), in.MenuIDs); err != nil {
		return nil, err
	}
	// super_admin 是权限系统中的硬编码入口，只允许种子数据创建。
	if trim(in.Code) == "super_admin" {
		return nil, apperr.New(apperr.CodeForbidden, "内置超级管理员角色不可重复创建")
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

// Update 更新角色；MenuIDs 不为 nil 时整体替换菜单授权。
func (s *RoleService) Update(ctx context.Context, id uint64, in RoleInput, actor ...*authn.Principal) (*model.Role, error) {
	role, err := s.roles.Get(ctx, id)
	if err != nil {
		if IsNotFound(err) {
			return nil, apperr.ErrNotFound
		}
		return nil, err
	}
	if err := s.checkManageSuper(role, firstPrincipal(actor)); err != nil {
		return nil, err
	}
	if err := s.validate(ctx, in, id); err != nil {
		return nil, err
	}
	if err := s.checkMenus(ctx, firstPrincipal(actor), in.MenuIDs); err != nil {
		return nil, err
	}
	// super_admin 是权限判断的硬编码入口，编码不允许修改，也不允许停用。
	// 停用后所有超级管理员会同时失去权限，且无法再进入角色管理恢复。
	if role.Code == "super_admin" {
		if trim(in.Code) != role.Code {
			return nil, apperr.New(apperr.CodeConflict, "内置超级管理员编码不可修改")
		}
		if in.Status != model.StatusEnabled {
			return nil, apperr.New(apperr.CodeForbidden, "内置超级管理员角色不可停用")
		}
	} else if trim(in.Code) == "super_admin" {
		return nil, apperr.New(apperr.CodeForbidden, "内置超级管理员编码不可占用")
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

// Delete 删除角色；内置超级管理员或仍有用户绑定时会被拒绝。
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

// Menus 返回角色已授权的菜单 ID。
func (s *RoleService) Menus(ctx context.Context, roleID uint64) ([]uint64, error) {
	if _, err := s.roles.Get(ctx, roleID); err != nil {
		if IsNotFound(err) {
			return nil, apperr.ErrNotFound
		}
		return nil, err
	}
	return s.menus.MenuIDsByRole(ctx, roleID)
}

// AssignMenus 整体替换角色的菜单授权。
// 非超级管理员只能授予自己已经拥有的权限，避免角色权限被横向放大。
func (s *RoleService) AssignMenus(ctx context.Context, roleID uint64, menuIDs []uint64, actor ...*authn.Principal) error {
	role, err := s.roles.Get(ctx, roleID)
	if err != nil {
		if IsNotFound(err) {
			return apperr.ErrNotFound
		}
		return err
	}
	if err := s.checkManageSuper(role, firstPrincipal(actor)); err != nil {
		return err
	}
	if err := s.checkMenus(ctx, firstPrincipal(actor), menuIDs); err != nil {
		return err
	}
	return s.roles.SetMenus(ctx, roleID, menuIDs)
}

// checkMenus 校验 actor 是否拥有待授予菜单对应的全部权限。
// 空权限节点（目录、个人中心等）不产生权限码，允许授权。
func (s *RoleService) checkMenus(ctx context.Context, actor *authn.Principal, menuIDs []uint64) error {
	if actor == nil || actor.Super || len(menuIDs) == 0 {
		return nil
	}
	all, err := s.menus.All(ctx)
	if err != nil {
		return err
	}
	allowed := make(map[uint64]struct{}, len(all))
	for _, menu := range all {
		if menu.Permission == "" || actor.Has(menu.Permission) {
			allowed[menu.ID] = struct{}{}
		}
	}
	for _, id := range menuIDs {
		if _, ok := allowed[id]; !ok {
			return apperr.New(apperr.CodeForbidden, "不能授予自己未拥有的菜单权限")
		}
	}
	return nil
}

// checkManageSuper 限制非超级管理员修改内置超级管理员角色。
func (s *RoleService) checkManageSuper(role *model.Role, actor *authn.Principal) error {
	if role.Code != "super_admin" || actor == nil || actor.Super {
		return nil
	}
	return apperr.New(apperr.CodeForbidden, "只有超级管理员可以修改内置超级管理员角色")
}

// firstPrincipal 读取可选的操作者身份。
func firstPrincipal(actors []*authn.Principal) *authn.Principal {
	if len(actors) == 0 {
		return nil
	}
	return actors[0]
}

// validate 校验角色名称、编码格式、状态和编码唯一性。
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
