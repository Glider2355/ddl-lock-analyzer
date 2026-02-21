package predictor

import (
	"github.com/Glider2355/ddl-lock-analyzer/internal/meta"
)

// columnRules は ADD/DROP/RENAME COLUMN, SET/DROP DEFAULT のルールを返す。
func columnRules() []PredictionRule {
	return []PredictionRule{
		// ============================================================
		// ADD COLUMN rules (order: most specific → least specific)
		// ============================================================

		// ADD COLUMN (auto-increment)
		// MySQL docs: concurrent DML is NOT permitted for auto-increment columns
		// https://dev.mysql.com/doc/refman/8.0/en/innodb-online-ddl-operations.html#online-ddl-column-operations
		{
			ActionType:  meta.ActionAddColumn,
			Description: "ADD COLUMN (auto-increment)",
			Condition: func(a meta.AlterAction, _ *meta.TableMeta) bool {
				return a.Detail.IsAutoIncrement
			},
			Algorithm:    meta.AlgorithmInplace,
			Lock:         meta.LockShared,
			TableRebuild: true,
			Notes:        []string{"Auto-increment column requires ALGORITHM=INPLACE with LOCK=SHARED"},
			Warnings:     []string{"SHARED lock — DML writes blocked during column addition", "Table rebuild required — INPLACE ADD COLUMN with auto-increment"},
		},
		// ADD COLUMN (STORED generated)
		// MySQL docs: only ALGORITHM=COPY, no concurrent DML
		{
			ActionType:  meta.ActionAddColumn,
			Description: "ADD COLUMN (STORED generated)",
			Condition: func(a meta.AlterAction, _ *meta.TableMeta) bool {
				return a.Detail.GeneratedType == "STORED"
			},
			Algorithm:    meta.AlgorithmCopy,
			Lock:         meta.LockShared,
			TableRebuild: true,
			Warnings: []string{
				"STORED generated column requires ALGORITHM=COPY",
				"SHARED lock — DML writes blocked during operation",
				"Table rebuild required — server must evaluate expression for each row",
			},
		},
		// ADD COLUMN (VIRTUAL generated — partitioned table)
		// MySQL docs: "Adding a VIRTUAL is not an in-place operation for partitioned tables."
		// Neither INSTANT nor INPLACE is available — falls back to COPY.
		// https://dev.mysql.com/doc/refman/8.0/en/innodb-online-ddl-operations.html#online-ddl-generated-column-operations
		{
			ActionType:  meta.ActionAddColumn,
			Description: "ADD COLUMN (VIRTUAL generated, partitioned table)",
			Condition: func(a meta.AlterAction, tm *meta.TableMeta) bool {
				return a.Detail.GeneratedType == "VIRTUAL" && tm != nil && tm.IsPartitioned
			},
			Algorithm:    meta.AlgorithmCopy,
			Lock:         meta.LockShared,
			TableRebuild: true,
			Warnings: []string{
				"VIRTUAL generated column on partitioned table requires ALGORITHM=COPY",
				"SHARED lock — DML writes blocked during operation",
				"Table rebuild required",
			},
		},
		// ADD COLUMN (VIRTUAL generated — non-partitioned)
		// MySQL docs: INSTANT by default for non-partitioned tables
		// https://dev.mysql.com/doc/refman/8.0/en/innodb-online-ddl-operations.html#online-ddl-generated-column-operations
		{
			ActionType:  meta.ActionAddColumn,
			Description: "ADD COLUMN (VIRTUAL generated)",
			Condition: func(a meta.AlterAction, _ *meta.TableMeta) bool {
				return a.Detail.GeneratedType == "VIRTUAL"
			},
			Algorithm:    meta.AlgorithmInstant,
			Lock:         meta.LockNone,
			TableRebuild: false,
			Notes:        []string{"VIRTUAL generated column — INSTANT for non-partitioned tables (MySQL 8.0+)"},
		},
		// ADD COLUMN (trailing, NULLABLE)
		{
			ActionType:  meta.ActionAddColumn,
			Description: "ADD COLUMN (trailing, NULLABLE)",
			Condition: func(a meta.AlterAction, _ *meta.TableMeta) bool {
				return a.Detail.Position == "" && isNullablePtr(a.Detail.IsNullable)
			},
			Algorithm:    meta.AlgorithmInstant,
			Lock:         meta.LockNone,
			TableRebuild: false,
			Notes:        []string{"INSTANT algorithm available (MySQL 8.0.12+)", "No table rebuild required", "DML operations are not blocked"},
		},
		// ADD COLUMN (non-trailing, NULLABLE) — MySQL 8.0.29+
		{
			ActionType:  meta.ActionAddColumn,
			Description: "ADD COLUMN (non-trailing, NULLABLE)",
			Condition: func(a meta.AlterAction, _ *meta.TableMeta) bool {
				return a.Detail.Position != "" && isNullablePtr(a.Detail.IsNullable)
			},
			Algorithm:    meta.AlgorithmInstant,
			Lock:         meta.LockNone,
			TableRebuild: false,
			Notes:        []string{"INSTANT algorithm available (MySQL 8.0.29+)", "No table rebuild required"},
		},
		// ADD COLUMN (trailing, NOT NULL)
		// MySQL 8.0.12+: INSTANT is available for NOT NULL columns with DEFAULT value
		{
			ActionType:  meta.ActionAddColumn,
			Description: "ADD COLUMN (trailing, NOT NULL)",
			Condition: func(a meta.AlterAction, _ *meta.TableMeta) bool {
				return a.Detail.Position == "" && !isNullablePtr(a.Detail.IsNullable)
			},
			Algorithm:    meta.AlgorithmInstant,
			Lock:         meta.LockNone,
			TableRebuild: false,
			Notes: []string{
				"INSTANT algorithm available (MySQL 8.0.12+)",
				"NOT NULL column requires a DEFAULT value (explicit or implicit)",
			},
		},
		// ADD COLUMN (non-trailing, NOT NULL)
		// MySQL 8.0.29+: INSTANT supports any position
		{
			ActionType:  meta.ActionAddColumn,
			Description: "ADD COLUMN (non-trailing, NOT NULL)",
			Condition: func(a meta.AlterAction, _ *meta.TableMeta) bool {
				return a.Detail.Position != "" && !isNullablePtr(a.Detail.IsNullable)
			},
			Algorithm:    meta.AlgorithmInstant,
			Lock:         meta.LockNone,
			TableRebuild: false,
			Notes: []string{
				"INSTANT algorithm available (MySQL 8.0.29+)",
				"NOT NULL column requires a DEFAULT value (explicit or implicit)",
			},
		},

		// ============================================================
		// DROP COLUMN rules
		// ============================================================

		// DROP COLUMN (STORED generated — detected from metadata)
		{
			ActionType:  meta.ActionDropColumn,
			Description: "DROP COLUMN (STORED generated)",
			Condition: func(a meta.AlterAction, tm *meta.TableMeta) bool {
				col := findColumn(tm, a.Detail.ColumnName)
				return col != nil && isStoredGenerated(col)
			},
			Algorithm:    meta.AlgorithmInplace,
			Lock:         meta.LockNone,
			TableRebuild: true,
			Notes:        []string{"Dropping STORED generated column requires table rebuild"},
			Warnings:     []string{"Table rebuild required — may take significant time for large tables"},
		},
		// DROP COLUMN (VIRTUAL generated — partitioned table)
		// MySQL docs: "Dropping a VIRTUAL column can be performed instantly or in place for non-partitioned tables."
		// For partitioned tables, neither INSTANT nor INPLACE is available — falls back to COPY.
		// https://dev.mysql.com/doc/refman/8.0/en/innodb-online-ddl-operations.html#online-ddl-generated-column-operations
		{
			ActionType:  meta.ActionDropColumn,
			Description: "DROP COLUMN (VIRTUAL generated, partitioned table)",
			Condition: func(a meta.AlterAction, tm *meta.TableMeta) bool {
				if tm == nil || !tm.IsPartitioned {
					return false
				}
				col := findColumn(tm, a.Detail.ColumnName)
				return col != nil && isVirtualGenerated(col)
			},
			Algorithm:    meta.AlgorithmCopy,
			Lock:         meta.LockShared,
			TableRebuild: true,
			Warnings: []string{
				"VIRTUAL generated column on partitioned table requires ALGORITHM=COPY",
				"SHARED lock — DML writes blocked during operation",
				"Table rebuild required",
			},
		},
		// DROP COLUMN (VIRTUAL generated — non-partitioned)
		// MySQL docs: INSTANT for non-partitioned tables
		// https://dev.mysql.com/doc/refman/8.0/en/innodb-online-ddl-operations.html#online-ddl-generated-column-operations
		{
			ActionType:  meta.ActionDropColumn,
			Description: "DROP COLUMN (VIRTUAL generated)",
			Condition: func(a meta.AlterAction, tm *meta.TableMeta) bool {
				col := findColumn(tm, a.Detail.ColumnName)
				return col != nil && isVirtualGenerated(col)
			},
			Algorithm:    meta.AlgorithmInstant,
			Lock:         meta.LockNone,
			TableRebuild: false,
			Notes:        []string{"INSTANT algorithm for VIRTUAL generated column (MySQL 8.0+)"},
		},
		// DROP COLUMN (regular)
		// MySQL docs: INSTANT available (8.0.29+), rebuilds table
		// https://dev.mysql.com/doc/refman/8.0/en/innodb-online-ddl-operations.html#online-ddl-column-operations
		{
			ActionType:   meta.ActionDropColumn,
			Description:  "DROP COLUMN",
			Condition:    alwaysMatch,
			Algorithm:    meta.AlgorithmInstant,
			Lock:         meta.LockNone,
			TableRebuild: true,
			Notes:        []string{"INSTANT algorithm available (MySQL 8.0.29+)", "Existing rows retain dropped column data until rewritten"},
		},

		// ============================================================
		// RENAME COLUMN
		// ============================================================

		// RENAME COLUMN (referenced by foreign key)
		// MySQL docs: renaming a column referenced by FK requires INPLACE, not INSTANT
		// https://dev.mysql.com/doc/refman/8.0/en/innodb-online-ddl-operations.html#online-ddl-column-operations
		{
			ActionType:  meta.ActionRenameColumn,
			Description: "RENAME COLUMN (referenced by foreign key)",
			Condition: func(a meta.AlterAction, tm *meta.TableMeta) bool {
				colName := a.Detail.OldColumnName
				if colName == "" {
					colName = a.Detail.ColumnName
				}
				return isColumnReferencedByFK(colName, tm)
			},
			Algorithm:    meta.AlgorithmInplace,
			Lock:         meta.LockNone,
			TableRebuild: false,
			Notes:        []string{"Column referenced by foreign key — requires ALGORITHM=INPLACE (INSTANT not available)"},
		},
		// RENAME COLUMN (regular)
		// https://dev.mysql.com/doc/refman/8.0/en/innodb-online-ddl-operations.html#online-ddl-column-operations
		{
			ActionType:   meta.ActionRenameColumn,
			Description:  "RENAME COLUMN",
			Condition:    alwaysMatch,
			Algorithm:    meta.AlgorithmInstant,
			Lock:         meta.LockNone,
			TableRebuild: false,
			Notes:        []string{"INSTANT algorithm available (MySQL 8.0.28+)"},
		},

		// ============================================================
		// ALTER COLUMN SET/DROP DEFAULT
		// ============================================================
		{
			ActionType:   meta.ActionSetDefault,
			Description:  "ALTER COLUMN SET DEFAULT",
			Condition:    alwaysMatch,
			Algorithm:    meta.AlgorithmInstant,
			Lock:         meta.LockNone,
			TableRebuild: false,
			Notes:        []string{"Metadata-only change"},
		},
		{
			ActionType:   meta.ActionDropDefault,
			Description:  "ALTER COLUMN DROP DEFAULT",
			Condition:    alwaysMatch,
			Algorithm:    meta.AlgorithmInstant,
			Lock:         meta.LockNone,
			TableRebuild: false,
			Notes:        []string{"Metadata-only change"},
		},
	}
}

