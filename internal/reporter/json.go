package reporter

import (
	"encoding/json"

	"github.com/Glider2355/ddl-lock-analyzer/internal/fkresolver"
	"github.com/Glider2355/ddl-lock-analyzer/internal/meta"
)

// JSONReporter はJSON形式で結果を出力する。
type JSONReporter struct{}

// NewJSONReporter は新しい JSONReporter を作成する。
func NewJSONReporter() *JSONReporter {
	return &JSONReporter{}
}

type jsonOutput struct {
	Analyses []jsonAnalysis `json:"analyses"`
}

type jsonAnalysis struct {
	Table         string             `json:"table"`
	SQL           string             `json:"sql"`
	Operation     string             `json:"operation"`
	Algorithm     meta.Algorithm     `json:"algorithm"`
	LockLevel     meta.LockLevel     `json:"lock_level"`
	TableRebuild  bool               `json:"table_rebuild"`
	TableInfo     *jsonTableInfo     `json:"table_info,omitempty"`
	RiskLevel     meta.RiskLevel     `json:"risk_level"`
	FKPropagation *jsonFKPropagation `json:"fk_propagation,omitempty"`
	Notes         []string           `json:"notes,omitempty"`
	Warnings      []string           `json:"warnings,omitempty"`
}

type jsonTableInfo struct {
	RowCount   int64 `json:"row_count"`
	DataSize   int64 `json:"data_size_bytes"`
	IndexSize  int64 `json:"index_size_bytes"`
	IndexCount int   `json:"index_count"`
}

type jsonFKPropagation struct {
	TotalAffectedTables int              `json:"total_affected_tables"`
	Relations           []jsonFKRelation `json:"relations"`
}

type jsonFKRelation struct {
	Direction         fkresolver.FKDirection `json:"direction"`
	Table             string                 `json:"table"`
	Constraint        string                 `json:"constraint"`
	Columns           []string               `json:"columns"`
	ReferencedColumns []string               `json:"referenced_columns"`
	LockType          string                 `json:"lock_type"`
	Depth             int                    `json:"depth"`
}

// Render はレポートをJSONとしてレンダリングする。
func (r *JSONReporter) Render(report *Report) (string, error) {
	output := jsonOutput{}

	for _, analysis := range report.Analyses {
		for _, pred := range analysis.Predictions {
			ja := jsonAnalysis{
				Table:        analysis.Table,
				SQL:          analysis.SQL,
				Operation:    string(pred.ActionType),
				Algorithm:    pred.Algorithm,
				LockLevel:    pred.Lock,
				TableRebuild: pred.TableRebuild,
				RiskLevel:    pred.RiskLevel,
				Notes:        pred.Notes,
				Warnings:     pred.Warnings,
			}

			if pred.TableInfo.Label != "" && pred.TableInfo.Label != "N/A (no table metadata)" {
				ja.TableInfo = &jsonTableInfo{
					RowCount:   pred.TableInfo.RowCount,
					DataSize:   pred.TableInfo.DataSize,
					IndexSize:  pred.TableInfo.IndexSize,
					IndexCount: pred.TableInfo.IndexCount,
				}
			}

			if analysis.FKGraph != nil && analysis.FKGraph.TotalAffectedTables() > 0 {
				fkp := &jsonFKPropagation{
					TotalAffectedTables: analysis.FKGraph.TotalAffectedTables(),
				}
				for _, rel := range analysis.FKGraph.AllRelations() {
					fkp.Relations = append(fkp.Relations, jsonFKRelation{
						Direction:         rel.Direction,
						Table:             rel.Table,
						Constraint:        rel.Constraint.ConstraintName,
						Columns:           rel.Constraint.SourceColumns,
						ReferencedColumns: rel.Constraint.ReferencedColumns,
						LockType:          FKLockTypeString(rel.LockImpact.LockLevel),
						Depth:             rel.Depth,
					})
				}
				ja.FKPropagation = fkp
			}

			output.Analyses = append(output.Analyses, ja)
		}
	}

	data, err := json.MarshalIndent(output, "", "  ")
	if err != nil {
		return "", err
	}
	return string(data), nil
}
