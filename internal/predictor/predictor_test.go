package predictor

import (
	"testing"

	"github.com/Glider2355/ddl-lock-analyzer/internal/meta"
)

func boolPtr(b bool) *bool { return &b }

// ============================================================
// ADD COLUMN tests
// MySQL公式ドキュメント:
//   - カラム操作: https://dev.mysql.com/doc/refman/8.0/en/innodb-online-ddl-operations.html#online-ddl-column-operations
//   - 生成カラム: https://dev.mysql.com/doc/refman/8.0/en/innodb-online-ddl-operations.html#online-ddl-generated-column-operations
// ============================================================

// TestPredictAddColumnTail — 末尾へのNULLABLEカラム追加はINSTANT
// MySQL docs: ADD COLUMN → INSTANT, Concurrent DML=Yes, Rebuilds Table=No
// https://dev.mysql.com/doc/refman/8.0/en/innodb-online-ddl-operations.html#online-ddl-column-operations
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

// TestPredictAddColumnFirst — FIRST指定のカラム追加はINSTANT (MySQL 8.0.29+)
// MySQL docs: ADD COLUMN at any position → INSTANT (8.0.29+)
// https://dev.mysql.com/doc/refman/8.0/en/innodb-online-ddl-operations.html#online-ddl-column-operations
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

// TestPredictAddColumnNotNull — NOT NULLカラム追加はINSTANT (MySQL 8.0.12+)
// MySQL docs: ADD COLUMN (NOT NULL with DEFAULT) → INSTANT
// https://dev.mysql.com/doc/refman/8.0/en/innodb-online-ddl-operations.html#online-ddl-column-operations
func TestPredictAddColumnNotNull(t *testing.T) {
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

// TestPredictAddColumnAutoIncrement — AUTO_INCREMENTカラム追加はINPLACE/SHARED/テーブル再構築
// MySQL docs: ADD COLUMN (auto-increment) → INPLACE, Concurrent DML=No, Rebuilds Table=Yes
// https://dev.mysql.com/doc/refman/8.0/en/innodb-online-ddl-operations.html#online-ddl-column-operations
func TestPredictAddColumnAutoIncrement(t *testing.T) {
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
	if !pred.TableRebuild {
		t.Error("テーブル再構築が必要であること")
	}
}

// TestPredictAddColumnStoredGenerated — STORED生成カラム追加はCOPY/SHARED
// MySQL docs: Add STORED column → COPY, Concurrent DML=No, Rebuilds Table=Yes
// https://dev.mysql.com/doc/refman/8.0/en/innodb-online-ddl-operations.html#online-ddl-generated-column-operations
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

// TestPredictAddColumnVirtualGenerated — VIRTUAL生成カラム追加（非パーティション）はINSTANT
// MySQL docs: Add VIRTUAL column → INSTANT, Concurrent DML=Yes, Rebuilds Table=No (non-partitioned)
// https://dev.mysql.com/doc/refman/8.0/en/innodb-online-ddl-operations.html#online-ddl-generated-column-operations
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

// TestPredictAddColumnVirtualGeneratedPartitioned — VIRTUAL生成カラム追加（パーティション）はINPLACE
// MySQL docs: Add VIRTUAL column on partitioned table → INPLACE (not INSTANT)
// https://dev.mysql.com/doc/refman/8.0/en/innodb-online-ddl-operations.html#online-ddl-generated-column-operations
func TestPredictAddColumnVirtualGeneratedPartitioned(t *testing.T) {
	p := New()
	action := meta.AlterAction{
		Type: meta.ActionAddColumn,
		Detail: meta.ActionDetail{
			ColumnName:    "full_name",
			ColumnType:    "VARCHAR(512)",
			GeneratedType: "VIRTUAL",
		},
	}
	tableMeta := &meta.TableMeta{
		Engine:        "InnoDB",
		IsPartitioned: true,
		PartitionType: "RANGE",
	}
	pred := p.Predict(action, tableMeta)
	if pred.Algorithm != meta.AlgorithmInplace {
		t.Errorf("パーティションテーブルではINPLACEであること: got %s", pred.Algorithm)
	}
}

// ============================================================
// DROP COLUMN tests
// MySQL公式ドキュメント:
//   - カラム操作: https://dev.mysql.com/doc/refman/8.0/en/innodb-online-ddl-operations.html#online-ddl-column-operations
//   - 生成カラム: https://dev.mysql.com/doc/refman/8.0/en/innodb-online-ddl-operations.html#online-ddl-generated-column-operations
// ============================================================

// TestPredictDropColumn — 通常カラム削除はINSTANT/テーブル再構築あり
// MySQL docs: Drop column → INSTANT, Concurrent DML=Yes, Rebuilds Table=Yes
// https://dev.mysql.com/doc/refman/8.0/en/innodb-online-ddl-operations.html#online-ddl-column-operations
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
	if !pred.TableRebuild {
		t.Error("テーブル再構築が必要であること（既存行のデータは遅延再構築）")
	}
}

// TestPredictDropColumnStoredGenerated — STORED生成カラム削除はINPLACE/テーブル再構築
// MySQL docs: Drop STORED column → INPLACE, Concurrent DML=Yes, Rebuilds Table=Yes
// https://dev.mysql.com/doc/refman/8.0/en/innodb-online-ddl-operations.html#online-ddl-generated-column-operations
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

// TestPredictDropColumnVirtualGenerated — VIRTUAL生成カラム削除はINSTANT/再構築なし
// MySQL docs: Drop VIRTUAL column → INSTANT, Concurrent DML=Yes, Rebuilds Table=No
// https://dev.mysql.com/doc/refman/8.0/en/innodb-online-ddl-operations.html#online-ddl-generated-column-operations
func TestPredictDropColumnVirtualGenerated(t *testing.T) {
	p := New()
	action := meta.AlterAction{
		Type:   meta.ActionDropColumn,
		Detail: meta.ActionDetail{ColumnName: "virt_col"},
	}
	tableMeta := &meta.TableMeta{
		Engine: "InnoDB",
		Columns: []meta.ColumnMeta{
			{Name: "virt_col", ColumnType: "INT", Extra: "VIRTUAL GENERATED"},
		},
	}
	pred := p.Predict(action, tableMeta)
	if pred.Algorithm != meta.AlgorithmInstant {
		t.Errorf("アルゴリズムがINSTANTであること: got %s", pred.Algorithm)
	}
	if pred.TableRebuild {
		t.Error("VIRTUAL生成カラム削除はテーブル再構築不要であること")
	}
}

// ============================================================
// MODIFY COLUMN tests
// MySQL公式ドキュメント:
//   https://dev.mysql.com/doc/refman/8.0/en/innodb-online-ddl-operations.html#online-ddl-column-operations
// ============================================================

// TestPredictModifyColumnTypeChange — データ型変更はCOPY/SHARED
// MySQL docs: Change column data type → COPY, Concurrent DML=No, Rebuilds Table=Yes
// https://dev.mysql.com/doc/refman/8.0/en/innodb-online-ddl-operations.html#online-ddl-column-operations
func TestPredictModifyColumnTypeChange(t *testing.T) {
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

// TestPredictModifyColumnVarcharExtension — VARCHAR拡張（同一バイト境界内）はINPLACE/NONE
// MySQL docs: Extend VARCHAR size → INPLACE, Concurrent DML=Yes, Rebuilds Table=No
// https://dev.mysql.com/doc/refman/8.0/en/innodb-online-ddl-operations.html#online-ddl-column-operations
func TestPredictModifyColumnVarcharExtension(t *testing.T) {
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

// TestPredictModifyColumnVarcharCrossBoundary — VARCHAR拡張（バイト境界超え）はCOPY
// MySQL docs: VARCHAR 255→256 crosses length-byte boundary → COPY
// https://dev.mysql.com/doc/refman/8.0/en/innodb-online-ddl-operations.html#online-ddl-column-operations
func TestPredictModifyColumnVarcharCrossBoundary(t *testing.T) {
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

// TestPredictModifyColumnNullToNotNull — NULL→NOT NULL変換はINPLACE/テーブル再構築
// MySQL docs: Make column NOT NULL → INPLACE, Concurrent DML=Yes, Rebuilds Table=Yes
// https://dev.mysql.com/doc/refman/8.0/en/innodb-online-ddl-operations.html#online-ddl-column-operations
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

// TestPredictModifyColumnNotNullToNull — NOT NULL→NULL変換はINPLACE/テーブル再構築
// MySQL docs: Make column NULL → INPLACE, Concurrent DML=Yes, Rebuilds Table=Yes
// https://dev.mysql.com/doc/refman/8.0/en/innodb-online-ddl-operations.html#online-ddl-column-operations
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

// TestPredictModifyColumnReorder — カラム並べ替えはINPLACE/テーブル再構築
// MySQL docs: Reorder columns → INPLACE, Concurrent DML=Yes, Rebuilds Table=Yes
// https://dev.mysql.com/doc/refman/8.0/en/innodb-online-ddl-operations.html#online-ddl-column-operations
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

// TestPredictModifyColumnEnumExtension — ENUM末尾追加はINSTANT
// MySQL docs: Modify ENUM/SET column → INSTANT, Concurrent DML=Yes, Rebuilds Table=No
// https://dev.mysql.com/doc/refman/8.0/en/innodb-online-ddl-operations.html#online-ddl-column-operations
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
// MySQL公式ドキュメント:
//   https://dev.mysql.com/doc/refman/8.0/en/innodb-online-ddl-operations.html#online-ddl-column-operations
// ============================================================

// TestPredictChangeColumnRenameOnly — リネームのみはINSTANT
// MySQL docs: Rename column (same type) → INSTANT (8.0.28+), Concurrent DML=Yes, Rebuilds Table=No
// https://dev.mysql.com/doc/refman/8.0/en/innodb-online-ddl-operations.html#online-ddl-column-operations
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

// TestPredictChangeColumnTypeChange — 型変更はCOPY/SHARED
// MySQL docs: Change column data type → COPY, Concurrent DML=No, Rebuilds Table=Yes
// https://dev.mysql.com/doc/refman/8.0/en/innodb-online-ddl-operations.html#online-ddl-column-operations
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

// TestPredictChangeColumnNoMetadata — メタデータなしは保守的にCOPY
// MySQL docs: Without metadata, assumes type change → COPY
// https://dev.mysql.com/doc/refman/8.0/en/innodb-online-ddl-operations.html#online-ddl-column-operations
func TestPredictChangeColumnNoMetadata(t *testing.T) {
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
// MySQL公式ドキュメント:
//   https://dev.mysql.com/doc/refman/8.0/en/innodb-online-ddl-operations.html#online-ddl-index-operations
// ============================================================

// TestPredictAddIndex — セカンダリインデックス追加はINPLACE/NONE
// MySQL docs: Create/Add secondary index → INPLACE, Concurrent DML=Yes, Rebuilds Table=No
// https://dev.mysql.com/doc/refman/8.0/en/innodb-online-ddl-operations.html#online-ddl-index-operations
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

// TestPredictAddFulltextIndexFirst — 最初のFULLTEXTインデックスはINPLACE/SHARED/テーブル再構築
// MySQL docs: Add FULLTEXT index → INPLACE, Concurrent DML=No, Rebuilds Table=No (Yes if no FTS_DOC_ID)
// https://dev.mysql.com/doc/refman/8.0/en/innodb-online-ddl-operations.html#online-ddl-index-operations
func TestPredictAddFulltextIndexFirst(t *testing.T) {
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

// TestPredictAddFulltextIndexSubsequent — 後続のFULLTEXTインデックスはテーブル再構築不要
// MySQL docs: Add FULLTEXT index (subsequent) → INPLACE, Concurrent DML=No, Rebuilds Table=No
// https://dev.mysql.com/doc/refman/8.0/en/innodb-online-ddl-operations.html#online-ddl-index-operations
func TestPredictAddFulltextIndexSubsequent(t *testing.T) {
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

// TestPredictAddSpatialIndex — SPATIALインデックス追加はINPLACE/SHARED
// MySQL docs: Add SPATIAL index → INPLACE, Concurrent DML=No, Rebuilds Table=No
// https://dev.mysql.com/doc/refman/8.0/en/innodb-online-ddl-operations.html#online-ddl-index-operations
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
// MySQL公式ドキュメント:
//   https://dev.mysql.com/doc/refman/8.0/en/innodb-online-ddl-operations.html#online-ddl-primary-key-operations
// ============================================================

// TestPredictAddPrimaryKey — 主キー追加はINPLACE/テーブル再構築
// MySQL docs: Add primary key → INPLACE, Concurrent DML=Yes, Rebuilds Table=Yes
// https://dev.mysql.com/doc/refman/8.0/en/innodb-online-ddl-operations.html#online-ddl-primary-key-operations
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

// TestPredictDropPrimaryKey — 主キー削除はCOPY/SHARED
// MySQL docs: Drop primary key → COPY, Concurrent DML=No, Rebuilds Table=Yes
// https://dev.mysql.com/doc/refman/8.0/en/innodb-online-ddl-operations.html#online-ddl-primary-key-operations
func TestPredictDropPrimaryKey(t *testing.T) {
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
// MySQL公式ドキュメント:
//   https://dev.mysql.com/doc/refman/8.0/en/innodb-online-ddl-operations.html#online-ddl-foreign-key-operations
// ============================================================

// TestPredictAddForeignKey — 外部キー追加はCOPY/SHARED (foreign_key_checks=ON)
// MySQL docs: Add foreign key → INPLACE (only if foreign_key_checks=OFF), otherwise COPY
// https://dev.mysql.com/doc/refman/8.0/en/innodb-online-ddl-operations.html#online-ddl-foreign-key-operations
func TestPredictAddForeignKey(t *testing.T) {
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

// TestPredictDropForeignKey — 外部キー削除はINPLACE/NONE
// MySQL docs: Drop foreign key → INPLACE, Concurrent DML=Yes, Rebuilds Table=No
// https://dev.mysql.com/doc/refman/8.0/en/innodb-online-ddl-operations.html#online-ddl-foreign-key-operations
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
// RENAME COLUMN tests
// MySQL公式ドキュメント:
//   https://dev.mysql.com/doc/refman/8.0/en/innodb-online-ddl-operations.html#online-ddl-column-operations
// ============================================================

// TestPredictRenameColumn — カラムリネームはINSTANT
// MySQL docs: Rename column → INSTANT (8.0.28+), Concurrent DML=Yes, Rebuilds Table=No
// https://dev.mysql.com/doc/refman/8.0/en/innodb-online-ddl-operations.html#online-ddl-column-operations
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

// TestPredictRenameColumnReferencedByFK — FK参照カラムのリネームはINPLACE（INSTANTではない）
// MySQL docs: Rename column referenced by FK → INPLACE (INSTANT not permitted)
// https://dev.mysql.com/doc/refman/8.0/en/innodb-online-ddl-operations.html#online-ddl-column-operations
func TestPredictRenameColumnReferencedByFK(t *testing.T) {
	p := New()
	action := meta.AlterAction{
		Type: meta.ActionRenameColumn,
		Detail: meta.ActionDetail{
			ColumnName:    "user_id_new",
			OldColumnName: "user_id",
		},
	}
	tableMeta := &meta.TableMeta{
		Engine: "InnoDB",
		ReferencedBy: []meta.ForeignKeyMeta{
			{
				ConstraintName:    "fk_orders_user",
				SourceTable:       "orders",
				ReferencedTable:   "users",
				ReferencedColumns: []string{"user_id"},
			},
		},
	}
	pred := p.Predict(action, tableMeta)
	if pred.Algorithm != meta.AlgorithmInplace {
		t.Errorf("FK参照カラムはINPLACEであること: got %s", pred.Algorithm)
	}
}

// ============================================================
// TABLE operation tests
// MySQL公式ドキュメント:
//   https://dev.mysql.com/doc/refman/8.0/en/innodb-online-ddl-operations.html#online-ddl-table-operations
// ============================================================

// TestPredictChangeEngine — エンジン変更（異なるエンジン）はCOPY/SHARED
// MySQL docs: Change storage engine (different) → COPY, Concurrent DML=No, Rebuilds Table=Yes
// https://dev.mysql.com/doc/refman/8.0/en/innodb-online-ddl-operations.html#online-ddl-table-operations
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

// TestPredictChangeEngineSame — 同一エンジン再指定（null rebuild）はINPLACE/NONE
// MySQL docs: Null rebuild (ENGINE=InnoDB on InnoDB table) → INPLACE, Concurrent DML=Yes, Rebuilds Table=Yes
// https://dev.mysql.com/doc/refman/8.0/en/innodb-online-ddl-operations.html#online-ddl-table-operations
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

// TestPredictConvertCharset — CONVERT TO CHARACTER SETはINPLACE/SHARED
// MySQL docs: Convert character set → INPLACE, Concurrent DML=No, Rebuilds Table=Yes
// https://dev.mysql.com/doc/refman/8.0/en/innodb-online-ddl-operations.html#online-ddl-table-operations
func TestPredictConvertCharset(t *testing.T) {
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

// TestPredictSpecifyCharset — Specify CHARACTER SET（CONVERT TOではない）はINPLACE/NONE
// MySQL docs: Specify character set → INPLACE, Concurrent DML=Yes, Rebuilds Table=Yes
// https://dev.mysql.com/doc/refman/8.0/en/innodb-online-ddl-operations.html#online-ddl-table-operations
func TestPredictSpecifyCharset(t *testing.T) {
	p := New()
	action := meta.AlterAction{
		Type:   meta.ActionSpecifyCharset,
		Detail: meta.ActionDetail{Charset: "utf8mb4"},
	}
	pred := p.Predict(action, nil)
	if pred.Algorithm != meta.AlgorithmInplace {
		t.Errorf("アルゴリズムがINPLACEであること: got %s", pred.Algorithm)
	}
	if pred.Lock != meta.LockNone {
		t.Errorf("ロックがNONEであること（CONVERT TOと異なりDML許可）: got %s", pred.Lock)
	}
	if !pred.TableRebuild {
		t.Error("テーブル再構築が必要であること")
	}
}

// TestPredictChangeRowFormat — ROW_FORMAT変更はINPLACE/テーブル再構築
// MySQL docs: Change ROW_FORMAT → INPLACE, Concurrent DML=Yes, Rebuilds Table=Yes
// https://dev.mysql.com/doc/refman/8.0/en/innodb-online-ddl-operations.html#online-ddl-table-operations
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

// TestPredictChangeKeyBlockSize — KEY_BLOCK_SIZE変更はINPLACE/テーブル再構築
// MySQL docs: Change KEY_BLOCK_SIZE → INPLACE, Concurrent DML=Yes, Rebuilds Table=Yes
// https://dev.mysql.com/doc/refman/8.0/en/innodb-online-ddl-operations.html#online-ddl-table-operations
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

// TestPredictChangeAutoIncrement — AUTO_INCREMENT値変更はINPLACE/再構築なし
// MySQL docs: Change auto-increment value → INPLACE, Concurrent DML=Yes, Rebuilds Table=No
// https://dev.mysql.com/doc/refman/8.0/en/innodb-online-ddl-operations.html#online-ddl-column-operations
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

// TestPredictForceRebuild — ALTER TABLE ... FORCEはINPLACE/テーブル再構築
// MySQL docs: Rebuild with FORCE → INPLACE, Concurrent DML=Yes, Rebuilds Table=Yes
// https://dev.mysql.com/doc/refman/8.0/en/innodb-online-ddl-operations.html#online-ddl-table-operations
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

// TestPredictSetTableStats — テーブル統計設定はINPLACE/メタデータのみ
// MySQL docs: Set persistent table statistics → INPLACE, Concurrent DML=Yes, Rebuilds Table=No
// https://dev.mysql.com/doc/refman/8.0/en/innodb-online-ddl-operations.html#online-ddl-table-operations
func TestPredictSetTableStats(t *testing.T) {
	p := New()
	action := meta.AlterAction{Type: meta.ActionSetTableStats}
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

// TestPredictTableEncryption — テーブル暗号化はCOPY/SHARED
// MySQL docs: Enable/disable file-per-table encryption → COPY, Concurrent DML=No, Rebuilds Table=Yes
// https://dev.mysql.com/doc/refman/8.0/en/innodb-online-ddl-operations.html#online-ddl-table-operations
func TestPredictTableEncryption(t *testing.T) {
	p := New()
	action := meta.AlterAction{Type: meta.ActionTableEncryption}
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
	if pred.RiskLevel != meta.RiskCritical {
		t.Errorf("リスクレベルがCRITICALであること: got %s", pred.RiskLevel)
	}
}

// ============================================================
// PARTITION operation tests
// MySQL公式ドキュメント:
//   https://dev.mysql.com/doc/refman/8.0/en/innodb-online-ddl-operations.html#online-ddl-partitioning-operations
// ============================================================

// TestPredictAddPartition — RANGE/LISTパーティション追加はINPLACE/NONE
// MySQL docs: ADD PARTITION (RANGE/LIST) → INPLACE, Concurrent DML=Yes, Rebuilds Table=No
// https://dev.mysql.com/doc/refman/8.0/en/innodb-online-ddl-operations.html#online-ddl-partitioning-operations
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

// TestPredictAddPartitionHash — HASH/KEYパーティション追加はINPLACE/SHARED
// MySQL docs: ADD PARTITION (HASH/KEY) → INPLACE, LOCK=SHARED minimum (data redistribution)
// https://dev.mysql.com/doc/refman/8.0/en/innodb-online-ddl-operations.html#online-ddl-partitioning-operations
func TestPredictAddPartitionHash(t *testing.T) {
	p := New()
	action := meta.AlterAction{Type: meta.ActionAddPartition}
	tableMeta := &meta.TableMeta{
		Engine:        "InnoDB",
		IsPartitioned: true,
		PartitionType: "HASH",
	}
	pred := p.Predict(action, tableMeta)
	if pred.Algorithm != meta.AlgorithmInplace {
		t.Errorf("アルゴリズムがINPLACEであること: got %s", pred.Algorithm)
	}
	if pred.Lock != meta.LockShared {
		t.Errorf("HASH/KEYパーティションはSHAREDロックであること: got %s", pred.Lock)
	}
}

// TestPredictDropPartition — RANGE/LISTパーティション削除はINPLACE/NONE
// MySQL docs: DROP PARTITION (RANGE/LIST) → INPLACE, Concurrent DML=Yes, Rebuilds Table=No
// https://dev.mysql.com/doc/refman/8.0/en/innodb-online-ddl-operations.html#online-ddl-partitioning-operations
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

// TestPredictDropPartitionKey — KEY パーティション削除はINPLACE/SHARED
// MySQL docs: DROP PARTITION (HASH/KEY) → INPLACE, LOCK=SHARED minimum (data redistribution)
// https://dev.mysql.com/doc/refman/8.0/en/innodb-online-ddl-operations.html#online-ddl-partitioning-operations
func TestPredictDropPartitionKey(t *testing.T) {
	p := New()
	action := meta.AlterAction{Type: meta.ActionDropPartition}
	tableMeta := &meta.TableMeta{
		Engine:        "InnoDB",
		IsPartitioned: true,
		PartitionType: "KEY",
	}
	pred := p.Predict(action, tableMeta)
	if pred.Lock != meta.LockShared {
		t.Errorf("KEY パーティションはSHAREDロックであること: got %s", pred.Lock)
	}
}

// TestPredictCoalescePartition — パーティション統合はINPLACE/SHARED
// MySQL docs: COALESCE PARTITION → INPLACE, Concurrent DML=No, LOCK=SHARED minimum
// https://dev.mysql.com/doc/refman/8.0/en/innodb-online-ddl-operations.html#online-ddl-partitioning-operations
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

// TestPredictReorganizePartition — パーティション再編成はINPLACE/SHARED
// MySQL docs: REORGANIZE PARTITION → INPLACE, Concurrent DML=No, LOCK=SHARED minimum
// https://dev.mysql.com/doc/refman/8.0/en/innodb-online-ddl-operations.html#online-ddl-partitioning-operations
func TestPredictReorganizePartition(t *testing.T) {
	p := New()
	action := meta.AlterAction{Type: meta.ActionReorganizePartition}
	pred := p.Predict(action, nil)
	if pred.Lock != meta.LockShared {
		t.Errorf("ロックがSHAREDであること: got %s", pred.Lock)
	}
}

// TestPredictTruncatePartition — パーティションTRUNCATEはINPLACE/NONE
// MySQL docs: TRUNCATE PARTITION → INPLACE, Concurrent DML=Yes, Rebuilds Table=No
// https://dev.mysql.com/doc/refman/8.0/en/innodb-online-ddl-operations.html#online-ddl-partitioning-operations
func TestPredictTruncatePartition(t *testing.T) {
	p := New()
	action := meta.AlterAction{Type: meta.ActionTruncatePartition}
	pred := p.Predict(action, nil)
	if pred.Lock != meta.LockNone {
		t.Errorf("ロックがNONEであること: got %s", pred.Lock)
	}
}

// TestPredictRemovePartitioning — パーティション削除はCOPY/SHARED
// MySQL docs: REMOVE PARTITIONING → COPY, Concurrent DML=No, Rebuilds Table=Yes
// https://dev.mysql.com/doc/refman/8.0/en/innodb-online-ddl-operations.html#online-ddl-partitioning-operations
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

// TestPredictPartitionBy — PARTITION BYはCOPY/SHARED
// MySQL docs: PARTITION BY → COPY, Concurrent DML=No, Rebuilds Table=Yes
// https://dev.mysql.com/doc/refman/8.0/en/innodb-online-ddl-operations.html#online-ddl-partitioning-operations
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

// TestPredictCheckPartition — CHECK PARTITIONはINPLACE/NONE
// MySQL docs: CHECK PARTITION → INPLACE, Concurrent DML=Yes, Rebuilds Table=No
// https://dev.mysql.com/doc/refman/8.0/en/innodb-online-ddl-operations.html#online-ddl-partitioning-operations
func TestPredictCheckPartition(t *testing.T) {
	p := New()
	action := meta.AlterAction{Type: meta.ActionCheckPartition}
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

// TestPredictOptimizePartition — OPTIMIZE PARTITIONはCOPY/SHARED/テーブル全体再構築
// MySQL docs: OPTIMIZE PARTITION → rebuilds entire table, ALGORITHM/LOCK clauses ignored
// https://dev.mysql.com/doc/refman/8.0/en/innodb-online-ddl-operations.html#online-ddl-partitioning-operations
func TestPredictOptimizePartition(t *testing.T) {
	p := New()
	action := meta.AlterAction{Type: meta.ActionOptimizePartition}
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

// TestPredictRepairPartition — REPAIR PARTITIONはINPLACE/NONE
// MySQL docs: REPAIR PARTITION → INPLACE, Concurrent DML=Yes, Rebuilds Table=No
// https://dev.mysql.com/doc/refman/8.0/en/innodb-online-ddl-operations.html#online-ddl-partitioning-operations
func TestPredictRepairPartition(t *testing.T) {
	p := New()
	action := meta.AlterAction{Type: meta.ActionRepairPartition}
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

// TestPredictDiscardPartitionTablespace — DISCARD PARTITION TABLESPACEはEXCLUSIVEロック
// MySQL docs: DISCARD PARTITION → only ALGORITHM=DEFAULT, LOCK=DEFAULT, Concurrent DML=No
// https://dev.mysql.com/doc/refman/8.0/en/innodb-online-ddl-operations.html#online-ddl-partitioning-operations
func TestPredictDiscardPartitionTablespace(t *testing.T) {
	p := New()
	action := meta.AlterAction{Type: meta.ActionDiscardPartitionTablespace}
	pred := p.Predict(action, nil)
	if pred.Lock != meta.LockExclusive {
		t.Errorf("ロックがEXCLUSIVEであること: got %s", pred.Lock)
	}
	if pred.RiskLevel != meta.RiskCritical {
		t.Errorf("リスクレベルがCRITICALであること: got %s", pred.RiskLevel)
	}
}

// TestPredictImportPartitionTablespace — IMPORT PARTITION TABLESPACEはEXCLUSIVEロック
// MySQL docs: IMPORT PARTITION → only ALGORITHM=DEFAULT, LOCK=DEFAULT, Concurrent DML=No
// https://dev.mysql.com/doc/refman/8.0/en/innodb-online-ddl-operations.html#online-ddl-partitioning-operations
func TestPredictImportPartitionTablespace(t *testing.T) {
	p := New()
	action := meta.AlterAction{Type: meta.ActionImportPartitionTablespace}
	pred := p.Predict(action, nil)
	if pred.Lock != meta.LockExclusive {
		t.Errorf("ロックがEXCLUSIVEであること: got %s", pred.Lock)
	}
	if pred.RiskLevel != meta.RiskCritical {
		t.Errorf("リスクレベルがCRITICALであること: got %s", pred.RiskLevel)
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
