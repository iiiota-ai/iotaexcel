package batch

import (
	"os"
	"path/filepath"
	"sort"
	"testing"
)

func TestDiscoverDirectoryAppliesIgnoreAndRecursion(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, ".iotaignore"), "skip/\n*.tmp\n")
	writeFile(t, filepath.Join(root, "Root.xlsx"), "")
	writeFile(t, filepath.Join(root, "~$Temp.xlsx"), "")
	writeFile(t, filepath.Join(root, "debug.tmp"), "")
	writeFile(t, filepath.Join(root, "skip", "Ignored.xlsx"), "")
	writeFile(t, filepath.Join(root, "nested", "Child.xlsx"), "")

	files, err := Discover(root, Options{Recursive: true})
	if err != nil {
		t.Fatal(err)
	}
	if got := relPaths(files); !equalStrings(got, []string{"Root.xlsx", "nested/Child.xlsx"}) {
		t.Fatalf("recursive rel paths = %#v", got)
	}

	files, err = Discover(root, Options{Recursive: false})
	if err != nil {
		t.Fatal(err)
	}
	if got := relPaths(files); !equalStrings(got, []string{"Root.xlsx"}) {
		t.Fatalf("non-recursive rel paths = %#v", got)
	}
}

func TestDiscoverSingleFile(t *testing.T) {
	root := t.TempDir()
	xlsx := filepath.Join(repoRoot(t), "tests", "testdata", "excels", "valid", "Config.xlsx")
	txt := filepath.Join(root, "Config.txt")
	writeFile(t, txt, "")

	files, err := Discover(xlsx, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if got := relPaths(files); !equalStrings(got, []string{"Config.xlsx"}) {
		t.Fatalf("xlsx rel paths = %#v", got)
	}

	files, err = Discover(txt, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 0 {
		t.Fatalf("txt files = %#v, want none", files)
	}
}

func repoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("go.mod not found")
		}
		dir = parent
	}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func relPaths(files []File) []string {
	out := make([]string, 0, len(files))
	for _, file := range files {
		out = append(out, filepath.ToSlash(file.RelPath))
	}
	sort.Strings(out)
	return out
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
