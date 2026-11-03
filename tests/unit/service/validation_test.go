package service_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"x-admin/internal/model"
	"x-admin/internal/service"
)

func TestValidateUsername(t *testing.T) {
	assert.NoError(t, service.ValidateUsername("admin_01"))
	assert.NoError(t, service.ValidateUsername("abc"))
	assert.Error(t, service.ValidateUsername("ab"))
	assert.Error(t, service.ValidateUsername("bad-name"))
	assert.Error(t, service.ValidateUsername("用户名"))
}

func TestValidatePassword(t *testing.T) {
	assert.NoError(t, service.ValidatePassword("Abc12345", 8))
	assert.Error(t, service.ValidatePassword("Ab1", 8))
	assert.Error(t, service.ValidatePassword("abcdefgh", 8))
	assert.Error(t, service.ValidatePassword("12345678", 8))
}

func TestValidateEmailAndPhone(t *testing.T) {
	assert.NoError(t, service.ValidateEmail(""))
	assert.NoError(t, service.ValidateEmail("a@b.com"))
	assert.Error(t, service.ValidateEmail("bad"))
	assert.Error(t, service.ValidateEmail("Name <a@b.com>"))

	assert.NoError(t, service.ValidatePhone(""))
	assert.NoError(t, service.ValidatePhone("138-0000 0000"))
	assert.Error(t, service.ValidatePhone("abc"))
	assert.Error(t, service.ValidatePhone("123"))
}

func TestValidateCodes(t *testing.T) {
	assert.NoError(t, service.ValidateRoleCode("operator_1"))
	assert.Error(t, service.ValidateRoleCode("Admin"))
	assert.Error(t, service.ValidateRoleCode("a"))

	assert.NoError(t, service.ValidatePermission(""))
	assert.NoError(t, service.ValidatePermission("system:user:create"))
	assert.Error(t, service.ValidatePermission("no-colon"))
	assert.Error(t, service.ValidatePermission("System:User"))

	assert.NoError(t, service.ValidateParamKey("site.name"))
	assert.Error(t, service.ValidateParamKey("1site"))
	assert.Error(t, service.ValidateParamKey("a"))
}

func TestValidateStatusAndNotFound(t *testing.T) {
	assert.NoError(t, service.ValidateStatus(model.StatusEnabled))
	assert.NoError(t, service.ValidateStatus(model.StatusDisabled))
	assert.Error(t, service.ValidateStatus(2))
	assert.True(t, service.IsNotFound(service.ErrRecordNotFound))
	assert.False(t, service.IsNotFound(assert.AnError))
}
