package predictor

import (
	"testing"

	"github.com/Glider2355/ddl-lock-analyzer/internal/meta"
)

func TestFindColumn(t *testing.T) {
	tm := &meta.TableMeta{
		Columns: []meta.ColumnMeta{
			{Name: "id", ColumnType: "int"},
			{Name: "Name", ColumnType: "varchar(255)"},
			{Name: "email", ColumnType: "varchar(100)"},
		},
	}

	tests := []struct {
		name     string
		tm       *meta.TableMeta
		colName  string
		wantNil  bool
		wantName string
	}{
		{"既存カラム", tm, "id", false, "id"},
		{"大文字小文字無視", tm, "NAME", false, "Name"},
		{"存在しないカラム", tm, "phone", true, ""},
		{"nilテーブル", nil, "id", true, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			col := findColumn(tt.tm, tt.colName)
			if tt.wantNil {
				if col != nil {
					t.Errorf("nilを期待したが %v を取得", col.Name)
				}
				return
			}
			if col == nil {
				t.Fatal("カラムが見つからない")
			}
			if col.Name != tt.wantName {
				t.Errorf("カラム名: want %s, got %s", tt.wantName, col.Name)
			}
		})
	}
}

func TestIsGeneratedColumn(t *testing.T) {
	tests := []struct {
		name  string
		extra string
		want  bool
	}{
		{"STORED GENERATED", "STORED GENERATED", true},
		{"VIRTUAL GENERATED", "VIRTUAL GENERATED", true},
		{"通常カラム", "", false},
		{"auto_increment", "auto_increment", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			col := &meta.ColumnMeta{Extra: tt.extra}
			if got := isGeneratedColumn(col); got != tt.want {
				t.Errorf("isGeneratedColumn(%q) = %v, want %v", tt.extra, got, tt.want)
			}
		})
	}
}

func TestIsStoredGenerated(t *testing.T) {
	tests := []struct {
		extra string
		want  bool
	}{
		{"STORED GENERATED", true},
		{"VIRTUAL GENERATED", false},
		{"", false},
	}
	for _, tt := range tests {
		t.Run(tt.extra, func(t *testing.T) {
			col := &meta.ColumnMeta{Extra: tt.extra}
			if got := isStoredGenerated(col); got != tt.want {
				t.Errorf("isStoredGenerated(%q) = %v, want %v", tt.extra, got, tt.want)
			}
		})
	}
}

func TestIsVirtualGenerated(t *testing.T) {
	tests := []struct {
		extra string
		want  bool
	}{
		{"VIRTUAL GENERATED", true},
		{"STORED GENERATED", false},
		{"", false},
	}
	for _, tt := range tests {
		t.Run(tt.extra, func(t *testing.T) {
			col := &meta.ColumnMeta{Extra: tt.extra}
			if got := isVirtualGenerated(col); got != tt.want {
				t.Errorf("isVirtualGenerated(%q) = %v, want %v", tt.extra, got, tt.want)
			}
		})
	}
}

func TestIsNullablePtr(t *testing.T) {
	trueVal := true
	falseVal := false
	tests := []struct {
		name string
		ptr  *bool
		want bool
	}{
		{"nil → true", nil, true},
		{"true → true", &trueVal, true},
		{"false → false", &falseVal, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isNullablePtr(tt.ptr); got != tt.want {
				t.Errorf("isNullablePtr = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsHashOrKeyPartition(t *testing.T) {
	tests := []struct {
		partType string
		want     bool
	}{
		{"HASH", true},
		{"KEY", true},
		{"LINEAR HASH", true},
		{"LINEAR KEY", true},
		{"hash", true},
		{"RANGE", false},
		{"LIST", false},
		{"", false},
	}
	for _, tt := range tests {
		t.Run(tt.partType, func(t *testing.T) {
			if got := isHashOrKeyPartition(tt.partType); got != tt.want {
				t.Errorf("isHashOrKeyPartition(%q) = %v, want %v", tt.partType, got, tt.want)
			}
		})
	}
}

func TestHasFulltextIndex(t *testing.T) {
	tests := []struct {
		name    string
		tm      *meta.TableMeta
		want    bool
	}{
		{
			"FULLTEXTあり",
			&meta.TableMeta{Indexes: []meta.IndexMeta{
				{Name: "idx_ft", IndexType: "FULLTEXT"},
				{Name: "idx_b", IndexType: "BTREE"},
			}},
			true,
		},
		{
			"FULLTEXTなし",
			&meta.TableMeta{Indexes: []meta.IndexMeta{
				{Name: "idx_b", IndexType: "BTREE"},
			}},
			false,
		},
		{
			"インデックスなし",
			&meta.TableMeta{},
			false,
		},
		{
			"nilテーブル",
			nil,
			false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := hasFulltextIndex(tt.tm); got != tt.want {
				t.Errorf("hasFulltextIndex = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsColumnReferencedByFK(t *testing.T) {
	tm := &meta.TableMeta{
		ReferencedBy: []meta.ForeignKeyMeta{
			{
				ReferencedColumns: []string{"id", "tenant_id"},
			},
		},
	}

	tests := []struct {
		name    string
		colName string
		tm      *meta.TableMeta
		want    bool
	}{
		{"参照されるカラム", "id", tm, true},
		{"複合キーのカラム", "tenant_id", tm, true},
		{"参照されないカラム", "email", tm, false},
		{"大文字小文字無視", "ID", tm, true},
		{"nilテーブル", "id", nil, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isColumnReferencedByFK(tt.colName, tt.tm); got != tt.want {
				t.Errorf("isColumnReferencedByFK(%q) = %v, want %v", tt.colName, got, tt.want)
			}
		})
	}
}

func TestIsEnumOrSetType(t *testing.T) {
	tests := []struct {
		colType string
		want    bool
	}{
		{"ENUM('a','b')", true},
		{"enum('x')", true},
		{"SET('a','b')", true},
		{"set('x')", true},
		{"VARCHAR(255)", false},
		{"INT", false},
		{"", false},
	}
	for _, tt := range tests {
		t.Run(tt.colType, func(t *testing.T) {
			if got := isEnumOrSetType(tt.colType); got != tt.want {
				t.Errorf("isEnumOrSetType(%q) = %v, want %v", tt.colType, got, tt.want)
			}
		})
	}
}
