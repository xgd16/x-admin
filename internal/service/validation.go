package service

import (
	"errors"
	"net/mail"
	"regexp"
	"strings"
	"unicode"

	"x-admin/internal/pkg/apperr"
)

// 各字段的格式约束。正则只做结构校验，业务唯一性仍由仓储查询保证。
var (
	usernameRE = regexp.MustCompile(`^[a-zA-Z0-9_]{3,32}$`)
	roleCodeRE = regexp.MustCompile(`^[a-z][a-z0-9_]{1,31}$`)
	permRE     = regexp.MustCompile(`^[a-z][a-z0-9_]*(:[a-z][a-z0-9_]*){1,3}$`)
	paramKeyRE = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9._-]{1,127}$`)
	phoneRE    = regexp.MustCompile(`^[0-9+\-\s]{6,20}$`)
)

// ValidateUsername 校验用户名：3-32 位字母、数字或下划线。
func ValidateUsername(v string) error {
	if !usernameRE.MatchString(v) {
		return apperr.New(apperr.CodeInvalidArgument, "用户名需为 3-32 位字母、数字或下划线")
	}
	return nil
}

// ValidatePassword 校验密码长度，并要求同时包含字母和数字。
func ValidatePassword(v string, min int) error {
	if len(v) < min {
		return apperr.Newf(apperr.CodeInvalidArgument, "密码至少 %d 位", min)
	}
	var letter, digit bool
	for _, r := range v {
		if unicode.IsLetter(r) {
			letter = true
		}
		if unicode.IsDigit(r) {
			digit = true
		}
	}
	if !letter || !digit {
		return apperr.New(apperr.CodeInvalidArgument, "密码必须同时包含字母和数字")
	}
	return nil
}

// ValidateEmail 校验邮箱格式；空值视为未填写，直接通过。
func ValidateEmail(v string) error {
	if v == "" {
		return nil
	}
	parsed, err := mail.ParseAddress(v)
	if err != nil || parsed.Address != v {
		return apperr.New(apperr.CodeInvalidArgument, "邮箱格式不正确")
	}
	return nil
}

// ValidatePhone 校验手机号格式；空值视为未填写，直接通过。
func ValidatePhone(v string) error {
	if v == "" {
		return nil
	}
	if !phoneRE.MatchString(v) {
		return apperr.New(apperr.CodeInvalidArgument, "手机号格式不正确")
	}
	return nil
}

// ValidateRoleCode 校验角色编码：小写字母开头，2-32 位。
func ValidateRoleCode(v string) error {
	if !roleCodeRE.MatchString(v) {
		return apperr.New(apperr.CodeInvalidArgument, "角色编码需为小写字母开头的 2-32 位字符")
	}
	return nil
}

// ValidatePermission 校验权限码格式（模块:资源:动作）；空值表示节点没有权限码。
func ValidatePermission(v string) error {
	if v == "" {
		return nil
	}
	if !permRE.MatchString(v) {
		return apperr.New(apperr.CodeInvalidArgument, "权限码格式应为模块:资源:动作")
	}
	return nil
}

// ValidateParamKey 校验系统参数键：字母开头，可包含点、下划线和短横线。
func ValidateParamKey(v string) error {
	if !paramKeyRE.MatchString(v) {
		return apperr.New(apperr.CodeInvalidArgument, "参数键需以字母开头，可包含字母、数字、点、下划线、短横线")
	}
	return nil
}

// ValidateStatus 校验启用/禁用状态只取 0 或 1。
func ValidateStatus(v int8) error {
	if v != 0 && v != 1 {
		return apperr.New(apperr.CodeInvalidArgument, "状态值不合法")
	}
	return nil
}

// IsNotFound 判断错误是否为“记录不存在”。
func IsNotFound(err error) bool {
	return errors.Is(err, ErrRecordNotFound)
}

// trim 去除字符串首尾空白。
func trim(v string) string { return strings.TrimSpace(v) }
