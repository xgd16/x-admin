package service

import (
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/webauthn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"x-admin/internal/model"
	"x-admin/internal/pkg/apperr"
)

func TestNormalizePasskeyName(t *testing.T) {
	assert.Equal(t, "未命名设备", normalizePasskeyName(""))
	assert.Equal(t, "未命名设备", normalizePasskeyName("   "))
	assert.Equal(t, "MacBook", normalizePasskeyName("  MacBook  "))
	long := strings.Repeat("中", passkeyNameLimit+5)
	assert.Equal(t, passkeyNameLimit, len([]rune(normalizePasskeyName(long))))
}

func TestPasskeyFailureMessages(t *testing.T) {
	origin := passkeyFailure("默认提示", &protocol.Error{Details: "origin does not match"}, apperr.CodeInvalidArgument)
	assert.Contains(t, origin.Error(), "访问地址")
	rpID := passkeyFailure("默认提示", &protocol.Error{DevInfo: "relying party id mismatch"}, apperr.CodeInvalidArgument)
	assert.Contains(t, rpID.Error(), "域名配置")
	generic := passkeyFailure("默认提示", errors.New("boom"), apperr.CodeInternal)
	assert.Contains(t, generic.Error(), "默认提示")
}

func TestWAUserAdapter(t *testing.T) {
	user := &model.User{Base: model.Base{ID: 1}, Username: "alice", Nickname: "Alice"}
	credentials := []webauthn.Credential{{ID: []byte("cred")}}
	wa := newWAUser(user, credentials)
	assert.NotEmpty(t, wa.WebAuthnID())
	assert.NotEqual(t, wa.WebAuthnID(), newWAUser(&model.User{Base: model.Base{ID: 2}, Username: "bob"}, nil).WebAuthnID())
	assert.Equal(t, "alice", wa.WebAuthnName())
	assert.Equal(t, "Alice", wa.WebAuthnDisplayName())
	assert.Len(t, wa.WebAuthnCredentials(), 1)

	noNickname := newWAUser(&model.User{Username: "bob"}, nil)
	assert.Equal(t, "bob", noNickname.WebAuthnDisplayName())
}

func TestCredentialRequest(t *testing.T) {
	req := credentialRequest([]byte(`{"id":"abc"}`))
	assert.Equal(t, "POST", req.Method)
	assert.Equal(t, "application/json", req.Header.Get("Content-Type"))
	body, err := io.ReadAll(req.Body)
	require.NoError(t, err)
	assert.JSONEq(t, `{"id":"abc"}`, string(body))
}
