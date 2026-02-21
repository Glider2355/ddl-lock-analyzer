package reporter

import (
	"fmt"
	"strings"

	"github.com/Glider2355/ddl-lock-analyzer/internal/meta"
)

// TextReporter は人間が読みやすいテキスト形式で結果を出力する。
type TextReporter struct{}

// NewTextReporter は新しい TextReporter を作成する。
func NewTextReporter() *TextReporter {
	return &TextReporter{}
}

// Render はレポートをテキストとしてレンダリングする。
func (r *TextReporter) Render(report *Report) (string, error) {
	var sb strings.Builder

	sb.WriteString("=== DDL Lock Analysis Report ===\n")

	for i, analysis := range report.Analyses {
		if i > 0 {
			sb.WriteString("\n---\n")
		}
		r.renderAnalysis(&sb, &analysis)
	}

	return sb.String(), nil
}

func (r *TextReporter) renderAnalysis(sb *strings.Builder, analysis *AnalysisResult) {
	fmt.Fprintf(sb, "\nTable: %s\n", analysis.Table)
	fmt.Fprintf(sb, "SQL:   %s\n", analysis.SQL)

	for _, pred := range analysis.Predictions {
		fmt.Fprintf(sb, "\n  Operation     : %s\n", pred.Description)
		fmt.Fprintf(sb, "  Algorithm     : %s\n", pred.Algorithm)
		fmt.Fprintf(sb, "  Lock Level    : %s%s\n", pred.Lock, lockDescription(pred.Lock))
		fmt.Fprintf(sb, "  Table Rebuild : %s\n", boolYesNo(pred.TableRebuild))
		fmt.Fprintf(sb, "  Table Info    : %s\n", pred.TableInfo.Label)
		fmt.Fprintf(sb, "  Risk Level    : %s\n", pred.RiskLevel)

		if len(pred.Notes) > 0 {
			sb.WriteString("\n  Note:\n")
			for _, note := range pred.Notes {
				fmt.Fprintf(sb, "    - %s\n", note)
			}
		}

		if len(pred.Warnings) > 0 {
			sb.WriteString("\n  Warning:\n")
			for _, w := range pred.Warnings {
				fmt.Fprintf(sb, "    - %s\n", w)
			}
		}
	}

	r.renderFKPropagation(sb, analysis)
}

func (r *TextReporter) renderFKPropagation(sb *strings.Builder, analysis *AnalysisResult) {
	graph := analysis.FKGraph
	if graph == nil || graph.TotalAffectedTables() == 0 {
		return
	}

	sb.WriteString("\n  FK Lock Propagation:\n")
	fmt.Fprintf(sb, "    %s has %d FK relationships — MDL will propagate to related tables\n\n",
		analysis.Table, graph.TotalAffectedTables())

	fmt.Fprintf(sb, "    %-10s %-22s %-15s %s\n",
		"Direction", "Table", "Lock Type", "Reason")
	fmt.Fprintf(sb, "    %s %s %s %s\n",
		strings.Repeat("─", 10), strings.Repeat("─", 22),
		strings.Repeat("─", 15), strings.Repeat("─", 30))

	for _, rel := range graph.Parents {
		prefix := depthPrefix(rel.Depth, "PARENT")
		lockType := "SHARED_READ"
		if rel.LockImpact.LockLevel == meta.LockExclusive {
			lockType = "EXCLUSIVE"
		}
		fmt.Fprintf(sb, "    %-10s %-22s %-15s %s\n",
			prefix, rel.Table, lockType, rel.LockImpact.Reason)
	}
	for _, rel := range graph.Children {
		prefix := depthPrefix(rel.Depth, "CHILD")
		lockType := "SHARED_READ"
		if rel.LockImpact.LockLevel == meta.LockExclusive {
			lockType = "EXCLUSIVE"
		}
		fmt.Fprintf(sb, "    %-10s %-22s %-15s %s\n",
			prefix, rel.Table, lockType, rel.LockImpact.Reason)
	}

	if len(graph.Warnings) > 0 {
		sb.WriteString("\n  FK Warning:\n")
		for _, w := range graph.Warnings {
			fmt.Fprintf(sb, "    - %s\n", w)
		}
	}

	sb.WriteString("\n  Warning:\n")
	fmt.Fprintf(sb, "    - MDL propagation to %d related tables detected\n", graph.TotalAffectedTables())
	sb.WriteString("    - Long-running DDL on related tables may cause MDL wait queue buildup\n")
	sb.WriteString("    - If concurrent DDL on related tables is planned, coordinate execution order\n")
}

func depthPrefix(depth int, direction string) string {
	if depth <= 1 {
		return direction
	}
	return strings.Repeat("  └─", depth-1) + direction
}

func lockDescription(lock meta.LockLevel) string {
	switch lock {
	case meta.LockNone:
		return " (concurrent DML allowed)"
	case meta.LockShared:
		return " (DML writes blocked)"
	case meta.LockExclusive:
		return " (DML blocked)"
	default:
		return ""
	}
}

func boolYesNo(b bool) string {
	if b {
		return "Yes"
	}
	return "No"
}
