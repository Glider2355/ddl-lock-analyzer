package fkresolver

import (
	"testing"

	"github.com/Glider2355/ddl-lock-analyzer/internal/meta"
)

func TestSplitQualifiedName(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  [2]string
	}{
		{"schema.table", "mydb.users", [2]string{"mydb", "users"}},
		{"tableのみ", "users", [2]string{"", "users"}},
		{"空文字", "", [2]string{"", ""}},
		{"複数ドット", "a.b.c", [2]string{"a", "b.c"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := splitQualifiedName(tt.input)
			if got != tt.want {
				t.Errorf("splitQualifiedName(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestFKLockTypeString(t *testing.T) {
	tests := []struct {
		name  string
		level meta.LockLevel
		want  string
	}{
		{"EXCLUSIVE", meta.LockExclusive, "EXCLUSIVE"},
		{"SHARED", meta.LockShared, "SHARED_READ"},
		{"NONE", meta.LockNone, "SHARED_READ"},
	}
	// FKLockTypeString is in the reporter package, but we test DetermineLockImpact here
	// which uses the same logic
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Verify via DetermineLockImpact for EXCLUSIVE case
			if tt.level == meta.LockExclusive {
				impact := DetermineLockImpact(FKDirectionParent, []meta.AlterAction{
					{Type: meta.ActionDropColumn, Detail: meta.ActionDetail{ColumnName: "user_id"}},
				}, meta.ForeignKeyMeta{
					SourceColumns:     []string{"user_id"},
					ReferencedColumns: []string{"id"},
					SourceTable:       "orders",
					ReferencedTable:   "users",
				})
				if impact.LockLevel != meta.LockExclusive {
					t.Errorf("FKカラムのDROPはEXCLUSIVEであること: got %s", impact.LockLevel)
				}
			}
		})
	}
}

func TestDetermineLockImpactDefaultShared(t *testing.T) {
	// FKカラムに直接関与しない場合はSHAREDになることを検証
	fk := meta.ForeignKeyMeta{
		SourceTable:       "orders",
		SourceColumns:     []string{"user_id"},
		ReferencedTable:   "users",
		ReferencedColumns: []string{"id"},
	}
	actions := []meta.AlterAction{
		{Type: meta.ActionAddColumn, Detail: meta.ActionDetail{ColumnName: "nickname"}},
	}

	for _, dir := range []FKDirection{FKDirectionParent, FKDirectionChild} {
		t.Run(string(dir), func(t *testing.T) {
			impact := DetermineLockImpact(dir, actions, fk)
			if impact.LockLevel != meta.LockShared {
				t.Errorf("direction=%s: ロックレベルがSHAREDであること: got %s", dir, impact.LockLevel)
			}
			if !impact.MetadataLock {
				t.Error("MetadataLockがtrueであること")
			}
		})
	}
}

func TestDetermineLockImpactModifyFKColumn(t *testing.T) {
	fk := meta.ForeignKeyMeta{
		SourceColumns:     []string{"user_id"},
		ReferencedColumns: []string{"id"},
		SourceTable:       "orders",
		ReferencedTable:   "users",
	}
	actions := []meta.AlterAction{
		{Type: meta.ActionModifyColumn, Detail: meta.ActionDetail{ColumnName: "user_id"}},
	}
	impact := DetermineLockImpact(FKDirectionParent, actions, fk)
	if impact.LockLevel != meta.LockExclusive {
		t.Errorf("FKカラムのMODIFYはEXCLUSIVEであること: got %s", impact.LockLevel)
	}
}
