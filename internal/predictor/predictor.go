package predictor

import (
	"strings"

	"github.com/Glider2355/ddl-lock-analyzer/internal/meta"
)

// Prediction represents the predicted lock behavior for a single ALTER action.
type Prediction struct {
	ActionType   meta.AlterActionType `json:"action_type"`
	Description  string               `json:"description"`
	Algorithm    meta.Algorithm       `json:"algorithm"`
	Lock         meta.LockLevel       `json:"lock_level"`
	TableRebuild bool                 `json:"table_rebuild"`
	RiskLevel    meta.RiskLevel       `json:"risk_level"`
	TableInfo    TableInfo            `json:"table_info"`
	Notes        []string             `json:"notes,omitempty"`
	Warnings     []string             `json:"warnings,omitempty"`
}

// Predictor predicts DDL lock behavior based on rules.
type Predictor struct {
	rules []PredictionRule
}

// New creates a new Predictor with default rules.
func New() *Predictor {
	return &Predictor{rules: defaultRules()}
}

// Predict predicts the lock behavior for a given ALTER action.
func (p *Predictor) Predict(action meta.AlterAction, tableMeta *meta.TableMeta) Prediction {
	// Non-InnoDB: everything is COPY/EXCLUSIVE
	if tableMeta != nil && !strings.EqualFold(tableMeta.Engine, "InnoDB") && tableMeta.Engine != "" {
		return Prediction{
			ActionType:   action.Type,
			Description:  string(action.Type) + " (non-InnoDB)",
			Algorithm:    meta.AlgorithmCopy,
			Lock:         meta.LockExclusive,
			TableRebuild: true,
			RiskLevel:    meta.RiskCritical,
			TableInfo:    CollectTableInfo(tableMeta),
			Warnings:     []string{"Non-InnoDB engine — all operations use COPY algorithm with EXCLUSIVE lock"},
		}
	}

	for _, rule := range p.rules {
		if rule.ActionType != action.Type {
			continue
		}
		if !rule.Condition(action, tableMeta) {
			continue
		}
		pred := Prediction{
			ActionType:   action.Type,
			Description:  rule.Description,
			Algorithm:    rule.Algorithm,
			Lock:         rule.Lock,
			TableRebuild: rule.TableRebuild,
			RiskLevel:    calculateRisk(rule.Algorithm, rule.Lock, rule.TableRebuild),
			TableInfo:    CollectTableInfo(tableMeta),
			Notes:        rule.Notes,
			Warnings:     rule.Warnings,
		}
		return pred
	}

	// Fallback: unknown operation defaults to COPY/EXCLUSIVE for safety
	return Prediction{
		ActionType:   action.Type,
		Description:  string(action.Type) + " (unknown)",
		Algorithm:    meta.AlgorithmCopy,
		Lock:         meta.LockExclusive,
		TableRebuild: true,
		RiskLevel:    meta.RiskCritical,
		TableInfo:    CollectTableInfo(tableMeta),
		Warnings:     []string{"Unknown operation — defaulting to COPY/EXCLUSIVE for safety"},
	}
}

// PredictAll predicts lock behavior for all actions in an ALTER operation.
func (p *Predictor) PredictAll(op meta.AlterOperation, tableMeta *meta.TableMeta) []Prediction {
	predictions := make([]Prediction, 0, len(op.Actions))
	for _, action := range op.Actions {
		predictions = append(predictions, p.Predict(action, tableMeta))
	}
	return predictions
}

func calculateRisk(algorithm meta.Algorithm, lock meta.LockLevel, rebuild bool) meta.RiskLevel {
	if algorithm == meta.AlgorithmCopy || lock == meta.LockExclusive {
		return meta.RiskCritical
	}
	if algorithm == meta.AlgorithmInstant {
		return meta.RiskLow
	}
	// INPLACE
	if rebuild {
		return meta.RiskHigh
	}
	return meta.RiskMedium
}
