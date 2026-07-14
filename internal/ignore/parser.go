// Package ignore 实现 .iotaignore 的轻量匹配规则。
//
// 语义参考 .gitignore 的常用子集：支持空行/注释、目录忽略、文件名通配符和简单路径匹配。
// 这里不追求完整 .gitignore 兼容，只覆盖配置表批量扫描所需的稳定规则。
package ignore

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
)

// Matcher 保存解析后的忽略模式。
// Match 方法会用这些模式判断相对路径是否应该跳过。
type Matcher struct {
	patterns []pattern
}

// pattern 是单条忽略规则。
// dir=true 表示该规则以目录为单位匹配，例如 temp/。
type pattern struct {
	raw string
	dir bool
}

// Load 从指定路径读取 .iotaignore。
// 文件不存在时返回空 matcher，这样没有 ignore 文件的项目无需特殊处理。
func Load(path string) (Matcher, error) {
	file, err := os.Open(path)
	if os.IsNotExist(err) {
		return Matcher{}, nil
	}
	if err != nil {
		return Matcher{}, err
	}
	defer file.Close()

	var patterns []pattern
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		dir := strings.HasSuffix(line, "/")
		line = strings.Trim(line, "/")
		if line != "" {
			patterns = append(patterns, pattern{raw: filepath.ToSlash(line), dir: dir})
		}
	}
	return Matcher{patterns: patterns}, scanner.Err()
}

// Match 判断相对路径是否命中任意忽略规则。
// rel 会统一转为 slash 分隔，以便 Windows 和类 Unix 平台使用同一套匹配逻辑。
func (m Matcher) Match(rel string, isDir bool) bool {
	rel = filepath.ToSlash(strings.TrimPrefix(rel, "./"))
	base := filepath.Base(rel)
	for _, p := range m.patterns {
		if p.dir && !isDir && !strings.HasPrefix(rel, p.raw+"/") {
			continue
		}
		if p.dir && (rel == p.raw || strings.HasPrefix(rel, p.raw+"/")) {
			return true
		}
		if ok, _ := filepath.Match(p.raw, rel); ok {
			return true
		}
		if ok, _ := filepath.Match(p.raw, base); ok {
			return true
		}
		if strings.Contains(p.raw, "/") && rel == p.raw {
			return true
		}
	}
	return false
}
