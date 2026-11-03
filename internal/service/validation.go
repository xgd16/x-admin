package service

import (
	"errors"
	"net/mail"
	"regexp"
	"strings"
	"unicode"

	"x-admin/internal/pkg/apperr"
)

var (
	usernameRE = regexp.MustCompile(`^[a-zA-Z0-9_]{3,32}$`)
	roleCodeRE = regexp.MustCompile(`^[a-z][a-z0-9_]{1,31}$`)
	permRE     = regexp.MustCompile(`^[a-z][a-z0-9_]*(:[a-z][a-z0-9_]*){1,3}$`)
	paramKeyRE = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9._-]{1,127}$`)
	phoneRE    = regexp.MustCompile(`^[0-9+\-\s]{6,20}$`)
)

func ValidateUsername(v string) error {
	if !usernameRE.MatchString(v) {
		return apperr.New(apperr.CodeInvalidArgument, "用户名需为 3-32 位字母、数字或下划线")
	}
	return nil
}

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

func ValidatePhone(v string) error {
	if v == "" {
		return nil
	}
	if !phoneRE.MatchString(v) {
		return apperr.New(apperr.CodeInvalidArgument, "手机号格式不正确")
	}
	return nil
}

func ValidateRoleCode(v string) error {
	if !roleCodeRE.MatchString(v) {
		return apperr.New(apperr.CodeInvalidArgument, "角色编码需为小写字母开头的 2-32 位字符")
	}
	return nil
}

func ValidatePermission(v string) error {
	if v == "" {
		return nil
	}
	if !permRE.MatchString(v) {
		return apperr.New(apperr.CodeInvalidArgument, "权限码格式应为模块:资源:动作")
	}
	return nil
}

func ValidateParamKey(v string) error {
	if !paramKeyRE.MatchString(v) {
		return apperr.New(apperr.CodeInvalidArgument, "参数键需以字母开头，可包含字母、数字、点、下划线、短横线")
	}
	return nil
}

func ValidateStatus(v int8) error {
	if v != 0 && v != 1 {
		return apperr.New(apperr.CodeInvalidArgument, "状态值不合法")
	}
	return nil
}

func IsNotFound(err error) bool {
	return errors.Is(err, ErrRecordNotFound)
}

func trim(v string) string { return strings.TrimSpace(v) }
