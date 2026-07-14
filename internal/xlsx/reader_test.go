package xlsx

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadFiltersSheetAndPreservesRows(t *testing.T) {
	root := repoRoot(t)
	path := filepath.Join(root, "tests", "testdata", "excels", "valid", "Config.xlsx")

	wb, err := Read(path, "Config.xlsx", "Item")
	if err != nil {
		t.Fatal(err)
	}
	if len(wb.Sheets) != 1 || wb.Sheets[0].Name != "Item" {
		t.Fatalf("sheets = %#v", wb.Sheets)
	}
	if got := wb.Sheets[0].Rows[4][1]; got != "Sword" {
		t.Fatalf("row 5 column 2 = %q, want Sword", got)
	}
}

func TestReadSupportsInlineStrings(t *testing.T) {
	root := repoRoot(t)
	path := filepath.Join(root, "tests", "testdata", "excels", "valid", "InlineStrings.xlsx")

	wb, err := Read(path, "InlineStrings.xlsx", "Inline")
	if err != nil {
		t.Fatal(err)
	}
	if len(wb.Sheets) != 1 || len(wb.Sheets[0].Rows) < 5 {
		t.Fatalf("inline workbook = %#v", wb)
	}
	if got := strings.Join(wb.Sheets[0].Rows[4], "|"); !strings.Contains(got, "inline") {
		t.Fatalf("inline row = %q", got)
	}
}

func TestReadMissingSheetReturnsError(t *testing.T) {
	root := repoRoot(t)
	path := filepath.Join(root, "tests", "testdata", "excels", "valid", "Config.xlsx")

	_, err := Read(path, "Config.xlsx", "Missing")
	if err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("err = %v, want not found", err)
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
