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

// CreateUserInput 是创建用户的输入。
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

// UpdateUserInput 是更新用户的输入。
// RoleIDs 使用指针区分“不修改角色”和“清空角色”。
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

// UserImportRow 是 Excel 导入的一行用户数据，字段均为原始文本。
type UserImportRow struct {
	Username string
	Password string
	Nickname string
	Email    string
	Phone    string
	DeptID   string
	Status   string
}

// ImportFailure 描述导入失败的行号、用户名和原因。
type ImportFailure struct {
	Row      int    `json:"row"`
	Username string `json:"username"`
	Message  string `json:"message"`
}

// ImportResult 汇总导入结果：成功条数与失败明细。
type ImportResult struct {
	Success int             `json:"success"`
	Failed  []ImportFailure `json:"failed"`
}

// UserService 实现用户管理、导入导出与相关通知。
type UserService struct {
	users       UserRepository
	roles       RoleRepository
	depts       DepartmentRepository
	hasher      security.PasswordHasher
	passwordMin int
	notifier    Notifier
}

// NewUserService 创建用户服务。passwordMin < 8 时按 8 处理。
func NewUserService(users UserRepository, roles RoleRepository, depts DepartmentRepository, hasher security.PasswordHasher, passwordMin int, notifier Notifier) *UserService {
	if passwordMin < 8 {
		passwordMin = 8
	}
	return &UserService{users: users, roles: roles, depts: depts, hasher: hasher, passwordMin: passwordMin, notifier: notifier}
}

// checkDept 校验部门是否存在；deptID 为 nil 或 0 表示不分配部门。
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

// checkRoles 去重后逐个校验角色是否存在。
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

// List 分页查询用户。
func (s *UserService) List(ctx context.Context, f model.UserFilter) ([]model.User, int64, error) {
	f.Normalize()
	return s.users.List(ctx, f)
}

// Get 查询用户详情，记录不存在时统一转换为业务 404。
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

// Create 创建用户：校验字段与关联、哈希密码、写入记录并绑定角色。
// 角色绑定失败时会回滚刚创建的用户，避免留下半成品账号。
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
	notify(ctx, s.notifier, NotificationInput{
		UserID:  user.ID,
		Title:   "欢迎使用 X-ADMIN",
		Content: "账号已创建，请尽快完善个人资料并修改初始密码。",
		Type:    model.NotificationSuccess,
		Link:    "/profile",
	})
	return s.Get(ctx, user.ID)
}

// Update 更新用户资料、状态、部门和角色。
// 状态或角色发生变化时会给用户发送站内通知。
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
	previousStatus := user.Status
	if in.Status != nil {
		if err := ValidateStatus(*in.Status); err != nil {
			return nil, err
		}
		// 内置 admin 是最后的兜底账号，禁止停用。
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
		s.notifyRoleChange(ctx, id)
	}
	if user.Status != previousStatus {
		s.notifyStatusChange(ctx, id, user.Status, 0)
	}
	return s.Get(ctx, id)
}

// SetStatus 启用或停用用户。
// 不允许停用内置 admin，也不允许操作者停用自己。
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
	if err := s.users.UpdateFields(ctx, id, map[string]any{"status": status}); err != nil {
		return err
	}
	s.notifyStatusChange(ctx, id, status, actorID)
	return nil
}

// notifyStatusChange 将管理员的状态变更通知给受影响账号。
// 操作者对自己执行的操作不会产生通知，避免打扰。
func (s *UserService) notifyStatusChange(ctx context.Context, id uint64, status int8, actorID uint64) {
	if id == actorID {
		return
	}
	in := NotificationInput{
		UserID: id, ActorID: actorID, Link: "/profile",
		Title: "账号状态变更", Type: model.NotificationSuccess,
		Content: "管理员已恢复你的账号，现在可以正常登录。",
	}
	if status != model.StatusEnabled {
		in.Title = "账号已停用"
		in.Type = model.NotificationWarning
		in.Content = "管理员已停用你的账号，如有疑问请联系管理员。"
	}
	notify(ctx, s.notifier, in)
}

// notifyRoleChange 通知用户角色权限已调整。
func (s *UserService) notifyRoleChange(ctx context.Context, id uint64) {
	notify(ctx, s.notifier, NotificationInput{
		UserID: id, Link: "/profile",
		Title: "角色权限已更新", Type: model.NotificationInfo,
		Content: "管理员调整了你的角色权限，重新登录后生效。",
	})
}

// ResetPassword 管理员重置指定用户的密码。
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
	if err := s.users.UpdateFields(ctx, id, map[string]any{"password": hash}); err != nil {
		return err
	}
	notify(ctx, s.notifier, NotificationInput{
		UserID: id, Link: "/profile",
		Title: "密码已重置", Type: model.NotificationWarning,
		Content: "管理员已重置你的登录密码，请使用新密码登录并尽快修改。",
	})
	return nil
}

// Delete 批量删除用户；逐个体检内置 admin 和当前登录账号。
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

// AssignRoles 整体替换用户的角色。
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
	if err := s.users.SetRoles(ctx, id, roleIDs); err != nil {
		return err
	}
	s.notifyRoleChange(ctx, id)
	return nil
}

// Import 逐行导入用户。
// 单行失败不会中断整体导入，失败行号按 Excel 实际行号（表头为第 1 行）返回。
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

// Export 按筛选条件导出全部用户（忽略分页）。
func (s *UserService) Export(ctx context.Context, f model.UserFilter) ([]byte, error) {
	// PageSize=0 表示不分页，导出当前筛选下的所有数据。
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

// Template 生成用户导入模板（含一行示例数据）。
func (s *UserService) Template() ([]byte, error) {
	return excel.Write("用户", userImportHeaders(), [][]string{{
		"zhangsan", "张三", "zhangsan@example.com", "13800000000", "2", "技术部", "12345678a", "1",
	}})
}

// userExportHeaders 返回导出文件的表头。
func userExportHeaders() []string {
	return []string{"用户名", "昵称", "邮箱", "手机号", "部门", "角色", "状态", "最后登录", "创建时间"}
}

// userImportHeaders 返回导入模板的表头。
func userImportHeaders() []string {
	return []string{"用户名", "昵称", "邮箱", "手机号", "部门ID", "部门名称", "密码", "状态(1正常/0停用)"}
}

// ImportRowFromCells 把不含表头的表格行映射为 UserImportRow。
// 模板列顺序：用户名、昵称、邮箱、手机号、部门ID、部门名称、密码、状态。
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
