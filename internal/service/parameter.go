package service

import (
	"context"

	"x-admin/internal/model"
	"x-admin/internal/pkg/apperr"
	"x-admin/internal/pkg/pagination"
)

// ParameterInput 是创建/更新系统参数的输入。
type ParameterInput struct {
	Key    string `json:"key"`
	Value  string `json:"value"`
	Name   string `json:"name"`
	Public bool   `json:"public"`
	Remark string `json:"remark"`
}

// ParameterService 实现系统参数的维护与读取。
type ParameterService struct {
	params ParameterRepository
}

// NewParameterService 创建参数服务。
func NewParameterService(params ParameterRepository) *ParameterService {
	return &ParameterService{params: params}
}

// List 分页查询系统参数。
func (s *ParameterService) List(ctx context.Context, q pagination.Query) ([]model.Parameter, int64, error) {
	q.Normalize()
	return s.params.List(ctx, q)
}

// Public 返回全部公开参数，供登录页/品牌配置等未登录场景读取。
func (s *ParameterService) Public(ctx context.Context) ([]model.Parameter, error) {
	rows, err := s.params.ListPublic(ctx)
	if err != nil {
		return nil, err
	}
	if rows == nil {
		rows = []model.Parameter{}
	}
	return rows, nil
}

// GetByKey 按 key 读取公开参数；非公开参数对外表现为不存在。
func (s *ParameterService) GetByKey(ctx context.Context, key string) (*model.Parameter, error) {
	p, err := s.params.GetByKey(ctx, key)
	if err != nil {
		if IsNotFound(err) {
			return nil, apperr.ErrNotFound
		}
		return nil, err
	}
	if !p.Public {
		return nil, apperr.ErrNotFound
	}
	return p, nil
}

// Create 校验并创建系统参数。
func (s *ParameterService) Create(ctx context.Context, in ParameterInput) (*model.Parameter, error) {
	if err := s.validate(ctx, in, 0); err != nil {
		return nil, err
	}
	p := &model.Parameter{Key: trim(in.Key), Value: in.Value, Name: trim(in.Name), Public: in.Public, Remark: trim(in.Remark)}
	if err := s.params.Create(ctx, p); err != nil {
		return nil, err
	}
	return p, nil
}

// Update 更新系统参数，Key 改动后仍需保持唯一。
func (s *ParameterService) Update(ctx context.Context, id uint64, in ParameterInput) (*model.Parameter, error) {
	p, err := s.params.Get(ctx, id)
	if err != nil {
		if IsNotFound(err) {
			return nil, apperr.ErrNotFound
		}
		return nil, err
	}
	if err := s.validate(ctx, in, id); err != nil {
		return nil, err
	}
	p.Key = trim(in.Key)
	p.Value = in.Value
	p.Name = trim(in.Name)
	p.Public = in.Public
	p.Remark = trim(in.Remark)
	if err := s.params.Update(ctx, p); err != nil {
		return nil, err
	}
	return p, nil
}

// Delete 删除系统参数。
func (s *ParameterService) Delete(ctx context.Context, id uint64) error {
	if _, err := s.params.Get(ctx, id); err != nil {
		if IsNotFound(err) {
			return apperr.ErrNotFound
		}
		return err
	}
	return s.params.Delete(ctx, id)
}

// validate 校验参数名称、Key 格式和 Key 唯一性。
func (s *ParameterService) validate(ctx context.Context, in ParameterInput, excludeID uint64) error {
	if trim(in.Name) == "" {
		return apperr.New(apperr.CodeInvalidArgument, "参数名称不能为空")
	}
	if err := ValidateParamKey(trim(in.Key)); err != nil {
		return err
	}
	exists, err := s.params.ExistsKey(ctx, trim(in.Key), excludeID)
	if err != nil {
		return err
	}
	if exists {
		return apperr.New(apperr.CodeConflict, "参数键已存在")
	}
	return nil
}
