package predictor

import (
	"strings"

	"github.com/Glider2355/ddl-lock-analyzer/internal/meta"
)

// modifyRules は MODIFY/CHANGE COLUMN のルールを返す。
func modifyRules() []PredictionRule {
	return []PredictionRule{
		// ============================================================
		// MODIFY COLUMN rules (order: most specific → least specific)
		// ============================================================

		// MODIFY COLUMN (generated column reorder — STORED or VIRTUAL)
		// MySQL docs: modifying stored/virtual column order requires COPY, no concurrent DML
		{
			ActionType:  meta.ActionModifyColumn,
			Description: "MODIFY COLUMN (generated column reorder)",
			Condition: func(a meta.AlterAction, tm *meta.TableMeta) bool {
				if a.Detail.Position == "" {
					return false
				}
				col := findColumn(tm, a.Detail.ColumnName)
				return col != nil && isGeneratedColumn(col)
			},
			Algorithm:    meta.AlgorithmCopy,
			Lock:         meta.LockShared,
			TableRebuild: true,
			Warnings: []string{
				"Modifying generated column order requires ALGORITHM=COPY",
				"SHARED lock — DML writes blocked during operation",
			},
		},
		// MODIFY COLUMN (ENUM/SET extension — add values at end, same storage size)
		{
			ActionType:  meta.ActionModifyColumn,
			Description: "MODIFY COLUMN (ENUM/SET extension)",
			Condition: func(a meta.AlterAction, tm *meta.TableMeta) bool {
				newType := strings.ToUpper(a.Detail.ColumnType)
				if !isEnumOrSetType(newType) {
					return false
				}
				col := findColumn(tm, a.Detail.ColumnName)
				if col == nil {
					return false
				}
				oldType := strings.ToUpper(col.ColumnType)
				// Both must be same base type (ENUM or SET)
				if strings.HasPrefix(newType, "ENUM") != strings.HasPrefix(oldType, "ENUM") {
					return false
				}
				if strings.HasPrefix(newType, "SET") != strings.HasPrefix(oldType, "SET") {
					return false
				}
				return true
			},
			Algorithm:    meta.AlgorithmInstant,
			Lock:         meta.LockNone,
			TableRebuild: false,
			Notes: []string{
				"INSTANT when adding new members to the end of the list without changing storage size",
				"Adding members in the middle or changing storage size requires COPY",
			},
		},
		// MODIFY COLUMN (VARCHAR extension — same length-byte boundary)
		// MySQL docs: in-place when staying within same length-byte boundary (0-255 vs 256+)
		{
			ActionType:  meta.ActionModifyColumn,
			Description: "MODIFY COLUMN (VARCHAR extension)",
			Condition: func(a meta.AlterAction, tm *meta.TableMeta) bool {
				newLen := extractVarcharLength(a.Detail.ColumnType)
				if newLen <= 0 {
					return false
				}
				col := findColumn(tm, a.Detail.ColumnName)
				if col == nil {
					return false
				}
				oldLen := extractVarcharLength(col.ColumnType)
				if oldLen <= 0 || newLen <= oldLen {
					return false
				}
				// Both within 0-255 or both within 256+
				return (oldLen <= 255 && newLen <= 255) || (oldLen >= 256 && newLen >= 256)
			},
			Algorithm:    meta.AlgorithmInplace,
			Lock:         meta.LockNone,
			TableRebuild: false,
			Notes: []string{
				"VARCHAR extension within same length-byte boundary — in-place, metadata-only",
				"Crossing the 255→256 byte boundary requires ALGORITHM=COPY (length byte changes from 1 to 2)",
			},
		},
		// MODIFY COLUMN (NULL → NOT NULL, same type)
		{
			ActionType:  meta.ActionModifyColumn,
			Description: "MODIFY COLUMN (NULL → NOT NULL)",
			Condition: func(a meta.AlterAction, tm *meta.TableMeta) bool {
				col := findColumn(tm, a.Detail.ColumnName)
				if col == nil {
					return false
				}
				sameType := strings.EqualFold(col.ColumnType, a.Detail.ColumnType)
				return sameType && col.IsNullable && !isNullablePtr(a.Detail.IsNullable)
			},
			Algorithm:    meta.AlgorithmInplace,
			Lock:         meta.LockNone,
			TableRebuild: true,
			Notes: []string{
				"INPLACE algorithm with table rebuild (NULL → NOT NULL conversion)",
				"Requires STRICT_ALL_TABLES or STRICT_TRANS_TABLES SQL mode",
			},
			Warnings: []string{"Table rebuild required — data validation for NOT NULL constraint; fails if column contains NULL values"},
		},
		// MODIFY COLUMN (NOT NULL → NULL, same type)
		// MySQL docs: INPLACE, rebuilds table, concurrent DML permitted
		{
			ActionType:  meta.ActionModifyColumn,
			Description: "MODIFY COLUMN (NOT NULL → NULL)",
			Condition: func(a meta.AlterAction, tm *meta.TableMeta) bool {
				col := findColumn(tm, a.Detail.ColumnName)
				if col == nil {
					return false
				}
				sameType := strings.EqualFold(col.ColumnType, a.Detail.ColumnType)
				return sameType && !col.IsNullable && isNullablePtr(a.Detail.IsNullable)
			},
			Algorithm:    meta.AlgorithmInplace,
			Lock:         meta.LockNone,
			TableRebuild: true,
			Notes:        []string{"INPLACE algorithm with table rebuild (NOT NULL → NULL conversion)"},
			Warnings:     []string{"Table rebuild required — may take significant time for large tables"},
		},
		// MODIFY COLUMN (reorder only — same type, same nullability, position change)
		// MySQL docs: INPLACE, rebuilds table, concurrent DML permitted
		{
			ActionType:  meta.ActionModifyColumn,
			Description: "MODIFY COLUMN (reorder columns)",
			Condition: func(a meta.AlterAction, tm *meta.TableMeta) bool {
				if a.Detail.Position == "" {
					return false
				}
				col := findColumn(tm, a.Detail.ColumnName)
				if col == nil {
					return false
				}
				if !strings.EqualFold(col.ColumnType, a.Detail.ColumnType) {
					return false
				}
				return col.IsNullable == isNullablePtr(a.Detail.IsNullable)
			},
			Algorithm:    meta.AlgorithmInplace,
			Lock:         meta.LockNone,
			TableRebuild: true,
			Notes:        []string{"Reordering columns requires table rebuild"},
			Warnings:     []string{"Table rebuild required — may take significant time for large tables"},
		},
		// MODIFY COLUMN (type change — with metadata confirmation)
		// MySQL docs: only ALGORITHM=COPY, no concurrent DML
		{
			ActionType:  meta.ActionModifyColumn,
			Description: "MODIFY COLUMN (type change)",
			Condition: func(a meta.AlterAction, tm *meta.TableMeta) bool {
				col := findColumn(tm, a.Detail.ColumnName)
				if col == nil {
					return true // no metadata — assume type change (conservative)
				}
				return !strings.EqualFold(col.ColumnType, a.Detail.ColumnType)
			},
			Algorithm:    meta.AlgorithmCopy,
			Lock:         meta.LockShared,
			TableRebuild: true,
			Warnings: []string{
				"SHARED lock — DML writes blocked during execution",
				"Table rebuild required — full table copy",
				"Consider using pt-online-schema-change or gh-ost for large tables",
			},
		},
		// MODIFY COLUMN (fallback — same type, no specific sub-case matched)
		// Treats as null rebuild (same type re-specification)
		{
			ActionType:   meta.ActionModifyColumn,
			Description:  "MODIFY COLUMN (rebuild)",
			Condition:    alwaysMatch,
			Algorithm:    meta.AlgorithmInplace,
			Lock:         meta.LockNone,
			TableRebuild: true,
			Notes:        []string{"INPLACE table rebuild (same type re-specification)"},
			Warnings:     []string{"Table rebuild required — may take significant time for large tables"},
		},

		// ============================================================
		// CHANGE COLUMN rules
		// ============================================================

		// CHANGE COLUMN (rename only — same type, detected from metadata)
		// MySQL docs: INSTANT (8.0.28+) when keeping same data type and only changing column name
		{
			ActionType:  meta.ActionChangeColumn,
			Description: "CHANGE COLUMN (rename only)",
			Condition: func(a meta.AlterAction, tm *meta.TableMeta) bool {
				col := findColumn(tm, a.Detail.OldColumnName)
				return col != nil && strings.EqualFold(col.ColumnType, a.Detail.ColumnType)
			},
			Algorithm:    meta.AlgorithmInstant,
			Lock:         meta.LockNone,
			TableRebuild: false,
			Notes:        []string{"INSTANT algorithm available (MySQL 8.0.28+) — rename only, same data type"},
		},
		// CHANGE COLUMN (type change — fallback)
		// MySQL docs: only ALGORITHM=COPY, no concurrent DML
		{
			ActionType:   meta.ActionChangeColumn,
			Description:  "CHANGE COLUMN (type change)",
			Condition:    alwaysMatch,
			Algorithm:    meta.AlgorithmCopy,
			Lock:         meta.LockShared,
			TableRebuild: true,
			Warnings: []string{
				"SHARED lock — DML writes blocked during execution",
				"Table rebuild required — full table copy",
				"Consider using pt-online-schema-change or gh-ost for large tables",
			},
		},
	}
}
