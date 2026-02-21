package reporter

import (
	"github.com/Glider2355/ddl-lock-analyzer/internal/fkresolver"
	"github.com/Glider2355/ddl-lock-analyzer/internal/meta"
	"github.com/Glider2355/ddl-lock-analyzer/internal/predictor"
)

// AnalysisResult holds the complete analysis result for one ALTER statement.
type AnalysisResult struct {
	Table       string                 `json:"table"`
	SQL         string                 `json:"sql"`
	Predictions []predictor.Prediction `json:"predictions"`
	FKGraph     *fkresolver.FKGraph    `json:"fk_propagation,omitempty"`
	TableMeta   *meta.TableMeta        `json:"-"`
}

// Report holds all analysis results.
type Report struct {
	Analyses []AnalysisResult `json:"analyses"`
}

// Reporter formats and outputs analysis results.
type Reporter interface {
	Render(report *Report) (string, error)
}

// WorstRiskLevel returns the highest risk level from all predictions.
func WorstRiskLevel(predictions []predictor.Prediction) meta.RiskLevel {
	worst := meta.RiskLow
	for _, p := range predictions {
		if riskOrd(p.RiskLevel) > riskOrd(worst) {
			worst = p.RiskLevel
		}
	}
	return worst
}

func riskOrd(r meta.RiskLevel) int {
	switch r {
	case meta.RiskLow:
		return 0
	case meta.RiskMedium:
		return 1
	case meta.RiskHigh:
		return 2
	case meta.RiskCritical:
		return 3
	default:
		return 0
	}
}
