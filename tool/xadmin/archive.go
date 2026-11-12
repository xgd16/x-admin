package xadmin

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"io/fs"
	"path"
	"strings"
)

const (
	maxArchiveFileSize  = 64 << 20  // 单文件 64 MiB
	maxArchiveTotalSize = 512 << 20 // 解压后总量 512 MiB
)

// ExtractTarGz 解压 GitHub 源码归档。
//
// GitHub 的 tar.gz 顶层会有一个 <repo>-<sha>/ 目录，这里统一剥掉。
// 只接受普通文件；路径穿越、绝对路径、反斜杠路径和符号链接都会被拒绝或跳过。
func ExtractTarGz(r io.Reader) (Tree, error) {
	gz, err := gzip.NewReader(r)
	if err != nil {
		return nil, fmt.Errorf("打开 gzip: %w", err)
	}
	defer gz.Close()

	tr := tar.NewReader(gz)
	tree := Tree{}
	var total int64
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("读取 tar: %w", err)
		}
		name := hdr.Name
		if strings.Contains(name, "\\") {
			return nil, fmt.Errorf("归档路径包含反斜杠: %q", name)
		}
		if strings.Contains(name, ":") {
			return nil, fmt.Errorf("归档路径包含冒号: %q", name)
		}
		clean := path.Clean(name)
		if clean == "." {
			continue
		}
		if path.IsAbs(clean) || clean == ".." || strings.HasPrefix(clean, "../") {
			return nil, fmt.Errorf("归档路径越界: %q", name)
		}
		if hdr.Typeflag != tar.TypeReg && hdr.Typeflag != tar.TypeRegA {
			// 目录、符号链接、硬链接等一律跳过，避免把链接写到工作区。
			continue
		}
		if hdr.Size < 0 || hdr.Size > maxArchiveFileSize {
			return nil, fmt.Errorf("归档文件过大: %s (%d bytes)", name, hdr.Size)
		}
		total += hdr.Size
		if total > maxArchiveTotalSize {
			return nil, fmt.Errorf("归档解压后超过 %d MiB", maxArchiveTotalSize>>20)
		}
		// 剥掉 tar.gz 的顶层目录。
		parts := strings.SplitN(clean, "/", 2)
		if len(parts) != 2 || parts[1] == "" {
			continue
		}
		rel := parts[1]
		if isIgnoredPath(rel) {
			continue
		}
		data, err := io.ReadAll(io.LimitReader(tr, maxArchiveFileSize+1))
		if err != nil {
			return nil, fmt.Errorf("读取 %s: %w", name, err)
		}
		if int64(len(data)) > maxArchiveFileSize {
			return nil, fmt.Errorf("归档文件过大: %s", name)
		}
		mode := fs.FileMode(hdr.Mode).Perm()
		if mode == 0 {
			mode = 0o644
		}
		tree[rel] = File{Data: data, Mode: mode}
	}
	return tree, nil
}
