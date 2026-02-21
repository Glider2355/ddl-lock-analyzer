package fkresolver

import (
	"fmt"

	"github.com/Glider2355/ddl-lock-analyzer/internal/meta"
)

// MetaProvider はテーブルメタデータを検索するためのインターフェース。
type MetaProvider interface {
	GetTableMeta(schema, table string) (*meta.TableMeta, error)
}

// Resolver はFK依存関係を解決し、FKグラフを構築する。
type Resolver struct {
	provider MetaProvider
	maxDepth int
	fkChecks bool
}

// NewResolver は新しいFKリゾルバーを作成する。
func NewResolver(provider MetaProvider, maxDepth int, fkChecks bool) *Resolver {
	return &Resolver{
		provider: provider,
		maxDepth: maxDepth,
		fkChecks: fkChecks,
	}
}

// Resolve は指定されたテーブルとアクションに対するFK依存関係グラフを構築する。
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
		return graph, nil // メタデータ取得不可、FK解決をスキップ
	}

	visited := map[string]bool{qualifiedName(schema, table): true}

	// 親方向: このテーブルのFKが参照するテーブル
	for _, fk := range tableMeta.ForeignKeys {
		r.resolveParent(graph, fk, actions, 1, visited)
	}

	// 子方向: このテーブルを参照するテーブル
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

	// 再帰: 親テーブル自身のFK親を検索
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

	// 再帰: 子テーブル自身のFK子を検索
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
