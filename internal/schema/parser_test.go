package schema

import (
	"strings"
	"testing"

	"iotaexcel/internal/model"
)

func TestParseTypeAndUsageAliases(t *testing.T) {
	refType, err := ParseType("ref<Item>")
	if err != nil {
		t.Fatal(err)
	}
	if refType.Kind != model.TypeRef || refType.Inner != "Item" {
		t.Fatalf("ref type = %#v", refType)
	}

	mapType, err := ParseType("map<string, int>")
	if err != nil {
		t.Fatal(err)
	}
	if mapType.Kind != model.TypeMap || len(mapType.Args) != 2 || mapType.Args[1] != "int" {
		t.Fatalf("map type = %#v", mapType)
	}

	usage, err := ParseUsage("c/server")
	if err != nil {
		t.Fatal(err)
	}
	if usage != model.UsageAll {
		t.Fatalf("usage = %q, want all", usage)
	}

	if _, err := ParseUsage("comment,client"); err == nil {
		t.Fatalf("comment combined with export usage should fail")
	}
}

func TestConvertValueDefaultsAndErrors(t *testing.T) {
	boolType := model.TypeSpec{Raw: "bool", Kind: model.TypeBool}
	value, usedDefault, err := ConvertValue("maybe", boolType)
	if err == nil {
		t.Fatalf("invalid bool should return an error")
	}
	if value != false || !usedDefault {
		t.Fatalf("invalid bool value/default = %#v/%v", value, usedDefault)
	}

	intType := model.TypeSpec{Raw: "int", Kind: model.TypeInt}
	value, usedDefault, err = ConvertValue(" 42 ", intType)
	if err != nil {
		t.Fatal(err)
	}
	if value != int32(42) || usedDefault {
		t.Fatalf("int value/default = %#v/%v", value, usedDefault)
	}

	mapType := model.TypeSpec{Raw: "map<string,int>", Kind: model.TypeMap}
	value, usedDefault, err = ConvertValue("atk:10|level:2", mapType)
	if err != nil {
		t.Fatal(err)
	}
	gotMap := value.(map[string]string)
	if gotMap["atk"] != "10" || gotMap["level"] != "2" || usedDefault {
		t.Fatalf("map value/default = %#v/%v", value, usedDefault)
	}
}

func TestParseWorkbookBuildsSchemaAndRows(t *testing.T) {
	raw := model.RawWorkbook{
		SourcePath: "Config.xlsx",
		RelPath:    "Config.xlsx",
		Sheets: []model.RawSheet{{
			Name: "Item",
			Rows: [][]string{
				{"id#", "name*", "code!", "score", "note"},
				{"int", "string", "string", "int", "string"},
				{"all", "client", "all", "server", "comment"},
				{"ID", "Name", "Code", "Score", "Note"},
				{"1", "Sword", "sword", "-12", "local note"},
				{"", "", "", "", ""},
				{"2", "Shield", "", "bad", "ignored note"},
			},
		}},
	}

	wb, err := ParseWorkbook(raw, Options{Target: "both"})
	if err != nil {
		t.Fatal(err)
	}
	sheet := wb.Sheets[0]
	if len(sheet.Fields) != 5 || !sheet.Fields[0].IsKey || !sheet.Fields[0].Required || !sheet.Fields[0].Unique || !sheet.Fields[1].Required || !sheet.Fields[2].Unique || sheet.Fields[4].Binary {
		t.Fatalf("fields = %#v", sheet.Fields)
	}
	if len(sheet.Rows) != 2 || sheet.SkippedEmptyRows[0] != 6 {
		t.Fatalf("rows/skipped = %#v/%#v", sheet.Rows, sheet.SkippedEmptyRows)
	}
	if sheet.DefaultValueCount != 2 || len(sheet.ConversionErrors) != 1 {
		t.Fatalf("defaults/errors = %d/%#v", sheet.DefaultValueCount, sheet.ConversionErrors)
	}
	if sheet.SchemaHash == "" {
		t.Fatalf("schema hash should be set")
	}
}

func TestParseWorkbookRejectsDuplicateKeys(t *testing.T) {
	raw := model.RawWorkbook{
		SourcePath: "Config.xlsx",
		RelPath:    "Config.xlsx",
		Sheets: []model.RawSheet{{
			Name: "Item",
			Rows: [][]string{
				{"id#", "name"},
				{"int", "string"},
				{"all", "all"},
				{"ID", "Name"},
				{"1", "Sword"},
				{"1", "Shield"},
			},
		}},
	}

	_, err := ParseWorkbook(raw, Options{Target: "both"})
	if err == nil || !strings.Contains(err.Error(), "duplicate key") {
		t.Fatalf("err = %v, want duplicate key", err)
	}
}

func TestParseWorkbookRejectsEmptyRequiredField(t *testing.T) {
	raw := model.RawWorkbook{
		SourcePath: "Config.xlsx",
		RelPath:    "Config.xlsx",
		Sheets: []model.RawSheet{{
			Name: "Item",
			Rows: [][]string{
				{"id#", "name*"},
				{"int", "string"},
				{"all", "all"},
				{"ID", "Name"},
				{"1", ""},
			},
		}},
	}

	_, err := ParseWorkbook(raw, Options{Target: "both"})
	if err == nil || !strings.Contains(err.Error(), "field name is required") {
		t.Fatalf("err = %v, want required field", err)
	}
}

func TestParseWorkbookRejectsDuplicateUniqueField(t *testing.T) {
	raw := model.RawWorkbook{
		SourcePath: "Config.xlsx",
		RelPath:    "Config.xlsx",
		Sheets: []model.RawSheet{{
			Name: "Item",
			Rows: [][]string{
				{"id#", "email!"},
				{"int", "string"},
				{"all", "all"},
				{"ID", "Email"},
				{"1", "a@test.com"},
				{"2", ""},
				{"3", ""},
				{"4", "a@test.com"},
			},
		}},
	}

	_, err := ParseWorkbook(raw, Options{Target: "both"})
	if err == nil || !strings.Contains(err.Error(), "duplicate unique value") {
		t.Fatalf("err = %v, want duplicate unique field", err)
	}
}
