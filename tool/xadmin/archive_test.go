package xadmin

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"testing"
)

func makeTarGz(t *testing.T, entries []tar.Header, bodies map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	for _, hdr := range entries {
		h := hdr
		if h.Typeflag == tar.TypeReg {
			h.Size = int64(len(bodies[h.Name]))
		}
		if err := tw.WriteHeader(&h); err != nil {
			t.Fatal(err)
		}
		if h.Typeflag == tar.TypeReg {
			if _, err := tw.Write([]byte(bodies[h.Name])); err != nil {
				t.Fatal(err)
			}
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestExtractTarGzStripsRoot(t *testing.T) {
	data := makeTarGz(t,
		[]tar.Header{
			{Name: "repo-sha/", Typeflag: tar.TypeDir, Mode: 0o755},
			{Name: "repo-sha/README.md", Typeflag: tar.TypeReg, Mode: 0o644},
			{Name: "repo-sha/internal/main.go", Typeflag: tar.TypeReg, Mode: 0o644},
			{Name: "repo-sha/link", Typeflag: tar.TypeSymlink, Linkname: "/etc/passwd"},
			{Name: "repo-sha/node_modules/x.js", Typeflag: tar.TypeReg, Mode: 0o644},
		},
		map[string]string{
			"repo-sha/README.md":         "# hi",
			"repo-sha/internal/main.go":  "package main",
			"repo-sha/node_modules/x.js": "ignored",
		})
	tree, err := ExtractTarGz(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	if string(tree["README.md"].Data) != "# hi" {
		t.Fatalf("README = %q", tree["README.md"].Data)
	}
	if string(tree["internal/main.go"].Data) != "package main" {
		t.Fatalf("main.go = %q", tree["internal/main.go"].Data)
	}
	if _, ok := tree["link"]; ok {
		t.Fatal("symlink should be skipped")
	}
	if _, ok := tree["node_modules/x.js"]; ok {
		t.Fatal("node_modules should be skipped")
	}
}

func TestExtractTarGzRejectsTraversal(t *testing.T) {
	data := makeTarGz(t,
		[]tar.Header{{Name: "../evil.txt", Typeflag: tar.TypeReg, Mode: 0o644}},
		map[string]string{"../evil.txt": "evil"})
	if _, err := ExtractTarGz(bytes.NewReader(data)); err == nil {
		t.Fatal("expected traversal error")
	}
}

func TestExtractTarGzRejectsAbsolute(t *testing.T) {
	data := makeTarGz(t,
		[]tar.Header{{Name: "/tmp/evil.txt", Typeflag: tar.TypeReg, Mode: 0o644}},
		map[string]string{"/tmp/evil.txt": "evil"})
	if _, err := ExtractTarGz(bytes.NewReader(data)); err == nil {
		t.Fatal("expected absolute path error")
	}
}

func TestExtractTarGzRejectsWindowsDrive(t *testing.T) {
	data := makeTarGz(t,
		[]tar.Header{{Name: "repo-sha/C:/Windows/evil.txt", Typeflag: tar.TypeReg, Mode: 0o644}},
		map[string]string{"repo-sha/C:/Windows/evil.txt": "evil"})
	if _, err := ExtractTarGz(bytes.NewReader(data)); err == nil {
		t.Fatal("expected windows drive error")
	}
}
