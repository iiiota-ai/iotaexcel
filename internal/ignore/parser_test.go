package ignore

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadAndMatchPatterns(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".iotaignore")
	content := "# comment\n\nignored/\n*.tmp\nnested/file.xlsx\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	matcher, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name  string
		rel   string
		isDir bool
		want  bool
	}{
		{name: "directory itself", rel: "ignored", isDir: true, want: true},
		{name: "directory child", rel: "ignored/config.xlsx", want: true},
		{name: "base wildcard", rel: "nested/debug.tmp", want: true},
		{name: "exact nested file", rel: "nested/file.xlsx", want: true},
		{name: "kept file", rel: "nested/keep.xlsx", want: false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := matcher.Match(tc.rel, tc.isDir); got != tc.want {
				t.Fatalf("Match(%q, %v) = %v, want %v", tc.rel, tc.isDir, got, tc.want)
			}
		})
	}
}

func TestLoadMissingFileReturnsEmptyMatcher(t *testing.T) {
	matcher, err := Load(filepath.Join(t.TempDir(), ".iotaignore"))
	if err != nil {
		t.Fatal(err)
	}
	if matcher.Match("anything.xlsx", false) {
		t.Fatalf("missing ignore file should not match paths")
	}
}
