package xadmin

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func createTestProject(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	writeTestFile(t, dir, "go.mod", "module example.com/app\n\ngo 1.27\n")
	if err := os.MkdirAll(filepath.Join(dir, "internal"), 0o755); err != nil {
		t.Fatal(err)
	}
	return dir
}

func readCreateFile(t *testing.T, root, rel string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestCreateAPIAndHandler(t *testing.T) {
	dir := createTestProject(t)

	result, err := CreateAPI(CreateAPIOptions{Dir: dir, Name: "partner", Version: "v1"})
	if err != nil {
		t.Fatal(err)
	}
	if result.Kind != "api" || result.Name != "partner" || result.Version != "v1" {
		t.Fatalf("unexpected result: %+v", result)
	}
	if !strings.Contains(result.Config, "prefix: /partner/v1") {
		t.Fatalf("missing config snippet:\n%s", result.Config)
	}
	baseHandler := readCreateFile(t, dir, "internal/handler/partner/v1/handler.go")
	if !strings.Contains(baseHandler, "package partnerv1") {
		t.Fatalf("unexpected base handler:\n%s", baseHandler)
	}
	baseRouter := readCreateFile(t, dir, "internal/router/partner_v1.go")
	if !strings.Contains(baseRouter, `RegisterSurface("partner"`) {
		t.Fatalf("unexpected base router:\n%s", baseRouter)
	}

	result, err = CreateHandler(CreateHandlerOptions{
		Dir: dir, API: "partner", Version: "v1", Name: "order_item", Path: "/order-items",
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Kind != "handler" || result.Name != "order_item" {
		t.Fatalf("unexpected result: %+v", result)
	}
	handler := readCreateFile(t, dir, "internal/handler/partner/v1/order_item.go")
	for _, want := range []string{"type OrderItemHandler", `secured.GET("/order-items"`, "secured.POST("} {
		if !strings.Contains(handler, want) {
			t.Fatalf("handler missing %q:\n%s", want, handler)
		}
	}
	route := readCreateFile(t, dir, "internal/router/partner_v1_order_item.go")
	for _, want := range []string{`API:     "partner"`, `Version: "v1"`, "RegisterModule"} {
		if !strings.Contains(route, want) {
			t.Fatalf("route missing %q:\n%s", want, route)
		}
	}
}

func TestCreateAdminHandlerUsesPermission(t *testing.T) {
	dir := createTestProject(t)
	if _, err := CreateHandler(CreateHandlerOptions{
		Dir: dir, API: "admin", Version: "v1", Name: "announcement", Path: "/announcements",
	}); err != nil {
		t.Fatal(err)
	}
	handler := readCreateFile(t, dir, "internal/handler/admin/v1/announcement.go")
	for _, want := range []string{
		`middleware.Require("system:announcement:list")`,
		`middleware.Require("system:announcement:create")`,
		`middleware.Require("system:announcement:update")`,
		`middleware.Require("system:announcement:delete")`,
	} {
		if !strings.Contains(handler, want) {
			t.Fatalf("admin handler missing %q:\n%s", want, handler)
		}
	}
}

func TestCreateAPIVersionedSurface(t *testing.T) {
	dir := createTestProject(t)
	result, err := CreateAPI(CreateAPIOptions{Dir: dir, Name: "partner-v2", Version: "v2"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result.Config, "prefix: /partner/v2") {
		t.Fatalf("unexpected config:\n%s", result.Config)
	}
	handler := readCreateFile(t, dir, "internal/handler/partner-v2/v2/handler.go")
	if !strings.Contains(handler, "package partnerv2") {
		t.Fatalf("unexpected package:\n%s", handler)
	}
}

func TestCreateClientSurface(t *testing.T) {
	dir := createTestProject(t)
	result, err := CreateAPI(CreateAPIOptions{Dir: dir, Name: "client", Version: "v1"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result.Config, "prefix: /client/v1") {
		t.Fatalf("unexpected config:\n%s", result.Config)
	}
	if _, err := os.Stat(filepath.Join(dir, "internal", "router", "client_v1.go")); err != nil {
		t.Fatal(err)
	}
}

func TestCreateAPIAuth(t *testing.T) {
	dir := createTestProject(t)
	result, err := CreateAPI(CreateAPIOptions{
		Dir: dir, Name: "client", Version: "v1", Auth: true,
		Table: "client_accounts", Migration: "001234",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !result.Auth || result.Table != "client_accounts" || result.Migration != "001234" {
		t.Fatalf("unexpected result: %+v", result)
	}
	if !strings.Contains(result.Config, "table: client_accounts") {
		t.Fatalf("config missing table:\n%s", result.Config)
	}
	routerSrc := readCreateFile(t, dir, "internal/router/client_v1.go")
	for _, want := range []string{
		"SurfaceAuthService",
		`/auth/register`,
		`/auth/login`,
		`/auth/refresh`,
		`/auth/logout`,
		`/auth/me`,
	} {
		if !strings.Contains(routerSrc, want) {
			t.Fatalf("router missing %q:\n%s", want, routerSrc)
		}
	}
	up := readCreateFile(t, dir, "migrations/001234_client_accounts.up.sql")
	if !strings.Contains(up, "CREATE TABLE IF NOT EXISTS `client_accounts`") {
		t.Fatalf("unexpected migration:\n%s", up)
	}
	down := readCreateFile(t, dir, "migrations/001234_client_accounts.down.sql")
	if !strings.Contains(down, "DROP TABLE IF EXISTS `client_accounts`") {
		t.Fatalf("unexpected down migration:\n%s", down)
	}
}

func TestCreateAPIAuthDefaultTableAndMigration(t *testing.T) {
	dir := createTestProject(t)
	result, err := CreateAPI(CreateAPIOptions{Dir: dir, Name: "merchant", Version: "v1", Auth: true})
	if err != nil {
		t.Fatal(err)
	}
	if result.Table != "merchant_users" || result.Migration != "001000" {
		t.Fatalf("unexpected defaults: %+v", result)
	}

	// 已有业务迁移时自动顺延，避免撞号。
	writeTestFile(t, dir, "migrations/001005_orders.up.sql", "-- orders\n")
	writeTestFile(t, dir, "migrations/001005_orders.down.sql", "-- down\n")
	result, err = CreateAPI(CreateAPIOptions{Dir: dir, Name: "partner", Version: "v1", Auth: true})
	if err != nil {
		t.Fatal(err)
	}
	if result.Migration != "001006" {
		t.Fatalf("migration = %q, want 001006", result.Migration)
	}
}

func TestCreateAPIAuthRejectsInvalidTable(t *testing.T) {
	dir := createTestProject(t)
	if _, err := CreateAPI(CreateAPIOptions{
		Dir: dir, Name: "client", Auth: true, Table: "bad-table",
	}); err == nil {
		t.Fatal("expected invalid table error")
	}
}

func TestCreateRefusesOverwriteWithoutForce(t *testing.T) {
	dir := createTestProject(t)
	opts := CreateHandlerOptions{Dir: dir, API: "client", Version: "v1", Name: "order"}
	if _, err := CreateHandler(opts); err != nil {
		t.Fatal(err)
	}
	if _, err := CreateHandler(opts); err == nil {
		t.Fatal("expected overwrite error")
	}
	opts.Force = true
	if _, err := CreateHandler(opts); err != nil {
		t.Fatalf("force overwrite failed: %v", err)
	}
}

func TestCreateRejectsBuiltinAPI(t *testing.T) {
	dir := createTestProject(t)
	if _, err := CreateAPI(CreateAPIOptions{Dir: dir, Name: "admin"}); err == nil {
		t.Fatal("expected builtin surface error")
	}
}

func TestCreateRejectsDuplicateSurfaceName(t *testing.T) {
	dir := createTestProject(t)
	if _, err := CreateAPI(CreateAPIOptions{Dir: dir, Name: "partner", Version: "v1"}); err != nil {
		t.Fatal(err)
	}
	if _, err := CreateAPI(CreateAPIOptions{Dir: dir, Name: "partner", Version: "v2", Prefix: "/partner/v2"}); err == nil {
		t.Fatal("expected duplicate surface error")
	}
}

func TestCreateRejectsInvalidNames(t *testing.T) {
	dir := createTestProject(t)
	for _, opts := range []CreateHandlerOptions{
		{Dir: dir, API: "client", Version: "1", Name: "order"},
		{Dir: dir, API: "client", Version: "v1", Name: "order item"},
		{Dir: dir, API: "client", Version: "v1", Name: "order", Path: "orders"},
	} {
		if _, err := CreateHandler(opts); err == nil {
			t.Fatalf("expected error for %+v", opts)
		}
	}
}
