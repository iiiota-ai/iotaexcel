// Package batch 负责把用户输入路径展开成需要处理的 Excel 文件列表。
//
// 它支持单文件输入、目录递归扫描、.iotaignore 过滤和 Excel 临时文件跳过，
// 是 convert/codegen 命令批量处理能力的入口。
package batch

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"iotaexcel/internal/ignore"
)

// Options 控制文件发现行为。
// Recursive 为 false 时只处理输入目录第一层文件。
// Extensions 为空时默认发现 .xlsx；传入时按指定扩展名发现文件，例如 decode 命令会传入 .bytes。
type Options struct {
	Recursive  bool
	Extensions []string
}

// File 表示一个待处理 Excel 文件。
// Path 是绝对或调用方传入的真实路径；RelPath 是相对扫描根目录的路径，用于输出时保留目录结构。
type File struct {
	Path    string
	RelPath string
}

// Discover 根据输入路径发现所有可处理的 .xlsx 文件。
// 如果 input 是文件，只在它是非临时 .xlsx 时返回；如果 input 是目录，则加载该目录下的 .iotaignore 后扫描。
func Discover(input string, opts Options) ([]File, error) {
	extensions := normalizedExtensions(opts.Extensions)
	info, err := os.Stat(input)
	if err != nil {
		return nil, err
	}

	root := input
	if !info.IsDir() {
		if hasExtension(input, extensions) && !isExcelTemp(input) {
			return []File{{Path: input, RelPath: filepath.Base(input)}}, nil
		}
		return nil, nil
	}

	matcher, err := ignore.Load(filepath.Join(root, ".iotaignore"))
	if err != nil {
		return nil, err
	}

	var files []File
	walkFn := func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if path == root {
			return nil
		}

		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)

		if matcher.Match(rel, d.IsDir()) {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if d.IsDir() {
			if !opts.Recursive {
				return filepath.SkipDir
			}
			return nil
		}
		if hasExtension(path, extensions) && !isExcelTemp(path) {
			files = append(files, File{Path: path, RelPath: rel})
		}
		return nil
	}

	if err := filepath.WalkDir(root, walkFn); err != nil {
		return nil, err
	}
	return files, nil
}

// isXLSX 判断路径扩展名是否为 .xlsx，大小写不敏感。
// normalizedExtensions 规范化调用方传入的扩展名列表。
// 扩展名统一带点号并转小写，便于后续跨平台大小写不敏感匹配。
func normalizedExtensions(values []string) []string {
	if len(values) == 0 {
		return []string{".xlsx"}
	}
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if !strings.HasPrefix(value, ".") {
			value = "." + value
		}
		out = append(out, strings.ToLower(value))
	}
	if len(out) == 0 {
		return []string{".xlsx"}
	}
	return out
}

// hasExtension 判断路径扩展名是否在允许列表中，大小写不敏感。
func hasExtension(path string, extensions []string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	for _, allowed := range extensions {
		if ext == allowed {
			return true
		}
	}
	return false
}

// isExcelTemp 判断文件名是否为 Excel 临时文件。
// Excel 打开工作簿时常创建 ~$ 前缀文件，这类文件不应进入导出流程。
func isExcelTemp(path string) bool {
	return strings.HasPrefix(filepath.Base(path), "~$")
}
