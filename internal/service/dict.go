package service

import (
	"context"
	"regexp"

	"x-admin/internal/model"
	"x-admin/internal/pkg/apperr"
	"x-admin/internal/pkg/pagination"
)

// dictCodeRE 限制字典编码：小写字母开头，2-64 位小写字母、数字或下划线。
var dictCodeRE = regexp.MustCompile(`^[a-z][a-z0-9_]{1,63}$`)

// DictTypeInput 是创建/更新字典类型的输入。
type DictTypeInput struct {
	Name   string `json:"name"`
	Code   string `json:"code"`
	Status int8   `json:"status"`
	Remark string `json:"remark"`
}

// DictItemInput 是创建/更新字典项的输入。
type DictItemInput struct {
	DictTypeID uint64 `json:"dict_type_id"`
	Label      string `json:"label"`
	Value      string `json:"value"`
	Sort       int    `json:"sort"`
	Status     int8   `json:"status"`
	TagColor   string `json:"tag_color"`
}

// DictService 实现字典类型与字典项的维护。
type DictService struct {
	dicts DictRepository
}

// NewDictService 创建字典服务。
func NewDictService(dicts DictRepository) *DictService { return &DictService{dicts: dicts} }

// ListTypes 分页查询字典类型。
func (s *DictService) ListTypes(ctx context.Context, q pagination.Query, status *int8) ([]model.DictType, int64, error) {
	q.Normalize()
	return s.dicts.ListTypes(ctx, q, status)
}

// CreateType 校验并创建字典类型。
func (s *DictService) CreateType(ctx context.Context, in DictTypeInput) (*model.DictType, error) {
	if err := s.validateType(ctx, in, 0); err != nil {
		return nil, err
	}
	t := &model.DictType{Name: trim(in.Name), Code: trim(in.Code), Status: in.Status, Remark: trim(in.Remark)}
	if err := s.dicts.CreateType(ctx, t); err != nil {
		return nil, err
	}
	return t, nil
}

// UpdateType 更新字典类型，编码仍需保持唯一。
func (s *DictService) UpdateType(ctx context.Context, id uint64, in DictTypeInput) (*model.DictType, error) {
	t, err := s.dicts.GetType(ctx, id)
	if err != nil {
		if IsNotFound(err) {
			return nil, apperr.ErrNotFound
		}
		return nil, err
	}
	if err := s.validateType(ctx, in, id); err != nil {
		return nil, err
	}
	t.Name = trim(in.Name)
	t.Code = trim(in.Code)
	t.Status = in.Status
	t.Remark = trim(in.Remark)
	if err := s.dicts.UpdateType(ctx, t); err != nil {
		return nil, err
	}
	return t, nil
}

// DeleteType 删除字典类型及其全部字典项。
func (s *DictService) DeleteType(ctx context.Context, id uint64) error {
	if _, err := s.dicts.GetType(ctx, id); err != nil {
		if IsNotFound(err) {
			return apperr.ErrNotFound
		}
		return err
	}
	return s.dicts.DeleteType(ctx, id)
}

// Items 返回指定类型下的字典项，类型不存在时返回 404。
func (s *DictService) Items(ctx context.Context, typeID uint64) ([]model.DictItem, error) {
	if _, err := s.dicts.GetType(ctx, typeID); err != nil {
		if IsNotFound(err) {
			return nil, apperr.ErrNotFound
		}
		return nil, err
	}
	return s.dicts.Items(ctx, typeID)
}

// CreateItem 校验并创建字典项。
func (s *DictService) CreateItem(ctx context.Context, in DictItemInput) (*model.DictItem, error) {
	if err := s.validateItem(ctx, in); err != nil {
		return nil, err
	}
	item := &model.DictItem{
		DictTypeID: in.DictTypeID, Label: trim(in.Label), Value: trim(in.Value),
		Sort: in.Sort, Status: in.Status, TagColor: trim(in.TagColor),
	}
	if err := s.dicts.CreateItem(ctx, item); err != nil {
		return nil, err
	}
	return item, nil
}

// UpdateItem 更新字典项。
func (s *DictService) UpdateItem(ctx context.Context, id uint64, in DictItemInput) (*model.DictItem, error) {
	item, err := s.dicts.GetItem(ctx, id)
	if err != nil {
		if IsNotFound(err) {
			return nil, apperr.ErrNotFound
		}
		return nil, err
	}
	if err := s.validateItem(ctx, in); err != nil {
		return nil, err
	}
	item.DictTypeID = in.DictTypeID
	item.Label = trim(in.Label)
	item.Value = trim(in.Value)
	item.Sort = in.Sort
	item.Status = in.Status
	item.TagColor = trim(in.TagColor)
	if err := s.dicts.UpdateItem(ctx, item); err != nil {
		return nil, err
	}
	return item, nil
}

// DeleteItem 删除字典项。
func (s *DictService) DeleteItem(ctx context.Context, id uint64) error {
	if _, err := s.dicts.GetItem(ctx, id); err != nil {
		if IsNotFound(err) {
			return apperr.ErrNotFound
		}
		return err
	}
	return s.dicts.DeleteItem(ctx, id)
}

// validateType 校验字典名称、编码格式、状态和编码唯一性。
func (s *DictService) validateType(ctx context.Context, in DictTypeInput, excludeID uint64) error {
	if trim(in.Name) == "" {
		return apperr.New(apperr.CodeInvalidArgument, "字典名称不能为空")
	}
	if !dictCodeRE.MatchString(trim(in.Code)) {
		return apperr.New(apperr.CodeInvalidArgument, "字典编码需为小写字母开头的 2-64 位字符")
	}
	if err := ValidateStatus(in.Status); err != nil {
		return err
	}
	existing, err := s.dicts.FindTypeByCode(ctx, trim(in.Code))
	if err != nil && !IsNotFound(err) {
		return err
	}
	if err == nil && existing.ID != excludeID {
		return apperr.New(apperr.CodeConflict, "字典编码已存在")
	}
	return nil
}

// validateItem 校验字典项的必填字段、状态以及所属字典类型是否存在。
func (s *DictService) validateItem(ctx context.Context, in DictItemInput) error {
	if in.DictTypeID == 0 {
		return apperr.New(apperr.CodeInvalidArgument, "请选择字典类型")
	}
	if trim(in.Label) == "" || trim(in.Value) == "" {
		return apperr.New(apperr.CodeInvalidArgument, "字典标签和值不能为空")
	}
	if err := ValidateStatus(in.Status); err != nil {
		return err
	}
	if _, err := s.dicts.GetType(ctx, in.DictTypeID); err != nil {
		if IsNotFound(err) {
			return apperr.New(apperr.CodeInvalidArgument, "字典类型不存在")
		}
		return err
	}
	return nil
}
