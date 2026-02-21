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

// resolveConfig は親/子方向の解決パラメータを定義する。
type resolveConfig struct {
	direction FKDirection
	tableKey  func(fk meta.ForeignKeyMeta) string
	appendTo  func(graph *FKGraph, rel FKRelation)
	nextFKs   func(tm *meta.TableMeta) []meta.ForeignKeyMeta
}

var parentConfig = resolveConfig{
	direction: FKDirectionParent,
	tableKey:  func(fk meta.ForeignKeyMeta) string { return qualifiedName(fk.ReferencedSchema, fk.ReferencedTable) },
	appendTo:  func(graph *FKGraph, rel FKRelation) { graph.Parents = append(graph.Parents, rel) },
	nextFKs:   func(tm *meta.TableMeta) []meta.ForeignKeyMeta { return tm.ForeignKeys },
}

var childConfig = resolveConfig{
	direction: FKDirectionChild,
	tableKey:  func(fk meta.ForeignKeyMeta) string { return qualifiedName(fk.SourceSchema, fk.SourceTable) },
	appendTo:  func(graph *FKGraph, rel FKRelation) { graph.Children = append(graph.Children, rel) },
	nextFKs:   func(tm *meta.TableMeta) []meta.ForeignKeyMeta { return tm.ReferencedBy },
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
		r.resolveDirection(graph, fk, actions, 1, visited, parentConfig)
	}

	// 子方向: このテーブルを参照するテーブル
	for _, fk := range tableMeta.ReferencedBy {
		r.resolveDirection(graph, fk, actions, 1, visited, childConfig)
	}

	return graph, nil
}

func (r *Resolver) resolveDirection(graph *FKGraph, fk meta.ForeignKeyMeta, actions []meta.AlterAction, depth int, visited map[string]bool, cfg resolveConfig) {
	if depth > r.maxDepth {
		return
	}

	key := cfg.tableKey(fk)
	if visited[key] {
		graph.Warnings = append(graph.Warnings,
			fmt.Sprintf("Circular FK reference detected: %s (skipping)", key))
		return
	}
	visited[key] = true

	impact := DetermineLockImpact(cfg.direction, actions, fk)
	cfg.appendTo(graph, FKRelation{
		Table:      key,
		Constraint: fk,
		Direction:  cfg.direction,
		Depth:      depth,
		LockImpact: impact,
	})

	if r.provider == nil {
		return
	}

	// 再帰: 関連テーブルの次のFK関係を検索
	parts := splitQualifiedName(key)
	nextMeta, err := r.provider.GetTableMeta(parts[0], parts[1])
	if err != nil {
		return
	}
	for _, nextFK := range cfg.nextFKs(nextMeta) {
		r.resolveDirection(graph, nextFK, actions, depth+1, visited, cfg)
	}
}

func qualifiedName(schema, table string) string {
	if schema == "" {
		return table
	}
	return schema + "." + table
}

// splitQualifiedName は "schema.table" を [schema, table] に分割する。
// schema がない場合は ["", table] を返す。
func splitQualifiedName(name string) [2]string {
	for i, c := range name {
		if c == '.' {
			return [2]string{name[:i], name[i+1:]}
		}
	}
	return [2]string{"", name}
}
