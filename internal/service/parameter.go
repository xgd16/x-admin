package service

import (
	"context"

	"x-admin/internal/model"
	"x-admin/internal/pkg/apperr"
	"x-admin/internal/pkg/pagination"
)

type ParameterInput struct {
	Key    string `json:"key"`
	Value  string `json:"value"`
	Name   string `json:"name"`
	Public bool   `json:"public"`
	Remark string `json:"remark"`
}

type ParameterService struct {
	params ParameterRepository
}

func NewParameterService(params ParameterRepository) *ParameterService {
	return &ParameterService{params: params}
}

func (s *ParameterService) List(ctx context.Context, q pagination.Query) ([]model.Parameter, int64, error) {
	q.Normalize()
	return s.params.List(ctx, q)
}

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

func (s *ParameterService) Delete(ctx context.Context, id uint64) error {
	if _, err := s.params.Get(ctx, id); err != nil {
		if IsNotFound(err) {
			return apperr.ErrNotFound
		}
		return err
	}
	return s.params.Delete(ctx, id)
}

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
