package service

import (
	"context"

	"x-admin/internal/model"
	"x-admin/internal/pkg/apperr"
	"x-admin/internal/pkg/tree"
)

type DepartmentNode struct {
	model.Department
	Children []*DepartmentNode `json:"children,omitempty"`
}

type DepartmentInput struct {
	ParentID uint64 `json:"parent_id"`
	Name     string `json:"name"`
	Leader   string `json:"leader"`
	Phone    string `json:"phone"`
	Email    string `json:"email"`
	Sort     int    `json:"sort"`
	Status   int8   `json:"status"`
}

type DepartmentService struct {
	depts DepartmentRepository
}

func NewDepartmentService(depts DepartmentRepository) *DepartmentService {
	return &DepartmentService{depts: depts}
}

func (s *DepartmentService) Tree(ctx context.Context) ([]*DepartmentNode, error) {
	rows, err := s.depts.All(ctx)
	if err != nil {
		return nil, err
	}
	nodes := make([]*DepartmentNode, 0, len(rows))
	for _, d := range rows {
		nodes = append(nodes, &DepartmentNode{Department: d})
	}
	return tree.Build(nodes, tree.Options[*DepartmentNode]{
		ID:       func(n *DepartmentNode) uint64 { return n.ID },
		ParentID: func(n *DepartmentNode) uint64 { return n.ParentID },
		SetChildren: func(n *DepartmentNode, kids []*DepartmentNode) {
			n.Children = kids
		},
	}), nil
}

func (s *DepartmentService) Create(ctx context.Context, in DepartmentInput) (*model.Department, error) {
	if err := s.validate(ctx, in, 0); err != nil {
		return nil, err
	}
	dept := &model.Department{
		ParentID: in.ParentID, Name: trim(in.Name), Leader: trim(in.Leader),
		Phone: trim(in.Phone), Email: trim(in.Email), Sort: in.Sort, Status: in.Status,
	}
	if err := s.depts.Create(ctx, dept); err != nil {
		return nil, err
	}
	return dept, nil
}

func (s *DepartmentService) Update(ctx context.Context, id uint64, in DepartmentInput) (*model.Department, error) {
	dept, err := s.depts.Get(ctx, id)
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
		return nil, apperr.New(apperr.CodeInvalidArgument, "上级部门不能是自己")
	}
	all, err := s.depts.All(ctx)
	if err != nil {
		return nil, err
	}
	if isDeptDescendant(all, in.ParentID, id) {
		return nil, apperr.New(apperr.CodeInvalidArgument, "上级部门不能是自己的子级")
	}
	dept.ParentID = in.ParentID
	dept.Name = trim(in.Name)
	dept.Leader = trim(in.Leader)
	dept.Phone = trim(in.Phone)
	dept.Email = trim(in.Email)
	dept.Sort = in.Sort
	dept.Status = in.Status
	if err := s.depts.Update(ctx, dept); err != nil {
		return nil, err
	}
	return dept, nil
}

func (s *DepartmentService) Delete(ctx context.Context, id uint64) error {
	if _, err := s.depts.Get(ctx, id); err != nil {
		if IsNotFound(err) {
			return apperr.ErrNotFound
		}
		return err
	}
	n, err := s.depts.CountChildren(ctx, id)
	if err != nil {
		return err
	}
	if n > 0 {
		return apperr.New(apperr.CodeConflict, "请先删除子部门")
	}
	users, err := s.depts.CountUsers(ctx, id)
	if err != nil {
		return err
	}
	if users > 0 {
		return apperr.New(apperr.CodeConflict, "部门下仍有用户，无法删除")
	}
	return s.depts.Delete(ctx, id)
}

func (s *DepartmentService) validate(ctx context.Context, in DepartmentInput, excludeID uint64) error {
	if trim(in.Name) == "" {
		return apperr.New(apperr.CodeInvalidArgument, "部门名称不能为空")
	}
	if err := ValidateEmail(trim(in.Email)); err != nil {
		return err
	}
	if err := ValidatePhone(trim(in.Phone)); err != nil {
		return err
	}
	if err := ValidateStatus(in.Status); err != nil {
		return err
	}
	if in.ParentID > 0 && in.ParentID != excludeID {
		if _, err := s.depts.Get(ctx, in.ParentID); err != nil {
			if IsNotFound(err) {
				return apperr.New(apperr.CodeInvalidArgument, "上级部门不存在")
			}
			return err
		}
	}
	return nil
}

func isDeptDescendant(all []model.Department, candidate, ancestor uint64) bool {
	if candidate == 0 {
		return false
	}
	parent := make(map[uint64]uint64, len(all))
	for _, d := range all {
		parent[d.ID] = d.ParentID
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
