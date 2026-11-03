package service

import (
	"context"

	"x-admin/internal/model"
	"x-admin/internal/pkg/apperr"
	"x-admin/internal/pkg/tree"
)

// MenuNode 是菜单树的节点，内嵌菜单数据并附带子节点。
type MenuNode struct {
	model.Menu
	Children []*MenuNode `json:"children,omitempty"`
}

// MenuInput 是创建/更新菜单的输入。
type MenuInput struct {
	ParentID   uint64 `json:"parent_id"`
	Name       string `json:"name"`
	Type       int8   `json:"type"` // 1 目录 / 2 菜单 / 3 按钮
	Path       string `json:"path"`
	Component  string `json:"component"`
	Permission string `json:"permission"`
	Icon       string `json:"icon"`
	Sort       int    `json:"sort"`
	Visible    bool   `json:"visible"`
	Status     int8   `json:"status"`
}

// MenuService 实现菜单树维护与校验。
type MenuService struct {
	menus MenuRepository
}

// NewMenuService 创建菜单服务。
func NewMenuService(menus MenuRepository) *MenuService {
	return &MenuService{menus: menus}
}

// Tree 返回完整菜单树（含按钮节点），供菜单管理页使用。
func (s *MenuService) Tree(ctx context.Context) ([]*MenuNode, error) {
	menus, err := s.menus.All(ctx)
	if err != nil {
		return nil, err
	}
	return buildMenuTree(menus), nil
}

// Create 校验并创建菜单节点。
func (s *MenuService) Create(ctx context.Context, in MenuInput) (*model.Menu, error) {
	if err := s.validate(ctx, in, 0); err != nil {
		return nil, err
	}
	menu := &model.Menu{
		ParentID: in.ParentID, Name: trim(in.Name), Type: in.Type, Path: trim(in.Path),
		Component: trim(in.Component), Permission: trim(in.Permission), Icon: trim(in.Icon),
		Sort: in.Sort, Visible: in.Visible, Status: in.Status,
	}
	if err := s.menus.Create(ctx, menu); err != nil {
		return nil, err
	}
	return menu, nil
}

// Update 更新菜单节点，并阻止形成自引用或环。
func (s *MenuService) Update(ctx context.Context, id uint64, in MenuInput) (*model.Menu, error) {
	menu, err := s.menus.Get(ctx, id)
	if err != nil {
		if IsNotFound(err) {
			return nil, apperr.ErrNotFound
		}
		return nil, err
	}
	if err := s.validate(ctx, in, id); err != nil {
		return nil, err
	}
	if in.ParentID == id {
		return nil, apperr.New(apperr.CodeInvalidArgument, "上级菜单不能是自己")
	}
	all, err := s.menus.All(ctx)
	if err != nil {
		return nil, err
	}
	if isDescendant(all, in.ParentID, id) {
		return nil, apperr.New(apperr.CodeInvalidArgument, "上级菜单不能是自己的子级")
	}
	menu.ParentID = in.ParentID
	menu.Name = trim(in.Name)
	menu.Type = in.Type
	menu.Path = trim(in.Path)
	menu.Component = trim(in.Component)
	menu.Permission = trim(in.Permission)
	menu.Icon = trim(in.Icon)
	menu.Sort = in.Sort
	menu.Visible = in.Visible
	menu.Status = in.Status
	if err := s.menus.Update(ctx, menu); err != nil {
		return nil, err
	}
	return menu, nil
}

// Delete 删除菜单；存在子节点时返回冲突错误。
func (s *MenuService) Delete(ctx context.Context, id uint64) error {
	if _, err := s.menus.Get(ctx, id); err != nil {
		if IsNotFound(err) {
			return apperr.ErrNotFound
		}
		return err
	}
	n, err := s.menus.CountChildren(ctx, id)
	if err != nil {
		return err
	}
	if n > 0 {
		return apperr.New(apperr.CodeConflict, "请先删除子菜单")
	}
	return s.menus.Delete(ctx, id)
}

// validate 校验菜单名称、类型、路径/权限码以及上级菜单是否存在。
func (s *MenuService) validate(ctx context.Context, in MenuInput, excludeID uint64) error {
	if trim(in.Name) == "" {
		return apperr.New(apperr.CodeInvalidArgument, "菜单名称不能为空")
	}
	if in.Type < model.MenuDirectory || in.Type > model.MenuButton {
		return apperr.New(apperr.CodeInvalidArgument, "菜单类型不合法")
	}
	// 菜单必须有前端路由路径；按钮必须提供权限码。
	if in.Type == model.MenuMenu && trim(in.Path) == "" {
		return apperr.New(apperr.CodeInvalidArgument, "菜单路径不能为空")
	}
	if in.Type == model.MenuButton && trim(in.Permission) == "" {
		return apperr.New(apperr.CodeInvalidArgument, "按钮权限码不能为空")
	}
	if err := ValidatePermission(trim(in.Permission)); err != nil {
		return err
	}
	if in.ParentID > 0 && in.ParentID != excludeID {
		if _, err := s.menus.Get(ctx, in.ParentID); err != nil {
			if IsNotFound(err) {
				return apperr.New(apperr.CodeInvalidArgument, "上级菜单不存在")
			}
			return err
		}
	}
	if err := ValidateStatus(in.Status); err != nil {
		return err
	}
	return nil
}

// buildMenuTree 把扁平菜单列表转换为树。
func buildMenuTree(menus []model.Menu) []*MenuNode {
	nodes := make([]*MenuNode, 0, len(menus))
	for _, m := range menus {
		nodes = append(nodes, &MenuNode{Menu: m})
	}
	return tree.Build(nodes, tree.Options[*MenuNode]{
		ID:       func(n *MenuNode) uint64 { return n.ID },
		ParentID: func(n *MenuNode) uint64 { return n.ParentID },
		SetChildren: func(n *MenuNode, kids []*MenuNode) {
			n.Children = kids
		},
	})
}

// isDescendant 判断 candidate 是否位于 ancestor 的子树中。
// 最多向上追溯 len(all) 步，即使数据存在环也能安全退出。
func isDescendant(all []model.Menu, candidate, ancestor uint64) bool {
	if candidate == 0 {
		return false
	}
	parent := make(map[uint64]uint64, len(all))
	for _, m := range all {
		parent[m.ID] = m.ParentID
	}
	seen := 0
	for id := candidate; id != 0 && seen <= len(all); seen++ {
		if id == ancestor {
			return true
		}
		id = parent[id]
	}
	return false
}
