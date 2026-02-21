package reporter

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/muramatsuryo/ddl-lock-analyzer/internal/meta"
	"github.com/muramatsuryo/ddl-lock-analyzer/internal/predictor"
)

func TestTextReporterBasic(t *testing.T) {
	r := NewTextReporter()
	report := &Report{
		Analyses: []AnalysisResult{
			{
				Table: "mydb.users",
				SQL:   "ALTER TABLE users ADD COLUMN nickname VARCHAR(255)",
				Predictions: []predictor.Prediction{
					{
						ActionType:   meta.ActionAddColumn,
						Description:  "ADD COLUMN (末尾, NULLABLE)",
						Algorithm:    meta.AlgorithmInstant,
						Lock:         meta.LockNone,
						TableRebuild: false,
						RiskLevel:    meta.RiskLow,
						Duration:     predictor.DurationEstimate{Label: "~0s (metadata only)"},
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
		"metadata only",
	}
	for _, check := range checks {
		if !strings.Contains(output, check) {
			t.Errorf("expected output to contain %q", check)
		}
	}
}

func TestTextReporterCritical(t *testing.T) {
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
						Duration:     predictor.DurationEstimate{Label: "~45s - ~180s"},
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
		t.Error("expected CRITICAL in output")
	}
	if !strings.Contains(output, "EXCLUSIVE") {
		t.Error("expected EXCLUSIVE in output")
	}
	if !strings.Contains(output, "Warning") {
		t.Error("expected Warning section in output")
	}
}

func TestJSONReporterBasic(t *testing.T) {
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
						Duration:     predictor.DurationEstimate{Label: "~0s"},
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
		t.Fatalf("invalid JSON output: %v", err)
	}

	if len(result.Analyses) != 1 {
		t.Fatalf("expected 1 analysis, got %d", len(result.Analyses))
	}
	a := result.Analyses[0]
	if a.Algorithm != meta.AlgorithmInstant {
		t.Errorf("expected INSTANT, got %s", a.Algorithm)
	}
	if a.RiskLevel != meta.RiskLow {
		t.Errorf("expected LOW, got %s", a.RiskLevel)
	}
}

func TestWorstRiskLevel(t *testing.T) {
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
	r := NewTextReporter()
	report := &Report{
		Analyses: []AnalysisResult{
			{Table: "mydb.users", SQL: "ALTER TABLE users ADD COLUMN a INT",
				Predictions: []predictor.Prediction{{Description: "ADD COLUMN", Algorithm: meta.AlgorithmInstant, Lock: meta.LockNone, RiskLevel: meta.RiskLow, Duration: predictor.DurationEstimate{Label: "~0s"}}}},
			{Table: "mydb.orders", SQL: "ALTER TABLE orders ADD INDEX idx (col)",
				Predictions: []predictor.Prediction{{Description: "ADD INDEX", Algorithm: meta.AlgorithmInplace, Lock: meta.LockNone, RiskLevel: meta.RiskMedium, Duration: predictor.DurationEstimate{Label: "~5s"}}}},
		},
	}
	output, err := r.Render(report)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output, "mydb.users") || !strings.Contains(output, "mydb.orders") {
		t.Error("expected both tables in output")
	}
	if !strings.Contains(output, "---") {
		t.Error("expected separator between analyses")
	}
}
