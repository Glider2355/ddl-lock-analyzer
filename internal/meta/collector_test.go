package meta

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestFileCollector(t *testing.T) {
	tables := []TableMeta{
		{
			Schema:      "mydb",
			Table:       "users",
			Engine:      "InnoDB",
			RowCount:    1000,
			DataLength:  1024000,
			IndexLength: 512000,
			Columns: []ColumnMeta{
				{Name: "id", OrdinalPos: 1, DataType: "int", ColumnType: "int", IsNullable: false, ColumnKey: "PRI"},
				{Name: "email", OrdinalPos: 2, DataType: "varchar", ColumnType: "varchar(255)", IsNullable: true},
			},
			Indexes: []IndexMeta{
				{Name: "PRIMARY", Columns: []string{"id"}, IsUnique: true, IsPrimary: true},
			},
		},
	}

	data, err := json.Marshal(tables)
	if err != nil {
		t.Fatal(err)
	}

	tmpFile := filepath.Join(t.TempDir(), "meta.json")
	if err := os.WriteFile(tmpFile, data, 0o600); err != nil {
		t.Fatal(err)
	}

	fc, err := NewFileCollector(tmpFile, "8.0.32")
	if err != nil {
		t.Fatal(err)
	}

	if fc.GetMySQLVersion() != "8.0.32" {
		t.Errorf("expected version 8.0.32, got %s", fc.GetMySQLVersion())
	}

	tm, err := fc.GetTableMeta("mydb", "users")
	if err != nil {
		t.Fatal(err)
	}

	if tm.Table != "users" {
		t.Errorf("expected table 'users', got %q", tm.Table)
	}
	if tm.Engine != "InnoDB" {
		t.Errorf("expected engine 'InnoDB', got %q", tm.Engine)
	}
	if tm.RowCount != 1000 {
		t.Errorf("expected 1000 rows, got %d", tm.RowCount)
	}
	if len(tm.Columns) != 2 {
		t.Errorf("expected 2 columns, got %d", len(tm.Columns))
	}
	if tm.MySQLVersion != "8.0.32" {
		t.Errorf("expected MySQL version 8.0.32, got %s", tm.MySQLVersion)
	}
}

func TestFileCollectorNotFound(t *testing.T) {
	tables := []TableMeta{
		{Schema: "mydb", Table: "users"},
	}

	data, err := json.Marshal(tables)
	if err != nil {
		t.Fatal(err)
	}

	tmpFile := filepath.Join(t.TempDir(), "meta.json")
	if err := os.WriteFile(tmpFile, data, 0o600); err != nil {
		t.Fatal(err)
	}

	fc, err := NewFileCollector(tmpFile, "8.0")
	if err != nil {
		t.Fatal(err)
	}

	_, err = fc.GetTableMeta("mydb", "nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent table")
	}
}

func TestFileCollectorByTableNameOnly(t *testing.T) {
	tables := []TableMeta{
		{Schema: "mydb", Table: "users"},
	}

	data, err := json.Marshal(tables)
	if err != nil {
		t.Fatal(err)
	}

	tmpFile := filepath.Join(t.TempDir(), "meta.json")
	if err := os.WriteFile(tmpFile, data, 0o600); err != nil {
		t.Fatal(err)
	}

	fc, err := NewFileCollector(tmpFile, "8.0")
	if err != nil {
		t.Fatal(err)
	}

	// Should find by table name only
	tm, err := fc.GetTableMeta("", "users")
	if err != nil {
		t.Fatal(err)
	}
	if tm.Table != "users" {
		t.Errorf("expected table 'users', got %q", tm.Table)
	}
}

func TestOfflineCollector(t *testing.T) {
	oc := NewOfflineCollector("8.0")
	if oc.GetMySQLVersion() != "8.0" {
		t.Errorf("expected version 8.0, got %s", oc.GetMySQLVersion())
	}
	_, err := oc.GetTableMeta("mydb", "users")
	if err == nil {
		t.Error("expected error from offline collector")
	}
}

func TestFileCollectorInvalidFile(t *testing.T) {
	_, err := NewFileCollector("/nonexistent/path.json", "8.0")
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}

func TestFileCollectorInvalidJSON(t *testing.T) {
	tmpFile := filepath.Join(t.TempDir(), "bad.json")
	if err := os.WriteFile(tmpFile, []byte("{not valid json}"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := NewFileCollector(tmpFile, "8.0")
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}
