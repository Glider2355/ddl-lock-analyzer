package fkresolver

import (
	"fmt"
	"testing"

	"github.com/Glider2355/ddl-lock-analyzer/internal/meta"
)

// mockProvider はテスト用の MetaProvider 実装。
type mockProvider struct {
	tables map[string]*meta.TableMeta
}

func (m *mockProvider) GetTableMeta(schema, table string) (*meta.TableMeta, error) {
	key := schema + "." + table
	if tm, ok := m.tables[key]; ok {
		return tm, nil
	}
	if tm, ok := m.tables[table]; ok {
		return tm, nil
	}
	return nil, fmt.Errorf("テーブルが見つかりません: %s", key)
}

func TestResolveNoFK(t *testing.T) {
	// FK関係なしの場合、影響テーブル数が0であることを検証
	provider := &mockProvider{
		tables: map[string]*meta.TableMeta{
			"mydb.users": {Schema: "mydb", Table: "users", Engine: "InnoDB"},
		},
	}
	resolver := NewResolver(provider, 5, true)
	graph, err := resolver.Resolve("mydb", "users", nil)
	if err != nil {
		t.Fatal(err)
	}
	if graph.TotalAffectedTables() != 0 {
		t.Errorf("影響テーブル数が0であること: got %d", graph.TotalAffectedTables())
	}
}

func TestResolveParentFK(t *testing.T) {
	// 親方向のFK解決を検証
	provider := &mockProvider{
		tables: map[string]*meta.TableMeta{
			"mydb.orders": {
				Schema: "mydb", Table: "orders", Engine: "InnoDB",
				ForeignKeys: []meta.ForeignKeyMeta{
					{
						ConstraintName:    "fk_orders_user_id",
						SourceSchema:      "mydb",
						SourceTable:       "orders",
						SourceColumns:     []string{"user_id"},
						ReferencedSchema:  "mydb",
						ReferencedTable:   "users",
						ReferencedColumns: []string{"id"},
					},
				},
			},
			"mydb.users": {Schema: "mydb", Table: "users", Engine: "InnoDB"},
		},
	}
	resolver := NewResolver(provider, 5, true)
	actions := []meta.AlterAction{{Type: meta.ActionAddColumn}}
	graph, err := resolver.Resolve("mydb", "orders", actions)
	if err != nil {
		t.Fatal(err)
	}
	if len(graph.Parents) != 1 {
		t.Fatalf("親テーブル数が1であること: got %d", len(graph.Parents))
	}
	if graph.Parents[0].Table != "mydb.users" {
		t.Errorf("親テーブルがmydb.usersであること: got %s", graph.Parents[0].Table)
	}
	if graph.Parents[0].Direction != FKDirectionParent {
		t.Errorf("方向がPARENTであること: got %s", graph.Parents[0].Direction)
	}
}

func TestResolveChildFK(t *testing.T) {
	// 子方向のFK解決を検証
	provider := &mockProvider{
		tables: map[string]*meta.TableMeta{
			"mydb.users": {
				Schema: "mydb", Table: "users", Engine: "InnoDB",
				ReferencedBy: []meta.ForeignKeyMeta{
					{
						ConstraintName:    "fk_orders_user_id",
						SourceSchema:      "mydb",
						SourceTable:       "orders",
						SourceColumns:     []string{"user_id"},
						ReferencedSchema:  "mydb",
						ReferencedTable:   "users",
						ReferencedColumns: []string{"id"},
					},
				},
			},
			"mydb.orders": {Schema: "mydb", Table: "orders", Engine: "InnoDB"},
		},
	}
	resolver := NewResolver(provider, 5, true)
	actions := []meta.AlterAction{{Type: meta.ActionAddColumn}}
	graph, err := resolver.Resolve("mydb", "users", actions)
	if err != nil {
		t.Fatal(err)
	}
	if len(graph.Children) != 1 {
		t.Fatalf("子テーブル数が1であること: got %d", len(graph.Children))
	}
	if graph.Children[0].Table != "mydb.orders" {
		t.Errorf("子テーブルがmydb.ordersであること: got %s", graph.Children[0].Table)
	}
}

func TestResolveCircularReference(t *testing.T) {
	// 循環参照の検出を検証
	provider := &mockProvider{
		tables: map[string]*meta.TableMeta{
			"mydb.a": {
				Schema: "mydb", Table: "a", Engine: "InnoDB",
				ForeignKeys: []meta.ForeignKeyMeta{
					{
						ConstraintName:    "fk_a_b",
						SourceSchema:      "mydb",
						SourceTable:       "a",
						SourceColumns:     []string{"b_id"},
						ReferencedSchema:  "mydb",
						ReferencedTable:   "b",
						ReferencedColumns: []string{"id"},
					},
				},
			},
			"mydb.b": {
				Schema: "mydb", Table: "b", Engine: "InnoDB",
				ForeignKeys: []meta.ForeignKeyMeta{
					{
						ConstraintName:    "fk_b_a",
						SourceSchema:      "mydb",
						SourceTable:       "b",
						SourceColumns:     []string{"a_id"},
						ReferencedSchema:  "mydb",
						ReferencedTable:   "a",
						ReferencedColumns: []string{"id"},
					},
				},
			},
		},
	}
	resolver := NewResolver(provider, 5, true)
	graph, err := resolver.Resolve("mydb", "a", nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(graph.Warnings) == 0 {
		t.Error("循環参照の警告があること")
	}
}

func TestResolveFKChecksOff(t *testing.T) {
	// foreign_key_checks=OFFの場合、FK解決がスキップされることを検証
	provider := &mockProvider{
		tables: map[string]*meta.TableMeta{
			"mydb.orders": {
				Schema: "mydb", Table: "orders", Engine: "InnoDB",
				ForeignKeys: []meta.ForeignKeyMeta{
					{
						ConstraintName:    "fk_orders_user_id",
						SourceSchema:      "mydb",
						SourceTable:       "orders",
						SourceColumns:     []string{"user_id"},
						ReferencedSchema:  "mydb",
						ReferencedTable:   "users",
						ReferencedColumns: []string{"id"},
					},
				},
			},
		},
	}
	resolver := NewResolver(provider, 5, false) // fk_checks=OFF
	graph, err := resolver.Resolve("mydb", "orders", nil)
	if err != nil {
		t.Fatal(err)
	}
	if graph.TotalAffectedTables() != 0 {
		t.Errorf("fk_checks=OFFで影響テーブル数が0であること: got %d", graph.TotalAffectedTables())
	}
}

func TestResolveDeepFK(t *testing.T) {
	// 深いFK依存関係の解決を検証
	provider := &mockProvider{
		tables: map[string]*meta.TableMeta{
			"mydb.orders": {
				Schema: "mydb", Table: "orders", Engine: "InnoDB",
				ReferencedBy: []meta.ForeignKeyMeta{
					{
						ConstraintName:    "fk_items_order",
						SourceSchema:      "mydb",
						SourceTable:       "order_items",
						SourceColumns:     []string{"order_id"},
						ReferencedSchema:  "mydb",
						ReferencedTable:   "orders",
						ReferencedColumns: []string{"id"},
					},
				},
			},
			"mydb.order_items": {
				Schema: "mydb", Table: "order_items", Engine: "InnoDB",
				ReferencedBy: []meta.ForeignKeyMeta{
					{
						ConstraintName:    "fk_discounts_item",
						SourceSchema:      "mydb",
						SourceTable:       "item_discounts",
						SourceColumns:     []string{"item_id"},
						ReferencedSchema:  "mydb",
						ReferencedTable:   "order_items",
						ReferencedColumns: []string{"id"},
					},
				},
			},
			"mydb.item_discounts": {
				Schema: "mydb", Table: "item_discounts", Engine: "InnoDB",
			},
		},
	}
	resolver := NewResolver(provider, 5, true)
	graph, err := resolver.Resolve("mydb", "orders", nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(graph.Children) != 2 {
		t.Fatalf("子テーブル数が2（深さ1+2）であること: got %d", len(graph.Children))
	}
	if graph.Children[0].Depth != 1 {
		t.Errorf("深さが1であること: got %d", graph.Children[0].Depth)
	}
	if graph.Children[1].Depth != 2 {
		t.Errorf("深さが2であること: got %d", graph.Children[1].Depth)
	}
}

func TestDropFKColumnImpact(t *testing.T) {
	// FKカラムのDROP COLUMNがEXCLUSIVEロックになることを検証
	fk := meta.ForeignKeyMeta{
		SourceColumns:     []string{"user_id"},
		ReferencedColumns: []string{"id"},
		SourceTable:       "orders",
		ReferencedTable:   "users",
	}
	actions := []meta.AlterAction{{
		Type:   meta.ActionDropColumn,
		Detail: meta.ActionDetail{ColumnName: "user_id"},
	}}
	impact := DetermineLockImpact(FKDirectionParent, actions, fk)
	if impact.LockLevel != meta.LockExclusive {
		t.Errorf("FKカラム削除でEXCLUSIVEであること: got %s", impact.LockLevel)
	}
}
