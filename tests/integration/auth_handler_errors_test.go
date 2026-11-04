package integration_test

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"x-admin/tests/testsupport"
)

func TestAuthHandlerErrorBranches(t *testing.T) {
	app := testsupport.New(t)
	app.SeedAdmin(t)

	malformed := []string{
		"/admin/v1/auth/login",
		"/admin/v1/auth/login/mfa",
		"/admin/v1/auth/passkey/login/finish",
		"/admin/v1/auth/refresh",
	}
	for _, path := range malformed {
		rr := app.Do(http.MethodPost, path, []byte("{not-json"), "")
		env := testsupport.Decode(t, rr)
		assert.NotZero(t, env.Code, path)
		require.NotEqual(t, 500, rr.Code, path)
	}

	rr := app.Do(http.MethodPost, "/admin/v1/auth/login/mfa", map[string]string{
		"mfa_token": "missing", "otp_code": "123456",
	}, "")
	assert.Equal(t, 40102, testsupport.Decode(t, rr).Code)

	rr = app.Do(http.MethodPost, "/admin/v1/auth/passkey/login/finish", map[string]any{
		"session_id": "missing", "credential": map[string]any{"id": "x"},
	}, "")
	assert.NotZero(t, testsupport.Decode(t, rr).Code)

	rr = app.Do(http.MethodPost, "/admin/v1/auth/refresh", map[string]string{"refresh_token": "missing"}, "")
	assert.Equal(t, 40102, testsupport.Decode(t, rr).Code)
}
