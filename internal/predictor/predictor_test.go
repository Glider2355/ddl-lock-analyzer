package predictor

import (
	"testing"

	"github.com/Glider2355/ddl-lock-analyzer/internal/meta"
)

func boolPtr(b bool) *bool { return &b }

// ============================================================
// ADD COLUMN tests
// ============================================================

func TestPredictAddColumnTail(t *testing.T) {
	p := New()
	action := meta.AlterAction{
		Type: meta.ActionAddColumn,
		Detail: meta.ActionDetail{
			ColumnName: "nickname",
			ColumnType: "VARCHAR(255)",
			IsNullable: boolPtr(true),
		},
	}
	pred := p.Predict(action, nil)
	if pred.Algorithm != meta.AlgorithmInstant {
		t.Errorf("アルゴリズムがINSTANTであること: got %s", pred.Algorithm)
	}
	if pred.Lock != meta.LockNone {
		t.Errorf("ロックがNONEであること: got %s", pred.Lock)
	}
	if pred.RiskLevel != meta.RiskLow {
		t.Errorf("リスクレベルがLOWであること: got %s", pred.RiskLevel)
	}
}

func TestPredictAddColumnFirst(t *testing.T) {
	p := New()
	action := meta.AlterAction{
		Type: meta.ActionAddColumn,
		Detail: meta.ActionDetail{
			ColumnName: "id",
			ColumnType: "INT",
			IsNullable: boolPtr(true),
			Position:   "FIRST",
		},
	}
	pred := p.Predict(action, nil)
	if pred.Algorithm != meta.AlgorithmInstant {
		t.Errorf("アルゴリズムがINSTANTであること: got %s", pred.Algorithm)
	}
}

func TestPredictAddColumnNotNull(t *testing.T) {
	// MySQL 8.0.12+: NOT NULL column with DEFAULT is INSTANT
	p := New()
	action := meta.AlterAction{
		Type: meta.ActionAddColumn,
		Detail: meta.ActionDetail{
			ColumnName: "status",
			ColumnType: "INT",
			IsNullable: boolPtr(false),
		},
	}
	pred := p.Predict(action, nil)
	if pred.Algorithm != meta.AlgorithmInstant {
		t.Errorf("アルゴリズムがINSTANTであること: got %s", pred.Algorithm)
	}
	if pred.Lock != meta.LockNone {
		t.Errorf("ロックがNONEであること: got %s", pred.Lock)
	}
	if pred.TableRebuild {
		t.Error("テーブル再構築が不要であること")
	}
}

func TestPredictAddColumnAutoIncrement(t *testing.T) {
	// AUTO_INCREMENT column requires SHARED lock
	p := New()
	action := meta.AlterAction{
		Type: meta.ActionAddColumn,
		Detail: meta.ActionDetail{
			ColumnName:      "id",
			ColumnType:      "BIGINT",
			IsNullable:      boolPtr(false),
			IsAutoIncrement: true,
		},
	}
	pred := p.Predict(action, nil)
	if pred.Algorithm != meta.AlgorithmInplace {
		t.Errorf("アルゴリズムがINPLACEであること: got %s", pred.Algorithm)
	}
	if pred.Lock != meta.LockShared {
		t.Errorf("ロックがSHAREDであること: got %s", pred.Lock)
	}
}

func TestPredictAddColumnStoredGenerated(t *testing.T) {
	p := New()
	action := meta.AlterAction{
		Type: meta.ActionAddColumn,
		Detail: meta.ActionDetail{
			ColumnName:    "full_name",
			ColumnType:    "VARCHAR(512)",
			GeneratedType: "STORED",
		},
	}
	pred := p.Predict(action, nil)
	if pred.Algorithm != meta.AlgorithmCopy {
		t.Errorf("アルゴリズムがCOPYであること: got %s", pred.Algorithm)
	}
	if pred.Lock != meta.LockShared {
		t.Errorf("ロックがSHAREDであること: got %s", pred.Lock)
	}
	if !pred.TableRebuild {
		t.Error("テーブル再構築が必要であること")
	}
}

func TestPredictAddColumnVirtualGenerated(t *testing.T) {
	p := New()
	action := meta.AlterAction{
		Type: meta.ActionAddColumn,
		Detail: meta.ActionDetail{
			ColumnName:    "full_name",
			ColumnType:    "VARCHAR(512)",
			GeneratedType: "VIRTUAL",
		},
	}
	pred := p.Predict(action, nil)
	if pred.Algorithm != meta.AlgorithmInstant {
		t.Errorf("アルゴリズムがINSTANTであること: got %s", pred.Algorithm)
	}
	if pred.Lock != meta.LockNone {
		t.Errorf("ロックがNONEであること: got %s", pred.Lock)
	}
}

// ============================================================
// DROP COLUMN tests
// ============================================================

func TestPredictDropColumn(t *testing.T) {
	p := New()
	action := meta.AlterAction{
		Type:   meta.ActionDropColumn,
		Detail: meta.ActionDetail{ColumnName: "nickname"},
	}
	pred := p.Predict(action, nil)
	if pred.Algorithm != meta.AlgorithmInstant {
		t.Errorf("アルゴリズムがINSTANTであること: got %s", pred.Algorithm)
	}
	if pred.RiskLevel != meta.RiskLow {
		t.Errorf("リスクレベルがLOWであること: got %s", pred.RiskLevel)
	}
}

func TestPredictDropColumnStoredGenerated(t *testing.T) {
	p := New()
	action := meta.AlterAction{
		Type:   meta.ActionDropColumn,
		Detail: meta.ActionDetail{ColumnName: "gen_col"},
	}
	tableMeta := &meta.TableMeta{
		Engine: "InnoDB",
		Columns: []meta.ColumnMeta{
			{Name: "gen_col", ColumnType: "INT", Extra: "STORED GENERATED"},
		},
	}
	pred := p.Predict(action, tableMeta)
	if pred.Algorithm != meta.AlgorithmInplace {
		t.Errorf("アルゴリズムがINPLACEであること: got %s", pred.Algorithm)
	}
	if !pred.TableRebuild {
		t.Error("テーブル再構築が必要であること")
	}
}

// ============================================================
// MODIFY COLUMN tests
// ============================================================

func TestPredictModifyColumnTypeChange(t *testing.T) {
	// Type change: COPY / SHARED (not EXCLUSIVE)
	p := New()
	action := meta.AlterAction{
		Type: meta.ActionModifyColumn,
		Detail: meta.ActionDetail{
			ColumnName: "email",
			ColumnType: "VARCHAR(512)",
		},
	}
	tableMeta := &meta.TableMeta{
		Engine: "InnoDB",
		Columns: []meta.ColumnMeta{
			{Name: "email", ColumnType: "VARCHAR(255)"},
		},
	}
	pred := p.Predict(action, tableMeta)
	// VARCHAR(255) → VARCHAR(512) crosses the byte boundary (255→512)
	// so it's a type change, not an in-place extension
	if pred.Algorithm != meta.AlgorithmCopy {
		t.Errorf("アルゴリズムがCOPYであること: got %s", pred.Algorithm)
	}
	if pred.Lock != meta.LockShared {
		t.Errorf("ロックがSHAREDであること: got %s", pred.Lock)
	}
	if pred.RiskLevel != meta.RiskCritical {
		t.Errorf("リスクレベルがCRITICALであること: got %s", pred.RiskLevel)
	}
}

func TestPredictModifyColumnVarcharExtension(t *testing.T) {
	// VARCHAR extension within same byte boundary → INPLACE / NONE
	p := New()
	action := meta.AlterAction{
		Type: meta.ActionModifyColumn,
		Detail: meta.ActionDetail{
			ColumnName: "name",
			ColumnType: "VARCHAR(200)",
			IsNullable: boolPtr(true),
		},
	}
	tableMeta := &meta.TableMeta{
		Engine: "InnoDB",
		Columns: []meta.ColumnMeta{
			{Name: "name", ColumnType: "VARCHAR(100)", IsNullable: true},
		},
	}
	pred := p.Predict(action, tableMeta)
	if pred.Algorithm != meta.AlgorithmInplace {
		t.Errorf("アルゴリズムがINPLACEであること: got %s", pred.Algorithm)
	}
	if pred.Lock != meta.LockNone {
		t.Errorf("ロックがNONEであること: got %s", pred.Lock)
	}
	if pred.TableRebuild {
		t.Error("テーブル再構築が不要であること")
	}
}

func TestPredictModifyColumnVarcharCrossBoundary(t *testing.T) {
	// VARCHAR(255) → VARCHAR(256) crosses byte boundary → COPY
	p := New()
	action := meta.AlterAction{
		Type: meta.ActionModifyColumn,
		Detail: meta.ActionDetail{
			ColumnName: "name",
			ColumnType: "VARCHAR(256)",
			IsNullable: boolPtr(true),
		},
	}
	tableMeta := &meta.TableMeta{
		Engine: "InnoDB",
		Columns: []meta.ColumnMeta{
			{Name: "name", ColumnType: "VARCHAR(255)", IsNullable: true},
		},
	}
	pred := p.Predict(action, tableMeta)
	if pred.Algorithm != meta.AlgorithmCopy {
		t.Errorf("アルゴリズムがCOPYであること: got %s", pred.Algorithm)
	}
	if pred.Lock != meta.LockShared {
		t.Errorf("ロックがSHAREDであること: got %s", pred.Lock)
	}
}

func TestPredictModifyColumnNullToNotNull(t *testing.T) {
	p := New()
	action := meta.AlterAction{
		Type: meta.ActionModifyColumn,
		Detail: meta.ActionDetail{
			ColumnName: "email",
			ColumnType: "varchar(255)",
			IsNullable: boolPtr(false),
		},
	}
	tableMeta := &meta.TableMeta{
		Engine: "InnoDB",
		Columns: []meta.ColumnMeta{
			{Name: "email", ColumnType: "varchar(255)", IsNullable: true},
		},
	}
	pred := p.Predict(action, tableMeta)
	if pred.Algorithm != meta.AlgorithmInplace {
		t.Errorf("アルゴリズムがINPLACEであること: got %s", pred.Algorithm)
	}
	if pred.RiskLevel != meta.RiskHigh {
		t.Errorf("リスクレベルがHIGHであること: got %s", pred.RiskLevel)
	}
	if !pred.TableRebuild {
		t.Error("テーブル再構築が必要であること")
	}
}

func TestPredictModifyColumnNotNullToNull(t *testing.T) {
	p := New()
	action := meta.AlterAction{
		Type: meta.ActionModifyColumn,
		Detail: meta.ActionDetail{
			ColumnName: "email",
			ColumnType: "varchar(255)",
			IsNullable: boolPtr(true),
		},
	}
	tableMeta := &meta.TableMeta{
		Engine: "InnoDB",
		Columns: []meta.ColumnMeta{
			{Name: "email", ColumnType: "varchar(255)", IsNullable: false},
		},
	}
	pred := p.Predict(action, tableMeta)
	if pred.Algorithm != meta.AlgorithmInplace {
		t.Errorf("アルゴリズムがINPLACEであること: got %s", pred.Algorithm)
	}
	if pred.Lock != meta.LockNone {
		t.Errorf("ロックがNONEであること: got %s", pred.Lock)
	}
	if !pred.TableRebuild {
		t.Error("テーブル再構築が必要であること")
	}
}

func TestPredictModifyColumnReorder(t *testing.T) {
	p := New()
	action := meta.AlterAction{
		Type: meta.ActionModifyColumn,
		Detail: meta.ActionDetail{
			ColumnName: "email",
			ColumnType: "varchar(255)",
			IsNullable: boolPtr(true),
			Position:   "FIRST",
		},
	}
	tableMeta := &meta.TableMeta{
		Engine: "InnoDB",
		Columns: []meta.ColumnMeta{
			{Name: "email", ColumnType: "varchar(255)", IsNullable: true},
		},
	}
	pred := p.Predict(action, tableMeta)
	if pred.Algorithm != meta.AlgorithmInplace {
		t.Errorf("アルゴリズムがINPLACEであること: got %s", pred.Algorithm)
	}
	if pred.Lock != meta.LockNone {
		t.Errorf("ロックがNONEであること: got %s", pred.Lock)
	}
	if !pred.TableRebuild {
		t.Error("テーブル再構築が必要であること")
	}
}

func TestPredictModifyColumnEnumExtension(t *testing.T) {
	p := New()
	action := meta.AlterAction{
		Type: meta.ActionModifyColumn,
		Detail: meta.ActionDetail{
			ColumnName: "status",
			ColumnType: "ENUM('active','inactive','pending')",
			IsNullable: boolPtr(false),
		},
	}
	tableMeta := &meta.TableMeta{
		Engine: "InnoDB",
		Columns: []meta.ColumnMeta{
			{Name: "status", ColumnType: "ENUM('active','inactive')", IsNullable: false},
		},
	}
	pred := p.Predict(action, tableMeta)
	if pred.Algorithm != meta.AlgorithmInstant {
		t.Errorf("アルゴリズムがINSTANTであること: got %s", pred.Algorithm)
	}
	if pred.Lock != meta.LockNone {
		t.Errorf("ロックがNONEであること: got %s", pred.Lock)
	}
}

// ============================================================
// CHANGE COLUMN tests
// ============================================================

func TestPredictChangeColumnRenameOnly(t *testing.T) {
	p := New()
	action := meta.AlterAction{
		Type: meta.ActionChangeColumn,
		Detail: meta.ActionDetail{
			ColumnName:    "full_name",
			OldColumnName: "name",
			ColumnType:    "varchar(255)",
		},
	}
	tableMeta := &meta.TableMeta{
		Engine: "InnoDB",
		Columns: []meta.ColumnMeta{
			{Name: "name", ColumnType: "varchar(255)"},
		},
	}
	pred := p.Predict(action, tableMeta)
	if pred.Algorithm != meta.AlgorithmInstant {
		t.Errorf("アルゴリズムがINSTANTであること: got %s", pred.Algorithm)
	}
	if pred.Lock != meta.LockNone {
		t.Errorf("ロックがNONEであること: got %s", pred.Lock)
	}
}

func TestPredictChangeColumnTypeChange(t *testing.T) {
	p := New()
	action := meta.AlterAction{
		Type: meta.ActionChangeColumn,
		Detail: meta.ActionDetail{
			ColumnName:    "full_name",
			OldColumnName: "name",
			ColumnType:    "TEXT",
		},
	}
	tableMeta := &meta.TableMeta{
		Engine: "InnoDB",
		Columns: []meta.ColumnMeta{
			{Name: "name", ColumnType: "varchar(255)"},
		},
	}
	pred := p.Predict(action, tableMeta)
	if pred.Algorithm != meta.AlgorithmCopy {
		t.Errorf("アルゴリズムがCOPYであること: got %s", pred.Algorithm)
	}
	if pred.Lock != meta.LockShared {
		t.Errorf("ロックがSHAREDであること: got %s", pred.Lock)
	}
}

func TestPredictChangeColumnNoMetadata(t *testing.T) {
	// Without metadata, CHANGE COLUMN assumes type change (conservative)
	p := New()
	action := meta.AlterAction{
		Type: meta.ActionChangeColumn,
		Detail: meta.ActionDetail{
			ColumnName:    "full_name",
			OldColumnName: "name",
			ColumnType:    "VARCHAR(255)",
		},
	}
	pred := p.Predict(action, nil)
	if pred.Algorithm != meta.AlgorithmCopy {
		t.Errorf("アルゴリズムがCOPYであること: got %s", pred.Algorithm)
	}
	if pred.Lock != meta.LockShared {
		t.Errorf("ロックがSHAREDであること: got %s", pred.Lock)
	}
}

// ============================================================
// INDEX tests
// ============================================================

func TestPredictAddIndex(t *testing.T) {
	p := New()
	action := meta.AlterAction{
		Type: meta.ActionAddIndex,
		Detail: meta.ActionDetail{
			IndexName:    "idx_email",
			IndexColumns: []string{"email"},
		},
	}
	pred := p.Predict(action, nil)
	if pred.Algorithm != meta.AlgorithmInplace {
		t.Errorf("アルゴリズムがINPLACEであること: got %s", pred.Algorithm)
	}
	if pred.Lock != meta.LockNone {
		t.Errorf("ロックがNONEであること: got %s", pred.Lock)
	}
	if pred.RiskLevel != meta.RiskMedium {
		t.Errorf("リスクレベルがMEDIUMであること: got %s", pred.RiskLevel)
	}
}

func TestPredictAddFulltextIndexFirst(t *testing.T) {
	// First FULLTEXT index may require table rebuild
	p := New()
	action := meta.AlterAction{
		Type: meta.ActionAddFulltextIndex,
		Detail: meta.ActionDetail{
			IndexName:    "ft_content",
			IndexColumns: []string{"content"},
		},
	}
	tableMeta := &meta.TableMeta{
		Engine:  "InnoDB",
		Indexes: []meta.IndexMeta{},
	}
	pred := p.Predict(action, tableMeta)
	if pred.Algorithm != meta.AlgorithmInplace {
		t.Errorf("アルゴリズムがINPLACEであること: got %s", pred.Algorithm)
	}
	if pred.Lock != meta.LockShared {
		t.Errorf("ロックがSHAREDであること: got %s", pred.Lock)
	}
	if !pred.TableRebuild {
		t.Error("最初のFULLTEXTインデックスはテーブル再構築の可能性あり")
	}
}

func TestPredictAddFulltextIndexSubsequent(t *testing.T) {
	// Subsequent FULLTEXT index — no rebuild
	p := New()
	action := meta.AlterAction{
		Type: meta.ActionAddFulltextIndex,
		Detail: meta.ActionDetail{
			IndexName:    "ft_title",
			IndexColumns: []string{"title"},
		},
	}
	tableMeta := &meta.TableMeta{
		Engine: "InnoDB",
		Indexes: []meta.IndexMeta{
			{Name: "ft_content", IndexType: "FULLTEXT"},
		},
	}
	pred := p.Predict(action, tableMeta)
	if pred.Lock != meta.LockShared {
		t.Errorf("ロックがSHAREDであること: got %s", pred.Lock)
	}
	if pred.TableRebuild {
		t.Error("後続のFULLTEXTインデックスはテーブル再構築不要")
	}
}

func TestPredictAddSpatialIndex(t *testing.T) {
	p := New()
	action := meta.AlterAction{
		Type: meta.ActionAddSpatialIndex,
	}
	pred := p.Predict(action, nil)
	if pred.Algorithm != meta.AlgorithmInplace {
		t.Errorf("アルゴリズムがINPLACEであること: got %s", pred.Algorithm)
	}
	if pred.Lock != meta.LockShared {
		t.Errorf("ロックがSHAREDであること: got %s", pred.Lock)
	}
}

// ============================================================
// PRIMARY KEY tests
// ============================================================

func TestPredictAddPrimaryKey(t *testing.T) {
	p := New()
	action := meta.AlterAction{
		Type: meta.ActionAddPrimaryKey,
		Detail: meta.ActionDetail{
			IndexColumns: []string{"id"},
		},
	}
	pred := p.Predict(action, nil)
	if pred.Algorithm != meta.AlgorithmInplace {
		t.Errorf("アルゴリズムがINPLACEであること: got %s", pred.Algorithm)
	}
	if !pred.TableRebuild {
		t.Error("テーブル再構築が必要であること")
	}
}

func TestPredictDropPrimaryKey(t *testing.T) {
	// DROP PRIMARY KEY: COPY / SHARED (not EXCLUSIVE)
	p := New()
	action := meta.AlterAction{Type: meta.ActionDropPrimaryKey}
	pred := p.Predict(action, nil)
	if pred.Algorithm != meta.AlgorithmCopy {
		t.Errorf("アルゴリズムがCOPYであること: got %s", pred.Algorithm)
	}
	if pred.Lock != meta.LockShared {
		t.Errorf("ロックがSHAREDであること: got %s", pred.Lock)
	}
	if pred.RiskLevel != meta.RiskCritical {
		t.Errorf("リスクレベルがCRITICALであること: got %s", pred.RiskLevel)
	}
}

// ============================================================
// FOREIGN KEY tests
// ============================================================

func TestPredictAddForeignKey(t *testing.T) {
	// ADD FOREIGN KEY: COPY / SHARED (default, foreign_key_checks=ON)
	p := New()
	action := meta.AlterAction{
		Type: meta.ActionAddForeignKey,
		Detail: meta.ActionDetail{
			ConstraintName: "fk_user",
			RefTable:       "users",
		},
	}
	pred := p.Predict(action, nil)
	if pred.Algorithm != meta.AlgorithmCopy {
		t.Errorf("アルゴリズムがCOPYであること: got %s", pred.Algorithm)
	}
	if pred.Lock != meta.LockShared {
		t.Errorf("ロックがSHAREDであること: got %s", pred.Lock)
	}
}

func TestPredictDropForeignKey(t *testing.T) {
	p := New()
	action := meta.AlterAction{
		Type: meta.ActionDropForeignKey,
		Detail: meta.ActionDetail{
			ConstraintName: "fk_user",
		},
	}
	pred := p.Predict(action, nil)
	if pred.Algorithm != meta.AlgorithmInplace {
		t.Errorf("アルゴリズムがINPLACEであること: got %s", pred.Algorithm)
	}
	if pred.Lock != meta.LockNone {
		t.Errorf("ロックがNONEであること: got %s", pred.Lock)
	}
}

// ============================================================
// TABLE operation tests
// ============================================================

func TestPredictRenameColumn(t *testing.T) {
	p := New()
	action := meta.AlterAction{
		Type: meta.ActionRenameColumn,
		Detail: meta.ActionDetail{
			ColumnName:    "full_name",
			OldColumnName: "name",
		},
	}
	pred := p.Predict(action, nil)
	if pred.Algorithm != meta.AlgorithmInstant {
		t.Errorf("アルゴリズムがINSTANTであること: got %s", pred.Algorithm)
	}
	if pred.RiskLevel != meta.RiskLow {
		t.Errorf("リスクレベルがLOWであること: got %s", pred.RiskLevel)
	}
}

func TestPredictChangeEngine(t *testing.T) {
	p := New()
	action := meta.AlterAction{
		Type:   meta.ActionChangeEngine,
		Detail: meta.ActionDetail{Engine: "MyISAM"},
	}
	tableMeta := &meta.TableMeta{Engine: "InnoDB"}
	pred := p.Predict(action, tableMeta)
	if pred.Algorithm != meta.AlgorithmCopy {
		t.Errorf("アルゴリズムがCOPYであること: got %s", pred.Algorithm)
	}
	if pred.Lock != meta.LockShared {
		t.Errorf("ロックがSHAREDであること: got %s", pred.Lock)
	}
	if pred.RiskLevel != meta.RiskCritical {
		t.Errorf("リスクレベルがCRITICALであること: got %s", pred.RiskLevel)
	}
}

func TestPredictChangeEngineSame(t *testing.T) {
	p := New()
	action := meta.AlterAction{
		Type:   meta.ActionChangeEngine,
		Detail: meta.ActionDetail{Engine: "InnoDB"},
	}
	tableMeta := &meta.TableMeta{Engine: "InnoDB"}
	pred := p.Predict(action, tableMeta)
	if pred.Algorithm != meta.AlgorithmInplace {
		t.Errorf("アルゴリズムがINPLACEであること: got %s", pred.Algorithm)
	}
	if pred.Lock != meta.LockNone {
		t.Errorf("ロックがNONEであること: got %s", pred.Lock)
	}
}

func TestPredictConvertCharset(t *testing.T) {
	// CONVERT CHARACTER SET: INPLACE / SHARED (not COPY/EXCLUSIVE)
	p := New()
	action := meta.AlterAction{
		Type:   meta.ActionConvertCharset,
		Detail: meta.ActionDetail{Charset: "utf8mb4"},
	}
	pred := p.Predict(action, nil)
	if pred.Algorithm != meta.AlgorithmInplace {
		t.Errorf("アルゴリズムがINPLACEであること: got %s", pred.Algorithm)
	}
	if pred.Lock != meta.LockShared {
		t.Errorf("ロックがSHAREDであること: got %s", pred.Lock)
	}
}

func TestPredictChangeRowFormat(t *testing.T) {
	p := New()
	action := meta.AlterAction{
		Type:   meta.ActionChangeRowFormat,
		Detail: meta.ActionDetail{RowFormat: "DYNAMIC"},
	}
	pred := p.Predict(action, nil)
	if pred.Algorithm != meta.AlgorithmInplace {
		t.Errorf("アルゴリズムがINPLACEであること: got %s", pred.Algorithm)
	}
	if !pred.TableRebuild {
		t.Error("テーブル再構築が必要であること")
	}
}

func TestPredictChangeKeyBlockSize(t *testing.T) {
	p := New()
	action := meta.AlterAction{Type: meta.ActionChangeKeyBlockSize}
	pred := p.Predict(action, nil)
	if pred.Algorithm != meta.AlgorithmInplace {
		t.Errorf("アルゴリズムがINPLACEであること: got %s", pred.Algorithm)
	}
	if !pred.TableRebuild {
		t.Error("テーブル再構築が必要であること")
	}
}

func TestPredictChangeAutoIncrement(t *testing.T) {
	p := New()
	action := meta.AlterAction{Type: meta.ActionChangeAutoIncrement}
	pred := p.Predict(action, nil)
	if pred.Algorithm != meta.AlgorithmInplace {
		t.Errorf("アルゴリズムがINPLACEであること: got %s", pred.Algorithm)
	}
	if pred.Lock != meta.LockNone {
		t.Errorf("ロックがNONEであること: got %s", pred.Lock)
	}
	if pred.TableRebuild {
		t.Error("テーブル再構築が不要であること")
	}
}

func TestPredictForceRebuild(t *testing.T) {
	p := New()
	action := meta.AlterAction{Type: meta.ActionForceRebuild}
	pred := p.Predict(action, nil)
	if pred.Algorithm != meta.AlgorithmInplace {
		t.Errorf("アルゴリズムがINPLACEであること: got %s", pred.Algorithm)
	}
	if pred.Lock != meta.LockNone {
		t.Errorf("ロックがNONEであること: got %s", pred.Lock)
	}
	if !pred.TableRebuild {
		t.Error("テーブル再構築が必要であること")
	}
}

// ============================================================
// PARTITION operation tests
// ============================================================

func TestPredictAddPartition(t *testing.T) {
	p := New()
	action := meta.AlterAction{Type: meta.ActionAddPartition}
	pred := p.Predict(action, nil)
	if pred.Algorithm != meta.AlgorithmInplace {
		t.Errorf("アルゴリズムがINPLACEであること: got %s", pred.Algorithm)
	}
	if pred.Lock != meta.LockNone {
		t.Errorf("ロックがNONEであること: got %s", pred.Lock)
	}
}

func TestPredictDropPartition(t *testing.T) {
	p := New()
	action := meta.AlterAction{Type: meta.ActionDropPartition}
	pred := p.Predict(action, nil)
	if pred.Algorithm != meta.AlgorithmInplace {
		t.Errorf("アルゴリズムがINPLACEであること: got %s", pred.Algorithm)
	}
	if pred.Lock != meta.LockNone {
		t.Errorf("ロックがNONEであること: got %s", pred.Lock)
	}
}

func TestPredictCoalescePartition(t *testing.T) {
	p := New()
	action := meta.AlterAction{Type: meta.ActionCoalescePartition}
	pred := p.Predict(action, nil)
	if pred.Algorithm != meta.AlgorithmInplace {
		t.Errorf("アルゴリズムがINPLACEであること: got %s", pred.Algorithm)
	}
	if pred.Lock != meta.LockShared {
		t.Errorf("ロックがSHAREDであること: got %s", pred.Lock)
	}
}

func TestPredictReorganizePartition(t *testing.T) {
	p := New()
	action := meta.AlterAction{Type: meta.ActionReorganizePartition}
	pred := p.Predict(action, nil)
	if pred.Lock != meta.LockShared {
		t.Errorf("ロックがSHAREDであること: got %s", pred.Lock)
	}
}

func TestPredictTruncatePartition(t *testing.T) {
	p := New()
	action := meta.AlterAction{Type: meta.ActionTruncatePartition}
	pred := p.Predict(action, nil)
	if pred.Lock != meta.LockNone {
		t.Errorf("ロックがNONEであること: got %s", pred.Lock)
	}
}

func TestPredictRemovePartitioning(t *testing.T) {
	p := New()
	action := meta.AlterAction{Type: meta.ActionRemovePartitioning}
	pred := p.Predict(action, nil)
	if pred.Algorithm != meta.AlgorithmCopy {
		t.Errorf("アルゴリズムがCOPYであること: got %s", pred.Algorithm)
	}
	if pred.Lock != meta.LockShared {
		t.Errorf("ロックがSHAREDであること: got %s", pred.Lock)
	}
}

func TestPredictPartitionBy(t *testing.T) {
	p := New()
	action := meta.AlterAction{Type: meta.ActionPartitionBy}
	pred := p.Predict(action, nil)
	if pred.Algorithm != meta.AlgorithmCopy {
		t.Errorf("アルゴリズムがCOPYであること: got %s", pred.Algorithm)
	}
	if pred.Lock != meta.LockShared {
		t.Errorf("ロックがSHAREDであること: got %s", pred.Lock)
	}
}

// ============================================================
// Non-InnoDB and risk level tests
// ============================================================

func TestPredictNonInnoDB(t *testing.T) {
	p := New()
	action := meta.AlterAction{
		Type: meta.ActionAddColumn,
		Detail: meta.ActionDetail{
			ColumnName: "nickname",
			IsNullable: boolPtr(true),
		},
	}
	tableMeta := &meta.TableMeta{Engine: "MyISAM"}
	pred := p.Predict(action, tableMeta)
	if pred.Algorithm != meta.AlgorithmCopy {
		t.Errorf("非InnoDBはCOPYであること: got %s", pred.Algorithm)
	}
	if pred.RiskLevel != meta.RiskCritical {
		t.Errorf("リスクレベルがCRITICALであること: got %s", pred.RiskLevel)
	}
}

func TestCollectTableInfoWithMeta(t *testing.T) {
	tm := &meta.TableMeta{
		RowCount:    1200000,
		DataLength:  500 * 1024 * 1024,
		IndexLength: 50 * 1024 * 1024,
		Indexes: []meta.IndexMeta{
			{Name: "PRIMARY"}, {Name: "idx_email"}, {Name: "idx_name"},
		},
	}
	info := CollectTableInfo(tm)
	if info.RowCount != 1200000 {
		t.Errorf("行数が1200000であること: got %d", info.RowCount)
	}
	if info.IndexCount != 3 {
		t.Errorf("インデックス数が3であること: got %d", info.IndexCount)
	}
	if info.Label == "" {
		t.Error("ラベルが空でないこと")
	}
}

func TestCollectTableInfoNoMeta(t *testing.T) {
	info := CollectTableInfo(nil)
	if info.Label != "N/A (no table metadata)" {
		t.Errorf("メタデータなしラベルであること: got %q", info.Label)
	}
}

func TestCalculateRisk(t *testing.T) {
	tests := []struct {
		algo    meta.Algorithm
		lock    meta.LockLevel
		rebuild bool
		want    meta.RiskLevel
	}{
		{meta.AlgorithmInstant, meta.LockNone, false, meta.RiskLow},
		{meta.AlgorithmInplace, meta.LockNone, false, meta.RiskMedium},
		{meta.AlgorithmInplace, meta.LockNone, true, meta.RiskHigh},
		{meta.AlgorithmCopy, meta.LockShared, true, meta.RiskCritical},
		{meta.AlgorithmCopy, meta.LockExclusive, true, meta.RiskCritical},
		{meta.AlgorithmInplace, meta.LockExclusive, false, meta.RiskCritical},
		{meta.AlgorithmInplace, meta.LockShared, false, meta.RiskMedium},
	}
	for _, tt := range tests {
		got := calculateRisk(tt.algo, tt.lock, tt.rebuild)
		if got != tt.want {
			t.Errorf("calculateRisk(%s, %s, %v) = %s, want %s", tt.algo, tt.lock, tt.rebuild, got, tt.want)
		}
	}
}

// ============================================================
// extractVarcharLength tests
// ============================================================

func TestExtractVarcharLength(t *testing.T) {
	tests := []struct {
		input string
		want  int
	}{
		{"VARCHAR(255)", 255},
		{"varchar(100)", 100},
		{"VARCHAR(256)", 256},
		{"INT", -1},
		{"TEXT", -1},
		{"VARCHAR", -1},
	}
	for _, tt := range tests {
		got := extractVarcharLength(tt.input)
		if got != tt.want {
			t.Errorf("extractVarcharLength(%q) = %d, want %d", tt.input, got, tt.want)
		}
	}
}
