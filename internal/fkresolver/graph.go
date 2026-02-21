package fkresolver

import "github.com/Glider2355/ddl-lock-analyzer/internal/meta"

// FKDirection は外部キー関係の方向を表す。
type FKDirection string

const (
	FKDirectionParent FKDirection = "PARENT"
	FKDirectionChild  FKDirection = "CHILD"
)

// FKLockImpact は関連テーブルへのロック影響を記述する。
type FKLockImpact struct {
	MetadataLock bool           `json:"metadata_lock"`
	LockLevel    meta.LockLevel `json:"lock_level"`
	Reason       string         `json:"reason"`
}

// FKRelation は依存関係グラフ内の外部キー関係を表す。
type FKRelation struct {
	Table      string              `json:"table"`
	Constraint meta.ForeignKeyMeta `json:"constraint"`
	Direction  FKDirection         `json:"direction"`
	Depth      int                 `json:"depth"`
	LockImpact FKLockImpact        `json:"lock_impact"`
}

// FKGraph はALTER対象テーブルの外部キー依存関係グラフを表す。
type FKGraph struct {
	Root     string       `json:"root"`
	Parents  []FKRelation `json:"parents,omitempty"`
	Children []FKRelation `json:"children,omitempty"`
	MaxDepth int          `json:"max_depth"`
	Warnings []string     `json:"warnings,omitempty"`
}

// TotalAffectedTables はFK伝播により影響を受けるテーブルの総数を返す。
func (g *FKGraph) TotalAffectedTables() int {
	return len(g.Parents) + len(g.Children)
}

// AllRelations は全FK関係（親+子）を返す。
func (g *FKGraph) AllRelations() []FKRelation {
	all := make([]FKRelation, 0, len(g.Parents)+len(g.Children))
	all = append(all, g.Parents...)
	all = append(all, g.Children...)
	return all
}
