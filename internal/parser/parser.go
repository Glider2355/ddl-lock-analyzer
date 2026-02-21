package parser

import (
	"fmt"
	"strings"

	"github.com/pingcap/tidb/pkg/parser"
	"github.com/pingcap/tidb/pkg/parser/ast"
	"github.com/pingcap/tidb/pkg/parser/format"
	_ "github.com/pingcap/tidb/pkg/parser/test_driver"

	"github.com/muramatsuryo/ddl-lock-analyzer/internal/meta"
)

// Parse parses one or more SQL statements and returns ALTER operations.
func Parse(sql string) ([]meta.AlterOperation, error) {
	p := parser.New()
	stmts, _, err := p.Parse(sql, "", "")
	if err != nil {
		return nil, fmt.Errorf("SQL parse error: %w", err)
	}

	var ops []meta.AlterOperation
	for _, stmt := range stmts {
		alterStmt, ok := stmt.(*ast.AlterTableStmt)
		if !ok {
			continue
		}
		op, err := buildAlterOperation(alterStmt, sql)
		if err != nil {
			return nil, err
		}
		ops = append(ops, op)
	}

	if len(ops) == 0 {
		return nil, fmt.Errorf("no ALTER TABLE statements found")
	}
	return ops, nil
}

func buildAlterOperation(stmt *ast.AlterTableStmt, rawSQL string) (meta.AlterOperation, error) {
	op := meta.AlterOperation{
		Table:  stmt.Table.Name.L,
		Schema: stmt.Table.Schema.L,
		RawSQL: extractSQL(stmt, rawSQL),
	}

	for _, spec := range stmt.Specs {
		actions := specToActions(spec)
		op.Actions = append(op.Actions, actions...)
	}

	if len(op.Actions) == 0 {
		return op, fmt.Errorf("no supported ALTER actions found in statement")
	}
	return op, nil
}

func extractSQL(stmt *ast.AlterTableStmt, rawSQL string) string {
	var sb strings.Builder
	ctx := format.NewRestoreCtx(format.DefaultRestoreFlags, &sb)
	if err := stmt.Restore(ctx); err == nil && sb.Len() > 0 {
		return sb.String()
	}
	return rawSQL
}

func specToActions(spec *ast.AlterTableSpec) []meta.AlterAction {
	switch spec.Tp {
	case ast.AlterTableAddColumns:
		return handleAddColumns(spec)
	case ast.AlterTableDropColumn:
		return handleDropColumn(spec)
	case ast.AlterTableModifyColumn:
		return handleModifyColumn(spec)
	case ast.AlterTableChangeColumn:
		return handleChangeColumn(spec)
	case ast.AlterTableRenameColumn:
		return handleRenameColumn(spec)
	case ast.AlterTableAlterColumn:
		return handleAlterColumn(spec)
	case ast.AlterTableAddConstraint:
		return handleAddConstraint(spec)
	case ast.AlterTableDropIndex:
		return handleDropIndex(spec)
	case ast.AlterTableDropPrimaryKey:
		return []meta.AlterAction{{Type: meta.ActionDropPrimaryKey}}
	case ast.AlterTableDropForeignKey:
		return handleDropForeignKey(spec)
	case ast.AlterTableRenameIndex:
		return handleRenameIndex(spec)
	case ast.AlterTableRenameTable:
		return handleRenameTable(spec)
	case ast.AlterTableOption:
		return handleTableOptions(spec)
	case ast.AlterTableAddPartitions:
		return []meta.AlterAction{{Type: meta.ActionAddPartition}}
	case ast.AlterTableDropPartition:
		return []meta.AlterAction{{Type: meta.ActionDropPartition}}
	default:
		return nil
	}
}

func handleAddColumns(spec *ast.AlterTableSpec) []meta.AlterAction {
	actions := make([]meta.AlterAction, 0, len(spec.NewColumns))
	for _, col := range spec.NewColumns {
		detail := meta.ActionDetail{
			ColumnName: col.Name.Name.L,
			ColumnType: columnTypeString(col),
		}
		nullable := isNullable(col)
		detail.IsNullable = &nullable
		detail.Position = positionString(spec.Position)
		detail.DefaultValue = defaultValueString(col)

		actions = append(actions, meta.AlterAction{
			Type:   meta.ActionAddColumn,
			Detail: detail,
		})
	}
	return actions
}

func handleDropColumn(spec *ast.AlterTableSpec) []meta.AlterAction {
	return []meta.AlterAction{{
		Type: meta.ActionDropColumn,
		Detail: meta.ActionDetail{
			ColumnName: spec.OldColumnName.Name.L,
		},
	}}
}

func handleModifyColumn(spec *ast.AlterTableSpec) []meta.AlterAction {
	if len(spec.NewColumns) == 0 {
		return nil
	}
	col := spec.NewColumns[0]
	detail := meta.ActionDetail{
		ColumnName: col.Name.Name.L,
		ColumnType: columnTypeString(col),
	}
	nullable := isNullable(col)
	detail.IsNullable = &nullable
	detail.Position = positionString(spec.Position)
	return []meta.AlterAction{{
		Type:   meta.ActionModifyColumn,
		Detail: detail,
	}}
}

func handleChangeColumn(spec *ast.AlterTableSpec) []meta.AlterAction {
	if len(spec.NewColumns) == 0 {
		return nil
	}
	col := spec.NewColumns[0]
	detail := meta.ActionDetail{
		ColumnName:    col.Name.Name.L,
		OldColumnName: spec.OldColumnName.Name.L,
		ColumnType:    columnTypeString(col),
	}
	nullable := isNullable(col)
	detail.IsNullable = &nullable
	detail.Position = positionString(spec.Position)

	return []meta.AlterAction{{Type: meta.ActionChangeColumn, Detail: detail}}
}

func handleRenameColumn(spec *ast.AlterTableSpec) []meta.AlterAction {
	if spec.OldColumnName == nil || spec.NewColumnName == nil {
		return nil
	}
	return []meta.AlterAction{{
		Type: meta.ActionRenameColumn,
		Detail: meta.ActionDetail{
			ColumnName:    spec.NewColumnName.Name.L,
			OldColumnName: spec.OldColumnName.Name.L,
		},
	}}
}

func handleAlterColumn(spec *ast.AlterTableSpec) []meta.AlterAction {
	if len(spec.NewColumns) == 0 {
		return nil
	}
	col := spec.NewColumns[0]
	colName := col.Name.Name.L

	// SET DEFAULT: parser stores the default expression in Options[0]
	if len(col.Options) > 0 {
		return []meta.AlterAction{{
			Type: meta.ActionSetDefault,
			Detail: meta.ActionDetail{
				ColumnName: colName,
			},
		}}
	}
	// DROP DEFAULT
	return []meta.AlterAction{{
		Type: meta.ActionDropDefault,
		Detail: meta.ActionDetail{
			ColumnName: colName,
		},
	}}
}

func handleAddConstraint(spec *ast.AlterTableSpec) []meta.AlterAction {
	if spec.Constraint == nil {
		return nil
	}
	switch spec.Constraint.Tp {
	case ast.ConstraintPrimaryKey:
		var cols []string
		for _, key := range spec.Constraint.Keys {
			cols = append(cols, key.Column.Name.L)
		}
		return []meta.AlterAction{{
			Type: meta.ActionAddPrimaryKey,
			Detail: meta.ActionDetail{
				IndexColumns: cols,
			},
		}}
	case ast.ConstraintIndex, ast.ConstraintKey:
		return addIndexAction(spec, meta.ActionAddIndex)
	case ast.ConstraintUniq, ast.ConstraintUniqKey, ast.ConstraintUniqIndex:
		return addIndexAction(spec, meta.ActionAddUniqueIndex)
	case ast.ConstraintFulltext:
		return addIndexAction(spec, meta.ActionAddFulltextIndex)
	case ast.ConstraintForeignKey:
		return handleAddForeignKey(spec)
	default:
		return nil
	}
}

func addIndexAction(spec *ast.AlterTableSpec, actionType meta.AlterActionType) []meta.AlterAction {
	cols := make([]string, 0, len(spec.Constraint.Keys))
	for _, key := range spec.Constraint.Keys {
		cols = append(cols, key.Column.Name.L)
	}
	return []meta.AlterAction{{
		Type: actionType,
		Detail: meta.ActionDetail{
			IndexName:    spec.Constraint.Name,
			IndexColumns: cols,
		},
	}}
}

func handleAddForeignKey(spec *ast.AlterTableSpec) []meta.AlterAction {
	srcCols := make([]string, 0, len(spec.Constraint.Keys))
	for _, key := range spec.Constraint.Keys {
		srcCols = append(srcCols, key.Column.Name.L)
	}
	refTable := ""
	var refCols []string
	if spec.Constraint.Refer != nil {
		refTable = spec.Constraint.Refer.Table.Name.L
		refCols = make([]string, 0, len(spec.Constraint.Refer.IndexPartSpecifications))
		for _, key := range spec.Constraint.Refer.IndexPartSpecifications {
			refCols = append(refCols, key.Column.Name.L)
		}
	}
	return []meta.AlterAction{{
		Type: meta.ActionAddForeignKey,
		Detail: meta.ActionDetail{
			ConstraintName: spec.Constraint.Name,
			IndexColumns:   srcCols,
			RefTable:       refTable,
			RefColumns:     refCols,
		},
	}}
}

func handleDropIndex(spec *ast.AlterTableSpec) []meta.AlterAction {
	return []meta.AlterAction{{
		Type: meta.ActionDropIndex,
		Detail: meta.ActionDetail{
			IndexName: spec.Name,
		},
	}}
}

func handleDropForeignKey(spec *ast.AlterTableSpec) []meta.AlterAction {
	return []meta.AlterAction{{
		Type: meta.ActionDropForeignKey,
		Detail: meta.ActionDetail{
			ConstraintName: spec.Name,
		},
	}}
}

func handleRenameIndex(spec *ast.AlterTableSpec) []meta.AlterAction {
	return []meta.AlterAction{{
		Type: meta.ActionRenameIndex,
		Detail: meta.ActionDetail{
			IndexName:    spec.ToKey.L,
			OldIndexName: spec.FromKey.L,
		},
	}}
}

func handleRenameTable(spec *ast.AlterTableSpec) []meta.AlterAction {
	name := ""
	if spec.NewTable != nil {
		name = spec.NewTable.Name.L
	}
	return []meta.AlterAction{{
		Type: meta.ActionRenameTable,
		Detail: meta.ActionDetail{
			ColumnName: name,
		},
	}}
}

func handleTableOptions(spec *ast.AlterTableSpec) []meta.AlterAction {
	var actions []meta.AlterAction
	for _, opt := range spec.Options {
		switch opt.Tp {
		case ast.TableOptionEngine:
			actions = append(actions, meta.AlterAction{
				Type:   meta.ActionChangeEngine,
				Detail: meta.ActionDetail{Engine: opt.StrValue},
			})
		case ast.TableOptionCharset:
			actions = append(actions, meta.AlterAction{
				Type:   meta.ActionConvertCharset,
				Detail: meta.ActionDetail{Charset: opt.StrValue},
			})
		case ast.TableOptionRowFormat:
			actions = append(actions, meta.AlterAction{
				Type:   meta.ActionChangeRowFormat,
				Detail: meta.ActionDetail{RowFormat: rowFormatString(opt.UintValue)},
			})
		}
	}
	return actions
}

func columnTypeString(col *ast.ColumnDef) string {
	if col.Tp == nil {
		return ""
	}
	var sb strings.Builder
	ctx := format.NewRestoreCtx(format.DefaultRestoreFlags, &sb)
	if err := col.Tp.Restore(ctx); err == nil {
		return sb.String()
	}
	return ""
}

func isNullable(col *ast.ColumnDef) bool {
	for _, opt := range col.Options {
		if opt.Tp == ast.ColumnOptionNotNull {
			return false
		}
	}
	return true
}

func defaultValueString(col *ast.ColumnDef) string {
	for _, opt := range col.Options {
		if opt.Tp == ast.ColumnOptionDefaultValue {
			if opt.Expr != nil {
				var sb strings.Builder
				ctx := format.NewRestoreCtx(format.DefaultRestoreFlags, &sb)
				if err := opt.Expr.Restore(ctx); err == nil {
					return sb.String()
				}
			}
		}
	}
	return ""
}

func positionString(pos *ast.ColumnPosition) string {
	if pos == nil {
		return ""
	}
	switch pos.Tp {
	case ast.ColumnPositionFirst:
		return "FIRST"
	case ast.ColumnPositionAfter:
		if pos.RelativeColumn != nil {
			return "AFTER " + pos.RelativeColumn.Name.L
		}
		return "AFTER"
	default:
		return ""
	}
}

func rowFormatString(v uint64) string {
	switch v {
	case ast.RowFormatDefault:
		return "DEFAULT"
	case ast.RowFormatDynamic:
		return "DYNAMIC"
	case ast.RowFormatCompressed:
		return "COMPRESSED"
	case ast.RowFormatRedundant:
		return "REDUNDANT"
	case ast.RowFormatCompact:
		return "COMPACT"
	default:
		return "UNKNOWN"
	}
}
