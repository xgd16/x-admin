package integration_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"x-admin/internal/model"
	"x-admin/internal/pkg/apperr"
	"x-admin/tests/testsupport"
)

type passkeyOptionsPayload struct {
	SessionID string `json:"session_id"`
	Options   struct {
		PublicKey struct {
			Challenge string `json:"challenge"`
			User      struct {
				ID string `json:"id"`
			} `json:"user"`
		} `json:"publicKey"`
	} `json:"options"`
}

func TestPasskeyManagementEndpoints(t *testing.T) {
	app := testsupport.New(t)
	admin := app.SeedAdmin(t)
	token := app.Login(t, "admin", "Admin@123456").AccessToken
	ctx := context.Background()

	rr := app.Do(http.MethodGet, "/admin/v1/profile/passkeys", nil, token)
	require.Equal(t, 0, testsupport.Decode(t, rr).Code)
	var list []model.Passkey
	require.NoError(t, json.Unmarshal(testsupport.Decode(t, rr).Data, &list))
	assert.Empty(t, list)

	// Registration is gated by the current password and starts a one-time session.
	rr = app.Do(http.MethodPost, "/admin/v1/profile/passkeys/begin", map[string]string{"password": "wrong"}, token)
	assert.Equal(t, 40001, testsupport.Decode(t, rr).Code)

	rr = app.Do(http.MethodPost, "/admin/v1/profile/passkeys/begin", map[string]string{"password": "Admin@123456"}, token)
	require.Equal(t, 0, testsupport.Decode(t, rr).Code, rr.Body.String())
	var begin passkeyOptionsPayload
	require.NoError(t, json.Unmarshal(testsupport.Decode(t, rr).Data, &begin))
	require.NotEmpty(t, begin.SessionID)
	require.NotEmpty(t, begin.Options.PublicKey.Challenge)
	require.NotEmpty(t, begin.Options.PublicKey.User.ID)
	_, err := app.Cache.Get(ctx, "passkey:register:"+begin.SessionID)
	require.NoError(t, err)

	// A malformed authenticator response fails and consumes the challenge.
	rr = app.Do(http.MethodPost, "/admin/v1/profile/passkeys/finish", map[string]any{
		"session_id": begin.SessionID,
		"name":       "MacBook",
		"credential": map[string]any{
			"id": "dGVzdA", "rawId": "dGVzdA", "type": "public-key",
			"response": map[string]any{"clientDataJSON": "e30", "attestationObject": "e30"},
		},
	}, token)
	assert.Equal(t, 40001, testsupport.Decode(t, rr).Code)
	exists, err := app.Cache.Exists(ctx, "passkey:register:"+begin.SessionID)
	require.NoError(t, err)
	assert.False(t, exists)

	// Store a credential row directly to exercise list/rename/delete scoping.
	row := &model.Passkey{
		UserID: admin.ID, CredentialID: "credential-admin", Name: "旧设备",
		Credential: `{"id":"Y3JlZGVudGlhbC1hZG1pbg"}`,
	}
	require.NoError(t, app.Repos.Passkeys.Create(ctx, row))

	other := app.CreateUser(t, "other", "Admin@123456", model.StatusEnabled)
	otherRow := &model.Passkey{
		UserID: other.ID, CredentialID: "credential-other", Name: "别人的设备",
		Credential: `{"id":"Y3JlZGVudGlhbC1vdGhlcg"}`,
	}
	require.NoError(t, app.Repos.Passkeys.Create(ctx, otherRow))

	rr = app.Do(http.MethodGet, "/admin/v1/profile/passkeys", nil, token)
	require.Equal(t, 0, testsupport.Decode(t, rr).Code)
	require.NoError(t, json.Unmarshal(testsupport.Decode(t, rr).Data, &list))
	require.Len(t, list, 1)
	assert.Equal(t, "旧设备", list[0].Name)

	rr = app.Do(http.MethodPut, fmt.Sprintf("/admin/v1/profile/passkeys/%d", row.ID), map[string]string{"name": "MacBook Pro"}, token)
	require.Equal(t, 0, testsupport.Decode(t, rr).Code, rr.Body.String())
	got, err := app.Repos.Passkeys.GetForUser(ctx, admin.ID, row.ID)
	require.NoError(t, err)
	assert.Equal(t, "MacBook Pro", got.Name)

	rr = app.Do(http.MethodPut, fmt.Sprintf("/admin/v1/profile/passkeys/%d", otherRow.ID), map[string]string{"name": "越权"}, token)
	assert.Equal(t, 40401, testsupport.Decode(t, rr).Code)
	rr = app.Do(http.MethodDelete, fmt.Sprintf("/admin/v1/profile/passkeys/%d", otherRow.ID), nil, token)
	assert.Equal(t, 40401, testsupport.Decode(t, rr).Code)

	// Login begin only works once at least one passkey exists.
	rr = app.Do(http.MethodPost, "/admin/v1/auth/passkey/login/begin", nil, "")
	require.Equal(t, 0, testsupport.Decode(t, rr).Code, rr.Body.String())
	var loginBegin passkeyOptionsPayload
	require.NoError(t, json.Unmarshal(testsupport.Decode(t, rr).Data, &loginBegin))
	require.NotEmpty(t, loginBegin.SessionID)
	require.NotEmpty(t, loginBegin.Options.PublicKey.Challenge)

	rr = app.Do(http.MethodPost, "/admin/v1/auth/passkey/login/finish", map[string]any{
		"session_id": loginBegin.SessionID,
		"credential": map[string]any{
			"id": "dGVzdA", "rawId": "dGVzdA", "type": "public-key",
			"response": map[string]any{
				"clientDataJSON": "e30", "authenticatorData": "e30",
				"signature": "e30", "userHandle": "e30",
			},
		},
	}, "")
	assert.Equal(t, 40101, testsupport.Decode(t, rr).Code)

	rr = app.Do(http.MethodDelete, fmt.Sprintf("/admin/v1/profile/passkeys/%d", row.ID), nil, token)
	require.Equal(t, 0, testsupport.Decode(t, rr).Code, rr.Body.String())
	_, err = app.Repos.Passkeys.GetForUser(ctx, admin.ID, row.ID)
	assert.Error(t, err)
}

func TestPasskeyLoginNeedsRegisteredDevice(t *testing.T) {
	app := testsupport.New(t)
	app.SeedAdmin(t)

	rr := app.Do(http.MethodPost, "/admin/v1/auth/passkey/login/begin", nil, "")
	assert.Equal(t, 40401, testsupport.Decode(t, rr).Code)
}

func TestPasskeyServiceLimitsDevices(t *testing.T) {
	app := testsupport.New(t)
	admin := app.SeedAdmin(t)
	ctx := context.Background()
	for i := 0; i < 10; i++ {
		require.NoError(t, app.Repos.Passkeys.Create(ctx, &model.Passkey{
			UserID: admin.ID, CredentialID: "cred-" + string(rune('a'+i)), Name: "device", Credential: "{}",
		}))
	}
	_, err := app.Services.Passkeys.BeginRegistration(ctx, admin.ID, "Admin@123456")
	assert.Equal(t, apperr.CodeConflict, apperr.From(err).Code)
}
