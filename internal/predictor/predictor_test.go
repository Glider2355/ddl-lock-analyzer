package predictor

import (
	"testing"

	"github.com/Glider2355/ddl-lock-analyzer/internal/meta"
)

func boolPtr(b bool) *bool { return &b }

func TestPredictAddColumnTail(t *testing.T) {
	// 末尾へのカラム追加はINSTANT/NONEになることを検証
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
	// FIRST位置へのカラム追加はINSTANTになることを検証
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
	// NOT NULLカラム追加はINPLACEでテーブル再構築が必要なことを検証
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
	if pred.Algorithm != meta.AlgorithmInplace {
		t.Errorf("アルゴリズムがINPLACEであること: got %s", pred.Algorithm)
	}
	if !pred.TableRebuild {
		t.Error("テーブル再構築が必要であること")
	}
}

func TestPredictDropColumn(t *testing.T) {
	// カラム削除はINSTANT/LOWになることを検証
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

func TestPredictModifyColumnTypeChange(t *testing.T) {
	// 型変更を伴うMODIFY COLUMNはCOPY/EXCLUSIVE/CRITICALになることを検証
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
	if pred.Algorithm != meta.AlgorithmCopy {
		t.Errorf("アルゴリズムがCOPYであること: got %s", pred.Algorithm)
	}
	if pred.Lock != meta.LockExclusive {
		t.Errorf("ロックがEXCLUSIVEであること: got %s", pred.Lock)
	}
	if pred.RiskLevel != meta.RiskCritical {
		t.Errorf("リスクレベルがCRITICALであること: got %s", pred.RiskLevel)
	}
}

func TestPredictModifyColumnNullToNotNull(t *testing.T) {
	// NULL→NOT NULL変更はINPLACE/HIGHになることを検証
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
}

func TestPredictAddIndex(t *testing.T) {
	// インデックス追加はINPLACE/NONE/MEDIUMになることを検証
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

func TestPredictDropPrimaryKey(t *testing.T) {
	// 主キー削除はCOPY/CRITICALになることを検証
	p := New()
	action := meta.AlterAction{Type: meta.ActionDropPrimaryKey}
	pred := p.Predict(action, nil)
	if pred.Algorithm != meta.AlgorithmCopy {
		t.Errorf("アルゴリズムがCOPYであること: got %s", pred.Algorithm)
	}
	if pred.RiskLevel != meta.RiskCritical {
		t.Errorf("リスクレベルがCRITICALであること: got %s", pred.RiskLevel)
	}
}

func TestPredictNonInnoDB(t *testing.T) {
	// 非InnoDBエンジンはCOPY/CRITICALになることを検証
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

func TestPredictRenameColumn(t *testing.T) {
	// カラムリネームはINSTANT/LOWになることを検証
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
	// エンジン変更はCOPY/CRITICALになることを検証
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
	if pred.RiskLevel != meta.RiskCritical {
		t.Errorf("リスクレベルがCRITICALであること: got %s", pred.RiskLevel)
	}
}

func TestCollectTableInfoWithMeta(t *testing.T) {
	// メタデータありの場合のテーブル情報収集を検証
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
	// メタデータなしの場合のテーブル情報収集を検証
	info := CollectTableInfo(nil)
	if info.Label != "N/A (no table metadata)" {
		t.Errorf("メタデータなしラベルであること: got %q", info.Label)
	}
}

func TestCalculateRisk(t *testing.T) {
	// リスクレベル計算を検証
	tests := []struct {
		algo    meta.Algorithm
		lock    meta.LockLevel
		rebuild bool
		want    meta.RiskLevel
	}{
		{meta.AlgorithmInstant, meta.LockNone, false, meta.RiskLow},
		{meta.AlgorithmInplace, meta.LockNone, false, meta.RiskMedium},
		{meta.AlgorithmInplace, meta.LockNone, true, meta.RiskHigh},
		{meta.AlgorithmCopy, meta.LockExclusive, true, meta.RiskCritical},
		{meta.AlgorithmInplace, meta.LockExclusive, false, meta.RiskCritical},
	}
	for _, tt := range tests {
		got := calculateRisk(tt.algo, tt.lock, tt.rebuild)
		if got != tt.want {
			t.Errorf("calculateRisk(%s, %s, %v) = %s, want %s", tt.algo, tt.lock, tt.rebuild, got, tt.want)
		}
	}
}
