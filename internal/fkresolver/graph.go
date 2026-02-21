package fkresolver

import "github.com/muramatsuryo/ddl-lock-analyzer/internal/meta"

// FKDirection represents the direction of a foreign key relationship.
type FKDirection string

const (
	FKDirectionParent FKDirection = "PARENT"
	FKDirectionChild  FKDirection = "CHILD"
)

// FKLockImpact describes the lock impact on a related table.
type FKLockImpact struct {
	MetadataLock bool           `json:"metadata_lock"`
	LockLevel    meta.LockLevel `json:"lock_level"`
	Reason       string         `json:"reason"`
}

// FKRelation represents a foreign key relationship in the dependency graph.
type FKRelation struct {
	Table      string              `json:"table"`
	Constraint meta.ForeignKeyMeta `json:"constraint"`
	Direction  FKDirection         `json:"direction"`
	Depth      int                 `json:"depth"`
	LockImpact FKLockImpact        `json:"lock_impact"`
}

// FKGraph represents the foreign key dependency graph for an ALTER target table.
type FKGraph struct {
	Root     string       `json:"root"`
	Parents  []FKRelation `json:"parents,omitempty"`
	Children []FKRelation `json:"children,omitempty"`
	MaxDepth int          `json:"max_depth"`
	Warnings []string     `json:"warnings,omitempty"`
}

// TotalAffectedTables returns the total number of tables affected by FK propagation.
func (g *FKGraph) TotalAffectedTables() int {
	return len(g.Parents) + len(g.Children)
}

// AllRelations returns all FK relations (parents + children).
func (g *FKGraph) AllRelations() []FKRelation {
	all := make([]FKRelation, 0, len(g.Parents)+len(g.Children))
	all = append(all, g.Parents...)
	all = append(all, g.Children...)
	return all
}
