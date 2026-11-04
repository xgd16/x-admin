package integration_test

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"x-admin/internal/model"
	"x-admin/tests/testsupport"
)

func TestPublicParametersWithoutLogin(t *testing.T) {
	app := testsupport.New(t)
	ctx := context.Background()
	require.NoError(t, app.Repos.Params.Create(ctx, &model.Parameter{
		Key: "site.name", Name: "站点名称", Value: "x-admin", Public: true,
	}))
	require.NoError(t, app.Repos.Params.Create(ctx, &model.Parameter{
		Key: "secret.key", Name: "私有参数", Value: "hidden", Public: false,
	}))

	rr := app.Do(http.MethodGet, "/admin/v1/parameters/public", nil, "")
	env := testsupport.Decode(t, rr)
	require.Equal(t, 0, env.Code)
	var rows []model.Parameter
	require.NoError(t, json.Unmarshal(env.Data, &rows))
	require.Len(t, rows, 1)
	assert.Equal(t, "site.name", rows[0].Key)
	assert.Equal(t, "x-admin", rows[0].Value)
}
