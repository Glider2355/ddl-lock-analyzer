package fkresolver

import (
	"fmt"

	"github.com/muramatsuryo/ddl-lock-analyzer/internal/meta"
)

// MetaProvider is an interface for looking up table metadata.
type MetaProvider interface {
	GetTableMeta(schema, table string) (*meta.TableMeta, error)
}

// Resolver resolves FK dependencies and builds the FK graph.
type Resolver struct {
	provider MetaProvider
	maxDepth int
	fkChecks bool
}

// NewResolver creates a new FK resolver.
func NewResolver(provider MetaProvider, maxDepth int, fkChecks bool) *Resolver {
	return &Resolver{
		provider: provider,
		maxDepth: maxDepth,
		fkChecks: fkChecks,
	}
}

// Resolve builds the FK dependency graph for the given table and actions.
func (r *Resolver) Resolve(schema, table string, actions []meta.AlterAction) (*FKGraph, error) {
	graph := &FKGraph{
		Root:     qualifiedName(schema, table),
		MaxDepth: r.maxDepth,
	}

	if !r.fkChecks {
		return graph, nil
	}

	tableMeta, err := r.provider.GetTableMeta(schema, table)
	if err != nil {
		return graph, nil // No meta available, skip FK resolution
	}

	visited := map[string]bool{qualifiedName(schema, table): true}

	// Parent direction: tables referenced by this table's FKs
	for _, fk := range tableMeta.ForeignKeys {
		r.resolveParent(graph, fk, actions, 1, visited)
	}

	// Child direction: tables that reference this table
	for _, fk := range tableMeta.ReferencedBy {
		r.resolveChild(graph, fk, actions, 1, visited)
	}

	return graph, nil
}

func (r *Resolver) resolveParent(graph *FKGraph, fk meta.ForeignKeyMeta, actions []meta.AlterAction, depth int, visited map[string]bool) {
	if depth > r.maxDepth {
		return
	}

	key := qualifiedName(fk.ReferencedSchema, fk.ReferencedTable)
	if visited[key] {
		graph.Warnings = append(graph.Warnings,
			fmt.Sprintf("Circular FK reference detected: %s (skipping)", key))
		return
	}
	visited[key] = true

	impact := DetermineLockImpact(FKDirectionParent, actions, fk)
	graph.Parents = append(graph.Parents, FKRelation{
		Table:      key,
		Constraint: fk,
		Direction:  FKDirectionParent,
		Depth:      depth,
		LockImpact: impact,
	})

	if r.provider == nil {
		return
	}

	// Recurse: find parent's own FK parents
	parentMeta, err := r.provider.GetTableMeta(fk.ReferencedSchema, fk.ReferencedTable)
	if err != nil {
		return
	}
	for _, parentFK := range parentMeta.ForeignKeys {
		r.resolveParent(graph, parentFK, actions, depth+1, visited)
	}
}

func (r *Resolver) resolveChild(graph *FKGraph, fk meta.ForeignKeyMeta, actions []meta.AlterAction, depth int, visited map[string]bool) {
	if depth > r.maxDepth {
		return
	}

	key := qualifiedName(fk.SourceSchema, fk.SourceTable)
	if visited[key] {
		graph.Warnings = append(graph.Warnings,
			fmt.Sprintf("Circular FK reference detected: %s (skipping)", key))
		return
	}
	visited[key] = true

	impact := DetermineLockImpact(FKDirectionChild, actions, fk)
	graph.Children = append(graph.Children, FKRelation{
		Table:      key,
		Constraint: fk,
		Direction:  FKDirectionChild,
		Depth:      depth,
		LockImpact: impact,
	})

	if r.provider == nil {
		return
	}

	// Recurse: find child's own FK children
	childMeta, err := r.provider.GetTableMeta(fk.SourceSchema, fk.SourceTable)
	if err != nil {
		return
	}
	for _, childFK := range childMeta.ReferencedBy {
		r.resolveChild(graph, childFK, actions, depth+1, visited)
	}
}

func qualifiedName(schema, table string) string {
	if schema == "" {
		return table
	}
	return schema + "." + table
}
