package reporter

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/Glider2355/ddl-lock-analyzer/internal/meta"
	"github.com/Glider2355/ddl-lock-analyzer/internal/predictor"
)

func TestTextReporterBasic(t *testing.T) {
	// テキストレポーターの基本出力を検証
	r := NewTextReporter()
	report := &Report{
		Analyses: []AnalysisResult{
			{
				Table: "mydb.users",
				SQL:   "ALTER TABLE users ADD COLUMN nickname VARCHAR(255)",
				Predictions: []predictor.Prediction{
					{
						ActionType:   meta.ActionAddColumn,
						Description:  "ADD COLUMN (trailing, NULLABLE)",
						Algorithm:    meta.AlgorithmInstant,
						Lock:         meta.LockNone,
						TableRebuild: false,
						RiskLevel:    meta.RiskLow,
						TableInfo:    predictor.TableInfo{Label: "N/A (no table metadata)"},
						Notes:        []string{"INSTANT algorithm available (MySQL 8.0.12+)"},
					},
				},
			},
		},
	}

	output, err := r.Render(report)
	if err != nil {
		t.Fatal(err)
	}

	checks := []string{
		"DDL Lock Analysis Report",
		"mydb.users",
		"ADD COLUMN",
		"INSTANT",
		"NONE",
		"LOW",
		"N/A",
	}
	for _, check := range checks {
		if !strings.Contains(output, check) {
			t.Errorf("出力に%qが含まれること", check)
		}
	}
}

func TestTextReporterCritical(t *testing.T) {
	// CRITICALリスクの出力を検証
	r := NewTextReporter()
	report := &Report{
		Analyses: []AnalysisResult{
			{
				Table: "mydb.users",
				SQL:   "ALTER TABLE users MODIFY COLUMN email VARCHAR(512)",
				Predictions: []predictor.Prediction{
					{
						ActionType:   meta.ActionModifyColumn,
						Description:  "MODIFY COLUMN (type change)",
						Algorithm:    meta.AlgorithmCopy,
						Lock:         meta.LockExclusive,
						TableRebuild: true,
						RiskLevel:    meta.RiskCritical,
						TableInfo:    predictor.TableInfo{RowCount: 1200000, DataSize: 500 * 1024 * 1024, IndexSize: 50 * 1024 * 1024, IndexCount: 3, Label: "rows: ~1,200,000, data: 524MB, indexes: 3"},
						Warnings:     []string{"EXCLUSIVE lock will block all DML"},
					},
				},
			},
		},
	}

	output, err := r.Render(report)
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(output, "CRITICAL") {
		t.Error("出力にCRITICALが含まれること")
	}
	if !strings.Contains(output, "EXCLUSIVE") {
		t.Error("出力にEXCLUSIVEが含まれること")
	}
	if !strings.Contains(output, "Warning") {
		t.Error("出力にWarningセクションが含まれること")
	}
}

func TestJSONReporterBasic(t *testing.T) {
	// JSONレポーターの基本出力を検証
	r := NewJSONReporter()
	report := &Report{
		Analyses: []AnalysisResult{
			{
				Table: "mydb.users",
				SQL:   "ALTER TABLE users ADD COLUMN nickname VARCHAR(255)",
				Predictions: []predictor.Prediction{
					{
						ActionType:   meta.ActionAddColumn,
						Description:  "ADD COLUMN",
						Algorithm:    meta.AlgorithmInstant,
						Lock:         meta.LockNone,
						TableRebuild: false,
						RiskLevel:    meta.RiskLow,
						TableInfo:    predictor.TableInfo{Label: "N/A (no table metadata)"},
					},
				},
			},
		},
	}

	output, err := r.Render(report)
	if err != nil {
		t.Fatal(err)
	}

	var result jsonOutput
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Fatalf("不正なJSON出力: %v", err)
	}

	if len(result.Analyses) != 1 {
		t.Fatalf("分析結果が1件であること: got %d", len(result.Analyses))
	}
	a := result.Analyses[0]
	if a.Algorithm != meta.AlgorithmInstant {
		t.Errorf("アルゴリズムがINSTANTであること: got %s", a.Algorithm)
	}
	if a.RiskLevel != meta.RiskLow {
		t.Errorf("リスクレベルがLOWであること: got %s", a.RiskLevel)
	}
}

func TestWorstRiskLevel(t *testing.T) {
	// 最大リスクレベルの判定を検証
	tests := []struct {
		predictions []predictor.Prediction
		want        meta.RiskLevel
	}{
		{
			predictions: []predictor.Prediction{{RiskLevel: meta.RiskLow}},
			want:        meta.RiskLow,
		},
		{
			predictions: []predictor.Prediction{
				{RiskLevel: meta.RiskLow},
				{RiskLevel: meta.RiskCritical},
			},
			want: meta.RiskCritical,
		},
		{
			predictions: []predictor.Prediction{
				{RiskLevel: meta.RiskMedium},
				{RiskLevel: meta.RiskHigh},
			},
			want: meta.RiskHigh,
		},
	}
	for _, tt := range tests {
		got := WorstRiskLevel(tt.predictions)
		if got != tt.want {
			t.Errorf("WorstRiskLevel() = %s, want %s", got, tt.want)
		}
	}
}

func TestMultipleAnalyses(t *testing.T) {
	// 複数分析結果の出力を検証
	r := NewTextReporter()
	report := &Report{
		Analyses: []AnalysisResult{
			{Table: "mydb.users", SQL: "ALTER TABLE users ADD COLUMN a INT",
				Predictions: []predictor.Prediction{{Description: "ADD COLUMN", Algorithm: meta.AlgorithmInstant, Lock: meta.LockNone, RiskLevel: meta.RiskLow, TableInfo: predictor.TableInfo{Label: "N/A (no table metadata)"}}}},
			{Table: "mydb.orders", SQL: "ALTER TABLE orders ADD INDEX idx (col)",
				Predictions: []predictor.Prediction{{Description: "ADD INDEX", Algorithm: meta.AlgorithmInplace, Lock: meta.LockNone, RiskLevel: meta.RiskMedium, TableInfo: predictor.TableInfo{Label: "N/A (no table metadata)"}}}},
		},
	}
	output, err := r.Render(report)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output, "mydb.users") || !strings.Contains(output, "mydb.orders") {
		t.Error("出力に両方のテーブルが含まれること")
	}
	if !strings.Contains(output, "---") {
		t.Error("分析結果間にセパレータが含まれること")
	}
}
