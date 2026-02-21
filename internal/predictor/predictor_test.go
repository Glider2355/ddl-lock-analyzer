package predictor

import (
	"testing"

	"github.com/Glider2355/ddl-lock-analyzer/internal/meta"
)

func boolPtr(b bool) *bool { return &b }

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
		t.Errorf("expected INSTANT, got %s", pred.Algorithm)
	}
	if pred.Lock != meta.LockNone {
		t.Errorf("expected NONE, got %s", pred.Lock)
	}
	if pred.RiskLevel != meta.RiskLow {
		t.Errorf("expected LOW, got %s", pred.RiskLevel)
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
		t.Errorf("expected INSTANT, got %s", pred.Algorithm)
	}
}

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
	if pred.Algorithm != meta.AlgorithmInplace {
		t.Errorf("expected INPLACE, got %s", pred.Algorithm)
	}
	if !pred.TableRebuild {
		t.Error("expected table rebuild")
	}
}

func TestPredictDropColumn(t *testing.T) {
	p := New()
	action := meta.AlterAction{
		Type:   meta.ActionDropColumn,
		Detail: meta.ActionDetail{ColumnName: "nickname"},
	}
	pred := p.Predict(action, nil)
	if pred.Algorithm != meta.AlgorithmInstant {
		t.Errorf("expected INSTANT, got %s", pred.Algorithm)
	}
	if pred.RiskLevel != meta.RiskLow {
		t.Errorf("expected LOW, got %s", pred.RiskLevel)
	}
}

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
	if pred.Algorithm != meta.AlgorithmCopy {
		t.Errorf("expected COPY, got %s", pred.Algorithm)
	}
	if pred.Lock != meta.LockExclusive {
		t.Errorf("expected EXCLUSIVE, got %s", pred.Lock)
	}
	if pred.RiskLevel != meta.RiskCritical {
		t.Errorf("expected CRITICAL, got %s", pred.RiskLevel)
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
		t.Errorf("expected INPLACE, got %s", pred.Algorithm)
	}
	if pred.RiskLevel != meta.RiskHigh {
		t.Errorf("expected HIGH, got %s", pred.RiskLevel)
	}
}

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
		t.Errorf("expected INPLACE, got %s", pred.Algorithm)
	}
	if pred.Lock != meta.LockNone {
		t.Errorf("expected NONE, got %s", pred.Lock)
	}
	if pred.RiskLevel != meta.RiskMedium {
		t.Errorf("expected MEDIUM, got %s", pred.RiskLevel)
	}
}

func TestPredictDropPrimaryKey(t *testing.T) {
	p := New()
	action := meta.AlterAction{Type: meta.ActionDropPrimaryKey}
	pred := p.Predict(action, nil)
	if pred.Algorithm != meta.AlgorithmCopy {
		t.Errorf("expected COPY, got %s", pred.Algorithm)
	}
	if pred.RiskLevel != meta.RiskCritical {
		t.Errorf("expected CRITICAL, got %s", pred.RiskLevel)
	}
}

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
		t.Errorf("expected COPY for non-InnoDB, got %s", pred.Algorithm)
	}
	if pred.RiskLevel != meta.RiskCritical {
		t.Errorf("expected CRITICAL, got %s", pred.RiskLevel)
	}
}

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
		t.Errorf("expected INSTANT, got %s", pred.Algorithm)
	}
	if pred.RiskLevel != meta.RiskLow {
		t.Errorf("expected LOW, got %s", pred.RiskLevel)
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
		t.Errorf("expected COPY, got %s", pred.Algorithm)
	}
	if pred.RiskLevel != meta.RiskCritical {
		t.Errorf("expected CRITICAL, got %s", pred.RiskLevel)
	}
}

func TestEstimateDurationInstant(t *testing.T) {
	est := EstimateDuration(meta.AlgorithmInstant, false, &meta.TableMeta{})
	if est.MinSeconds != 0 || est.MaxSeconds != 0 {
		t.Errorf("expected 0s for INSTANT, got min=%f max=%f", est.MinSeconds, est.MaxSeconds)
	}
}

func TestEstimateDurationNoMeta(t *testing.T) {
	est := EstimateDuration(meta.AlgorithmCopy, true, nil)
	if est.Label != "N/A (no table metadata)" {
		t.Errorf("expected no-metadata label, got %q", est.Label)
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
