package xadmin

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestCreateAPIWritesSortedConfig(t *testing.T) {
	dir := createTestProject(t)
	writeTestFile(t, dir, "configs/config.yaml", `# 本地配置
app:
  name: x-admin

# 多 API 面
api:
  merchant:
    enabled: true
    prefix: /merchant/v1
  admin: # 后台
    enabled: true
    prefix: /admin/v1

database:
  dsn: root:pass@tcp(127.0.0.1:3306)/xadmin
`)

	result, err := CreateAPI(CreateAPIOptions{Dir: dir, Name: "client", Version: "v1", Auth: true})
	if err != nil {
		t.Fatal(err)
	}
	if result.ConfigPath != "configs/config.yaml" {
		t.Fatalf("ConfigPath = %q, want configs/config.yaml", result.ConfigPath)
	}
	if len(result.NextSteps) == 0 || strings.Contains(result.NextSteps[0], "configs/config.yaml") {
		t.Fatalf("next steps should not ask for manual config: %+v", result.NextSteps)
	}

	config := readCreateFile(t, dir, "configs/config.yaml")
	want := `api:
  admin: # 后台
    enabled: true
    prefix: /admin/v1
  client:
    enabled: true
    prefix: /client/v1
    table: client_users
  merchant:
    enabled: true
    prefix: /merchant/v1
`
	if !strings.Contains(config, want) {
		t.Fatalf("api section not sorted as expected:\n%s", config)
	}
	for _, keep := range []string{"# 本地配置", "app:\n  name: x-admin", "database:\n  dsn: root:pass"} {
		if !strings.Contains(config, keep) {
			t.Fatalf("config lost %q:\n%s", keep, config)
		}
	}
}

func TestCreateAPIUpdatesExistingSurfaceConfig(t *testing.T) {
	dir := createTestProject(t)
	writeTestFile(t, dir, "configs/config.yaml", `api:
  client:
    enabled: false
    issuer: x-admin:client
    prefix: /old
`)

	if _, err := CreateAPI(CreateAPIOptions{
		Dir: dir, Name: "client", Version: "v1", Auth: true, Table: "client_accounts",
	}); err != nil {
		t.Fatal(err)
	}

	config := readCreateFile(t, dir, "configs/config.yaml")
	want := `api:
  client:
    enabled: true
    issuer: x-admin:client
    prefix: /client/v1
    table: client_accounts
`
	if !strings.Contains(config, want) {
		t.Fatalf("unexpected config:\n%s", config)
	}
	if strings.Count(config, "client:") != 1 {
		t.Fatalf("duplicate surface entry:\n%s", config)
	}
}

func TestCreateAPIConfigAppendsMissingSection(t *testing.T) {
	dir := createTestProject(t)
	writeTestFile(t, dir, "configs/config.yaml", "app:\n  name: demo\n")

	if _, err := CreateAPI(CreateAPIOptions{Dir: dir, Name: "partner", Version: "v1"}); err != nil {
		t.Fatal(err)
	}

	config := readCreateFile(t, dir, "configs/config.yaml")
	if !strings.HasPrefix(config, "app:\n  name: demo\n\napi:\n") {
		t.Fatalf("api section should be appended after existing content:\n%s", config)
	}
	if !strings.Contains(config, "  partner:\n    enabled: true\n    prefix: /partner/v1\n") {
		t.Fatalf("missing surface config:\n%s", config)
	}
}

func TestCreateAPIConfigFallbacks(t *testing.T) {
	dir := createTestProject(t)
	// 没有 configs/config.yaml 时只输出片段，提示手动添加。
	result, err := CreateAPI(CreateAPIOptions{Dir: dir, Name: "shop", Version: "v1"})
	if err != nil {
		t.Fatal(err)
	}
	if result.ConfigPath != "" {
		t.Fatalf("ConfigPath = %q, want empty without config file", result.ConfigPath)
	}
	if !strings.Contains(result.Config, "prefix: /shop/v1") {
		t.Fatalf("missing config snippet:\n%s", result.Config)
	}
	if len(result.NextSteps) == 0 || !strings.Contains(result.NextSteps[0], "configs/config.yaml") {
		t.Fatalf("next steps should mention manual config: %+v", result.NextSteps)
	}

	// --no-config 不修改已有配置。
	writeTestFile(t, dir, "configs/config.yaml", "api:\n  admin:\n    enabled: true\n    prefix: /admin/v1\n")
	before, err := os.ReadFile(filepath.Join(dir, "configs", "config.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	result, err = CreateAPI(CreateAPIOptions{Dir: dir, Name: "api2", Version: "v1", NoConfig: true})
	if err != nil {
		t.Fatal(err)
	}
	if result.ConfigPath != "" {
		t.Fatalf("ConfigPath = %q, want empty with NoConfig", result.ConfigPath)
	}
	after, err := os.ReadFile(filepath.Join(dir, "configs", "config.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Fatalf("config changed with NoConfig:\nbefore:\n%s\nafter:\n%s", before, after)
	}
}

func TestUpsertSurfaceConfigFlowStyles(t *testing.T) {
	out, err := upsertSurfaceConfig([]byte("api: {}\n"), "client", "/client/v1", "")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), "api:\n  client:\n    enabled: true\n    prefix: /client/v1\n") {
		t.Fatalf("unexpected config:\n%s", out)
	}

	out, err = upsertSurfaceConfig([]byte("api: {admin: {enabled: true}}\n"), "client", "/client/v1", "")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), "api:\n  admin: {enabled: true}\n  client:\n") {
		t.Fatalf("inline api section should be rewritten at block level:\n%s", out)
	}

	out, err = upsertSurfaceConfig(nil, "client", "/client/v1", "")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(out), "api:\n  client:\n") {
		t.Fatalf("unexpected empty-file config:\n%q", out)
	}

	out, err = upsertSurfaceConfig([]byte("api:\n  admin:\n    enabled: true\n"), "client", "/client/v1", "")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), "api:\n  admin:") || !strings.Contains(string(out), "\n  client:") {
		t.Fatalf("unexpected config:\n%s", out)
	}
}

func TestUpsertSurfaceConfigPreservesCRLF(t *testing.T) {
	out, err := upsertSurfaceConfig([]byte("app:\r\n  name: demo\r\napi:\r\n  admin:\r\n    enabled: true\r\n    prefix: /admin/v1\r\n"), "client", "/client/v1", "")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(strings.ReplaceAll(string(out), "\r\n", ""), "\n") {
		t.Fatalf("mixed line endings:\n%q", out)
	}
	if !strings.Contains(string(out), "\r\n  client:\r\n") {
		t.Fatalf("missing client surface:\n%q", out)
	}
}

func TestUpsertSurfaceConfigKeepsComments(t *testing.T) {
	out, err := upsertSurfaceConfig([]byte("api: # 多 API 面\n  # 客户端\n  client:\n    enabled: true\n"), "merchant", "/merchant/v1", "")
	if err != nil {
		t.Fatal(err)
	}
	got := string(out)
	for _, want := range []string{"api: # 多 API 面\n", "  # 客户端\n  client:\n", "  merchant:\n    enabled: true\n    prefix: /merchant/v1\n"} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing %q:\n%s", want, got)
		}
	}
	if strings.Index(got, "client:") > strings.Index(got, "merchant:") {
		t.Fatalf("surfaces not sorted:\n%s", got)
	}
}

func TestUpsertSurfaceConfigSectionBoundaries(t *testing.T) {
	src := `api:
  admin:
    enabled: true
# 客户端
  client:
    enabled: true
# 数据库
database:
  dsn: x
`
	out, err := upsertSurfaceConfig([]byte(src), "merchant", "/merchant/v1", "")
	if err != nil {
		t.Fatal(err)
	}
	got := string(out)
	for _, want := range []string{
		"  admin:\n",
		"# 客户端\n  client:\n",
		"  merchant:\n",
		"# 数据库\ndatabase:\n  dsn: x\n",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing %q:\n%s", want, got)
		}
	}
	if strings.Count(got, "# 客户端") != 1 {
		t.Fatalf("comment duplicated:\n%s", got)
	}
	var parsed map[string]any
	if err := yaml.Unmarshal(out, &parsed); err != nil {
		t.Fatalf("result is not valid YAML: %v\n%s", err, got)
	}
}

func TestUpsertSurfaceConfigPreservesAllComments(t *testing.T) {
	src := `# 顶部
app:
  name: x-admin

# 多 API 面说明
api: # 行尾注释
  # 商户端
  merchant:
    enabled: true
    # 前缀说明
    prefix: /merchant/v1
    issuer: x-admin:merchant # 令牌

  # 后台
  admin:
    enabled: true
    prefix: /admin/v1

  # 末尾注释
security:
  bcrypt_cost: 10 # 安全
`
	out, err := upsertSurfaceConfig([]byte(src), "client", "/client/v1", "")
	if err != nil {
		t.Fatal(err)
	}
	got := string(out)
	for _, comment := range []string{
		"# 顶部",
		"# 多 API 面说明",
		"api: # 行尾注释",
		"# 商户端",
		"# 前缀说明",
		"# 令牌",
		"# 后台",
		"# 末尾注释",
		"# 安全",
	} {
		if count := strings.Count(got, comment); count != 1 {
			t.Fatalf("comment %q appears %d times:\n%s", comment, count, got)
		}
	}
	if strings.Index(got, "  admin:") > strings.Index(got, "  client:") ||
		strings.Index(got, "  client:") > strings.Index(got, "  merchant:") {
		t.Fatalf("surfaces not sorted:\n%s", got)
	}
	if !strings.Contains(got, "    prefix: /admin/v1\n\n  client:") {
		t.Fatalf("blank line between surfaces was not preserved:\n%s", got)
	}
}

func TestUpsertSurfaceConfigUpdatesFieldsKeepsComments(t *testing.T) {
	src := `api:
  client:
    # 账户表
    enabled: false # 暂时
    prefix: /old # 前缀
    issuer: x-admin:client
`
	out, err := upsertSurfaceConfig([]byte(src), "client", "/client/v1", "client_users")
	if err != nil {
		t.Fatal(err)
	}
	want := `api:
  client:
    # 账户表
    enabled: true # 暂时
    prefix: /client/v1 # 前缀
    table: client_users
    issuer: x-admin:client
`
	if string(out) != want {
		t.Fatalf("unexpected config:\n%s", out)
	}
}

func TestUpsertSurfaceConfigCommentOnlySection(t *testing.T) {
	src := `api:
  # 还没有自定义面

security:
  bcrypt_cost: 10
`
	out, err := upsertSurfaceConfig([]byte(src), "client", "/client/v1", "")
	if err != nil {
		t.Fatal(err)
	}
	got := string(out)
	if strings.Count(got, "# 还没有自定义面") != 1 {
		t.Fatalf("comment lost or duplicated:\n%s", got)
	}
	if !strings.Contains(got, "  client:\n    enabled: true\n    prefix: /client/v1\n") {
		t.Fatalf("missing surface:\n%s", got)
	}
	if !strings.Contains(got, "\n\nsecurity:\n  bcrypt_cost: 10\n") {
		t.Fatalf("following section changed:\n%s", got)
	}
	var parsed map[string]any
	if err := yaml.Unmarshal(out, &parsed); err != nil {
		t.Fatalf("result is not valid YAML: %v\n%s", err, got)
	}
}

func TestFilterGeneratedSurfaceKeys(t *testing.T) {
	in := []ConfigChange{
		{Key: "api.client.enabled"},
		{Key: "api.admin.legacy"},
		{Key: "app.legacy"},
	}
	out := filterGeneratedSurfaceKeys(in)
	if len(out) != 2 || out[0].Key != "api.admin.legacy" || out[1].Key != "app.legacy" {
		t.Fatalf("unexpected keys: %+v", out)
	}
}
