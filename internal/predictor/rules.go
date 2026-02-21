package predictor

import (
	"regexp"
	"strconv"

	"github.com/Glider2355/ddl-lock-analyzer/internal/meta"
)

// PredictionRule はDDLロック動作を予測するためのルールを定義する。
type PredictionRule struct {
	ActionType   meta.AlterActionType
	Description  string
	Condition    func(action meta.AlterAction, tableMeta *meta.TableMeta) bool
	Algorithm    meta.Algorithm
	Lock         meta.LockLevel
	TableRebuild bool
	Notes        []string
	Warnings     []string
}

// defaultRules はカテゴリ別ファイルのルールを正しい順序で結合して返す。
func defaultRules() []PredictionRule {
	var rules []PredictionRule
	rules = append(rules, columnRules()...)
	rules = append(rules, modifyRules()...)
	rules = append(rules, indexRules()...)
	rules = append(rules, tableRules()...)
	rules = append(rules, partitionRules()...)
	return rules
}

func alwaysMatch(_ meta.AlterAction, _ *meta.TableMeta) bool {
	return true
}

// varcharLenRegex extracts the length from VARCHAR(N) type strings.
var varcharLenRegex = regexp.MustCompile(`(?i)varchar\((\d+)\)`)

// extractVarcharLength returns the numeric length from a VARCHAR(N) type string.
// Returns -1 if the type is not VARCHAR or cannot be parsed.
func extractVarcharLength(colType string) int {
	matches := varcharLenRegex.FindStringSubmatch(colType)
	if len(matches) < 2 {
		return -1
	}
	n, err := strconv.Atoi(matches[1])
	if err != nil {
		return -1
	}
	return n
}
