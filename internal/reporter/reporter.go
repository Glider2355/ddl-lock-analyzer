package reporter

import (
	"github.com/Glider2355/ddl-lock-analyzer/internal/fkresolver"
	"github.com/Glider2355/ddl-lock-analyzer/internal/meta"
	"github.com/Glider2355/ddl-lock-analyzer/internal/predictor"
)

// AnalysisResult は1つのALTER文に対する完全な分析結果を保持する。
type AnalysisResult struct {
	Table       string                 `json:"table"`
	SQL         string                 `json:"sql"`
	Predictions []predictor.Prediction `json:"predictions"`
	FKGraph     *fkresolver.FKGraph    `json:"fk_propagation,omitempty"`
	TableMeta   *meta.TableMeta        `json:"-"`
}

// Report は全分析結果を保持する。
type Report struct {
	Analyses []AnalysisResult `json:"analyses"`
}

// Reporter は分析結果をフォーマットして出力する。
type Reporter interface {
	Render(report *Report) (string, error)
}

// WorstRiskLevel は全予測結果から最も高いリスクレベルを返す。
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

// FKLockTypeString はFKロックレベルを表示用文字列に変換する。
func FKLockTypeString(level meta.LockLevel) string {
	if level == meta.LockExclusive {
		return "EXCLUSIVE"
	}
	return "SHARED_READ"
}
