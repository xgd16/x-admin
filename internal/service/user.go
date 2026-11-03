package service

import (
	"context"
	"strconv"
	"strings"

	"x-admin/internal/model"
	"x-admin/internal/pkg/apperr"
	"x-admin/internal/pkg/excel"
	"x-admin/internal/pkg/security"
)

type CreateUserInput struct {
	Username string   `json:"username"`
	Password string   `json:"password"`
	Nickname string   `json:"nickname"`
	Email    string   `json:"email"`
	Phone    string   `json:"phone"`
	Avatar   string   `json:"avatar"`
	DeptID   *uint64  `json:"dept_id"`
	Status   *int8    `json:"status"`
	RoleIDs  []uint64 `json:"role_ids"`
	Remark   string   `json:"remark"`
}

type UpdateUserInput struct {
	Nickname string    `json:"nickname"`
	Email    string    `json:"email"`
	Phone    string    `json:"phone"`
	Avatar   string    `json:"avatar"`
	DeptID   *uint64   `json:"dept_id"`
	Status   *int8     `json:"status"`
	RoleIDs  *[]uint64 `json:"role_ids"`
	Remark   string    `json:"remark"`
}

type UserImportRow struct {
	Username string
	Password string
	Nickname string
	Email    string
	Phone    string
	DeptID   string
	Status   string
}

type ImportFailure struct {
	Row      int    `json:"row"`
	Username string `json:"username"`
	Message  string `json:"message"`
}

type ImportResult struct {
	Success int             `json:"success"`
	Failed  []ImportFailure `json:"failed"`
}

type UserService struct {
	users       UserRepository
	roles       RoleRepository
	depts       DepartmentRepository
	hasher      security.PasswordHasher
	passwordMin int
}

func NewUserService(users UserRepository, roles RoleRepository, depts DepartmentRepository, hasher security.PasswordHasher, passwordMin int) *UserService {
	if passwordMin < 8 {
		passwordMin = 8
	}
	return &UserService{users: users, roles: roles, depts: depts, hasher: hasher, passwordMin: passwordMin}
}

func (s *UserService) checkDept(ctx context.Context, deptID *uint64) error {
	if deptID == nil || *deptID == 0 {
		return nil
	}
	if _, err := s.depts.Get(ctx, *deptID); err != nil {
		if IsNotFound(err) {
			return apperr.New(apperr.CodeInvalidArgument, "部门不存在")
		}
		return err
	}
	return nil
}

func (s *UserService) checkRoles(ctx context.Context, roleIDs []uint64) error {
	seen := make(map[uint64]struct{}, len(roleIDs))
	for _, id := range roleIDs {
		if id == 0 {
			return apperr.New(apperr.CodeInvalidArgument, "角色 ID 不合法")
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		if _, err := s.roles.Get(ctx, id); err != nil {
			if IsNotFound(err) {
				return apperr.New(apperr.CodeInvalidArgument, "角色不存在")
			}
			return err
		}
	}
	return nil
}

func (s *UserService) List(ctx context.Context, f model.UserFilter) ([]model.User, int64, error) {
	f.Normalize()
	return s.users.List(ctx, f)
}

func (s *UserService) Get(ctx context.Context, id uint64) (*model.User, error) {
	user, err := s.users.Get(ctx, id)
	if err != nil {
		if IsNotFound(err) {
			return nil, apperr.ErrNotFound
		}
		return nil, err
	}
	return user, nil
}

func (s *UserService) Create(ctx context.Context, in CreateUserInput) (*model.User, error) {
	in.Username = trim(in.Username)
	in.Nickname = trim(in.Nickname)
	if err := ValidateUsername(in.Username); err != nil {
		return nil, err
	}
	if err := ValidatePassword(in.Password, s.passwordMin); err != nil {
		return nil, err
	}
	if err := ValidateEmail(trim(in.Email)); err != nil {
		return nil, err
	}
	if err := ValidatePhone(trim(in.Phone)); err != nil {
		return nil, err
	}
	exists, err := s.users.ExistsUsername(ctx, in.Username, 0)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, apperr.New(apperr.CodeConflict, "用户名已存在")
	}
	if err := s.checkDept(ctx, in.DeptID); err != nil {
		return nil, err
	}
	if err := s.checkRoles(ctx, in.RoleIDs); err != nil {
		return nil, err
	}
	status := model.StatusEnabled
	if in.Status != nil {
		if err := ValidateStatus(*in.Status); err != nil {
			return nil, err
		}
		status = *in.Status
	}
	hash, err := s.hasher.Hash(in.Password)
	if err != nil {
		return nil, apperr.Wrap(err, apperr.CodeInternal, "密码加密失败")
	}
	user := &model.User{
		Username: in.Username, Password: hash, Nickname: in.Nickname,
		Email: trim(in.Email), Phone: trim(in.Phone), Avatar: trim(in.Avatar),
		DeptID: in.DeptID, Status: status, Remark: trim(in.Remark),
	}
	if err := s.users.Create(ctx, user); err != nil {
		return nil, err
	}
	if len(in.RoleIDs) > 0 {
		if err := s.users.SetRoles(ctx, user.ID, in.RoleIDs); err != nil {
			_ = s.users.Delete(ctx, user.ID)
			return nil, err
		}
	}
	return s.Get(ctx, user.ID)
}

func (s *UserService) Update(ctx context.Context, id uint64, in UpdateUserInput) (*model.User, error) {
	user, err := s.users.Get(ctx, id)
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
	if in.Status != nil {
		if err := ValidateStatus(*in.Status); err != nil {
			return nil, err
		}
		if user.Username == "admin" && *in.Status != model.StatusEnabled {
			return nil, apperr.New(apperr.CodeForbidden, "内置管理员不可停用")
		}
		user.Status = *in.Status
	}
	if in.DeptID != nil {
		if err := s.checkDept(ctx, in.DeptID); err != nil {
			return nil, err
		}
		user.DeptID = in.DeptID
	}
	if in.RoleIDs != nil {
		if err := s.checkRoles(ctx, *in.RoleIDs); err != nil {
			return nil, err
		}
	}
	user.Nickname = trim(in.Nickname)
	user.Email = trim(in.Email)
	user.Phone = trim(in.Phone)
	user.Avatar = trim(in.Avatar)
	user.Remark = trim(in.Remark)
	if err := s.users.Update(ctx, user); err != nil {
		return nil, err
	}
	if in.RoleIDs != nil {
		if err := s.users.SetRoles(ctx, id, *in.RoleIDs); err != nil {
			return nil, err
		}
	}
	return s.Get(ctx, id)
}

func (s *UserService) SetStatus(ctx context.Context, id uint64, status int8, actorID uint64) error {
	if err := ValidateStatus(status); err != nil {
		return err
	}
	user, err := s.users.Get(ctx, id)
	if err != nil {
		if IsNotFound(err) {
			return apperr.ErrNotFound
		}
		return err
	}
	if user.Username == "admin" && status != model.StatusEnabled {
		return apperr.New(apperr.CodeForbidden, "内置管理员不可停用")
	}
	if id == actorID && status != model.StatusEnabled {
		return apperr.New(apperr.CodeConflict, "不能停用当前登录账号")
	}
	return s.users.UpdateFields(ctx, id, map[string]any{"status": status})
}

func (s *UserService) ResetPassword(ctx context.Context, id uint64, password string) error {
	if err := ValidatePassword(password, s.passwordMin); err != nil {
		return err
	}
	if _, err := s.users.Get(ctx, id); err != nil {
		if IsNotFound(err) {
			return apperr.ErrNotFound
		}
		return err
	}
	hash, err := s.hasher.Hash(password)
	if err != nil {
		return apperr.Wrap(err, apperr.CodeInternal, "密码加密失败")
	}
	return s.users.UpdateFields(ctx, id, map[string]any{"password": hash})
}

func (s *UserService) Delete(ctx context.Context, ids []uint64, actorID uint64) error {
	if len(ids) == 0 {
		return apperr.New(apperr.CodeInvalidArgument, "请选择要删除的用户")
	}
	for _, id := range ids {
		user, err := s.users.Get(ctx, id)
		if err != nil {
			if IsNotFound(err) {
				return apperr.ErrNotFound
			}
			return err
		}
		if user.Username == "admin" {
			return apperr.New(apperr.CodeForbidden, "内置管理员不可删除")
		}
		if id == actorID {
			return apperr.New(apperr.CodeConflict, "不能删除当前登录账号")
		}
	}
	return s.users.Delete(ctx, ids...)
}

func (s *UserService) AssignRoles(ctx context.Context, id uint64, roleIDs []uint64) error {
	if _, err := s.users.Get(ctx, id); err != nil {
		if IsNotFound(err) {
			return apperr.ErrNotFound
		}
		return err
	}
	if err := s.checkRoles(ctx, roleIDs); err != nil {
		return err
	}
	return s.users.SetRoles(ctx, id, roleIDs)
}

func (s *UserService) Import(ctx context.Context, rows []UserImportRow) (*ImportResult, error) {
	if len(rows) == 0 {
		return nil, apperr.New(apperr.CodeInvalidArgument, "导入文件没有数据")
	}
	if len(rows) > 500 {
		return nil, apperr.New(apperr.CodeInvalidArgument, "单次最多导入 500 行")
	}
	result := &ImportResult{Failed: make([]ImportFailure, 0)}
	for i, row := range rows {
		status := model.StatusEnabled
		if strings.TrimSpace(row.Status) == "0" {
			status = model.StatusDisabled
		}
		var deptID *uint64
		if v := strings.TrimSpace(row.DeptID); v != "" {
			if id, err := strconv.ParseUint(v, 10, 64); err == nil {
				deptID = &id
			} else {
				result.Failed = append(result.Failed, ImportFailure{Row: i + 2, Username: row.Username, Message: "部门 ID 不合法"})
				continue
			}
		}
		_, err := s.Create(ctx, CreateUserInput{
			Username: row.Username, Password: row.Password, Nickname: row.Nickname,
			Email: row.Email, Phone: row.Phone, DeptID: deptID, Status: &status,
		})
		if err != nil {
			result.Failed = append(result.Failed, ImportFailure{Row: i + 2, Username: row.Username, Message: apperr.From(err).Message})
			continue
		}
		result.Success++
	}
	return result, nil
}

func (s *UserService) Export(ctx context.Context, f model.UserFilter) ([]byte, error) {
	f.Page = 1
	f.PageSize = 0
	users, _, err := s.users.List(ctx, f)
	if err != nil {
		return nil, err
	}
	rows := make([][]string, 0, len(users))
	for _, u := range users {
		dept := ""
		if u.Dept != nil {
			dept = u.Dept.Name
		}
		roles := make([]string, 0, len(u.Roles))
		for _, r := range u.Roles {
			roles = append(roles, r.Name)
		}
		lastLogin := ""
		if u.LastLoginAt != nil {
			lastLogin = u.LastLoginAt.Format("2006-01-02 15:04:05")
		}
		status := "停用"
		if u.Status == model.StatusEnabled {
			status = "正常"
		}
		rows = append(rows, []string{
			u.Username, u.Nickname, u.Email, u.Phone, dept, strings.Join(roles, "/"),
			status, lastLogin, u.CreatedAt.Format("2006-01-02 15:04:05"),
		})
	}
	return excel.Write("用户", userExportHeaders(), rows)
}

func (s *UserService) Template() ([]byte, error) {
	return excel.Write("用户", userImportHeaders(), [][]string{{
		"zhangsan", "张三", "zhangsan@example.com", "13800000000", "2", "技术部", "12345678a", "1",
	}})
}

func userExportHeaders() []string {
	return []string{"用户名", "昵称", "邮箱", "手机号", "部门", "角色", "状态", "最后登录", "创建时间"}
}

func userImportHeaders() []string {
	return []string{"用户名", "昵称", "邮箱", "手机号", "部门ID", "部门名称", "密码", "状态(1正常/0停用)"}
}

// ImportRowFromCells maps a spreadsheet row (without header) to UserImportRow.
// The template column order is: username, nickname, email, phone, dept_id,
// dept_name, password, status.
func ImportRowFromCells(cells []string) UserImportRow {
	at := func(i int) string {
		if i < len(cells) {
			return strings.TrimSpace(cells[i])
		}
		return ""
	}
	return UserImportRow{
		Username: at(0), Nickname: at(1), Email: at(2), Phone: at(3),
		DeptID: at(4), Password: at(6), Status: at(7),
	}
}
