package predictor

import (
	"regexp"
	"strconv"
	"strings"

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

func defaultRules() []PredictionRule {
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
				return a.Detail.Position == "" && (a.Detail.IsNullable == nil || *a.Detail.IsNullable)
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
				return a.Detail.Position != "" && (a.Detail.IsNullable == nil || *a.Detail.IsNullable)
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
				return a.Detail.Position == "" && a.Detail.IsNullable != nil && !*a.Detail.IsNullable
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
				return a.Detail.Position != "" && a.Detail.IsNullable != nil && !*a.Detail.IsNullable
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
				if tm == nil {
					return false
				}
				for _, col := range tm.Columns {
					if strings.EqualFold(col.Name, a.Detail.ColumnName) {
						return strings.Contains(strings.ToUpper(col.Extra), "STORED GENERATED")
					}
				}
				return false
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
				for _, col := range tm.Columns {
					if strings.EqualFold(col.Name, a.Detail.ColumnName) {
						return strings.Contains(strings.ToUpper(col.Extra), "VIRTUAL GENERATED")
					}
				}
				return false
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
				if tm == nil {
					return false
				}
				for _, col := range tm.Columns {
					if strings.EqualFold(col.Name, a.Detail.ColumnName) {
						return strings.Contains(strings.ToUpper(col.Extra), "VIRTUAL GENERATED")
					}
				}
				return false
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
				if tm == nil {
					return false
				}
				colName := a.Detail.OldColumnName
				if colName == "" {
					colName = a.Detail.ColumnName
				}
				for _, fk := range tm.ReferencedBy {
					for _, refCol := range fk.ReferencedColumns {
						if strings.EqualFold(refCol, colName) {
							return true
						}
					}
				}
				return false
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

		// ============================================================
		// MODIFY COLUMN rules (order: most specific → least specific)
		// ============================================================

		// MODIFY COLUMN (generated column reorder — STORED or VIRTUAL)
		// MySQL docs: modifying stored/virtual column order requires COPY, no concurrent DML
		{
			ActionType:  meta.ActionModifyColumn,
			Description: "MODIFY COLUMN (generated column reorder)",
			Condition: func(a meta.AlterAction, tm *meta.TableMeta) bool {
				if tm == nil {
					return false
				}
				if a.Detail.Position == "" {
					return false
				}
				for _, col := range tm.Columns {
					if strings.EqualFold(col.Name, a.Detail.ColumnName) {
						extra := strings.ToUpper(col.Extra)
						return strings.Contains(extra, "GENERATED")
					}
				}
				return false
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
				if tm == nil {
					return false
				}
				newType := strings.ToUpper(a.Detail.ColumnType)
				if !strings.HasPrefix(newType, "ENUM") && !strings.HasPrefix(newType, "SET") {
					return false
				}
				for _, col := range tm.Columns {
					if strings.EqualFold(col.Name, a.Detail.ColumnName) {
						oldType := strings.ToUpper(col.ColumnType)
						// Both must be same base type (ENUM or SET)
						if strings.HasPrefix(newType, "ENUM") != strings.HasPrefix(oldType, "ENUM") {
							return false
						}
						if strings.HasPrefix(newType, "SET") != strings.HasPrefix(oldType, "SET") {
							return false
						}
						return true
					}
				}
				return false
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
				if tm == nil {
					return false
				}
				newLen := extractVarcharLength(a.Detail.ColumnType)
				if newLen <= 0 {
					return false
				}
				for _, col := range tm.Columns {
					if strings.EqualFold(col.Name, a.Detail.ColumnName) {
						oldLen := extractVarcharLength(col.ColumnType)
						if oldLen <= 0 || newLen <= oldLen {
							return false
						}
						// Both within 0-255 or both within 256+
						return (oldLen <= 255 && newLen <= 255) || (oldLen >= 256 && newLen >= 256)
					}
				}
				return false
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
				if tm == nil {
					return false
				}
				for _, col := range tm.Columns {
					if strings.EqualFold(col.Name, a.Detail.ColumnName) {
						sameType := strings.EqualFold(col.ColumnType, a.Detail.ColumnType)
						wasNullable := col.IsNullable
						isNotNull := a.Detail.IsNullable != nil && !*a.Detail.IsNullable
						return sameType && wasNullable && isNotNull
					}
				}
				return false
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
				if tm == nil {
					return false
				}
				for _, col := range tm.Columns {
					if strings.EqualFold(col.Name, a.Detail.ColumnName) {
						sameType := strings.EqualFold(col.ColumnType, a.Detail.ColumnType)
						wasNotNullable := !col.IsNullable
						isNullable := a.Detail.IsNullable == nil || *a.Detail.IsNullable
						return sameType && wasNotNullable && isNullable
					}
				}
				return false
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
				if tm == nil || a.Detail.Position == "" {
					return false
				}
				for _, col := range tm.Columns {
					if strings.EqualFold(col.Name, a.Detail.ColumnName) {
						sameType := strings.EqualFold(col.ColumnType, a.Detail.ColumnType)
						if !sameType {
							return false
						}
						// Check same nullability
						isNullable := a.Detail.IsNullable == nil || *a.Detail.IsNullable
						return col.IsNullable == isNullable
					}
				}
				return false
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
				if tm == nil {
					return true // no metadata — assume type change (conservative)
				}
				for _, col := range tm.Columns {
					if strings.EqualFold(col.Name, a.Detail.ColumnName) {
						return !strings.EqualFold(col.ColumnType, a.Detail.ColumnType)
					}
				}
				return true
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
				if tm == nil {
					return false
				}
				for _, col := range tm.Columns {
					if strings.EqualFold(col.Name, a.Detail.OldColumnName) {
						return strings.EqualFold(col.ColumnType, a.Detail.ColumnType)
					}
				}
				return false
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

		// ============================================================
		// INDEX rules
		// ============================================================

		// ADD INDEX (secondary)
		{
			ActionType:   meta.ActionAddIndex,
			Description:  "ADD INDEX",
			Condition:    alwaysMatch,
			Algorithm:    meta.AlgorithmInplace,
			Lock:         meta.LockNone,
			TableRebuild: false,
			Notes:        []string{"Online index creation — DML allowed during build"},
		},
		// ADD UNIQUE INDEX
		{
			ActionType:   meta.ActionAddUniqueIndex,
			Description:  "ADD UNIQUE INDEX",
			Condition:    alwaysMatch,
			Algorithm:    meta.AlgorithmInplace,
			Lock:         meta.LockNone,
			TableRebuild: false,
			Notes:        []string{"Online index creation — DML allowed during build"},
		},
		// ADD FULLTEXT INDEX (first FULLTEXT on table — may require rebuild)
		{
			ActionType:  meta.ActionAddFulltextIndex,
			Description: "ADD FULLTEXT INDEX (first on table)",
			Condition: func(_ meta.AlterAction, tm *meta.TableMeta) bool {
				if tm == nil {
					return false
				}
				for _, idx := range tm.Indexes {
					if strings.EqualFold(idx.IndexType, "FULLTEXT") {
						return false // already has a FULLTEXT index
					}
				}
				return true
			},
			Algorithm:    meta.AlgorithmInplace,
			Lock:         meta.LockShared,
			TableRebuild: true,
			Notes:        []string{"First FULLTEXT index may require table rebuild if no user-defined FTS_DOC_ID column"},
			Warnings:     []string{"SHARED lock — DML writes blocked during index creation", "Table rebuild may be required"},
		},
		// ADD FULLTEXT INDEX (subsequent — no rebuild)
		{
			ActionType:   meta.ActionAddFulltextIndex,
			Description:  "ADD FULLTEXT INDEX",
			Condition:    alwaysMatch,
			Algorithm:    meta.AlgorithmInplace,
			Lock:         meta.LockShared,
			TableRebuild: false,
			Notes:        []string{"FULLTEXT index creation requires SHARED lock"},
			Warnings:     []string{"SHARED lock — DML writes blocked during index creation"},
		},
		// ADD SPATIAL INDEX
		// MySQL docs: INPLACE, requires at minimum LOCK=SHARED, no concurrent DML
		{
			ActionType:   meta.ActionAddSpatialIndex,
			Description:  "ADD SPATIAL INDEX",
			Condition:    alwaysMatch,
			Algorithm:    meta.AlgorithmInplace,
			Lock:         meta.LockShared,
			TableRebuild: false,
			Notes:        []string{"SPATIAL index creation requires at minimum LOCK=SHARED"},
			Warnings:     []string{"SHARED lock — DML writes blocked during index creation"},
		},
		// DROP INDEX
		{
			ActionType:   meta.ActionDropIndex,
			Description:  "DROP INDEX",
			Condition:    alwaysMatch,
			Algorithm:    meta.AlgorithmInplace,
			Lock:         meta.LockNone,
			TableRebuild: false,
			Notes:        []string{"Metadata-only change"},
		},
		// RENAME INDEX
		{
			ActionType:   meta.ActionRenameIndex,
			Description:  "RENAME INDEX",
			Condition:    alwaysMatch,
			Algorithm:    meta.AlgorithmInplace,
			Lock:         meta.LockNone,
			TableRebuild: false,
			Notes:        []string{"Metadata-only change"},
		},

		// ============================================================
		// PRIMARY KEY rules
		// ============================================================

		// ADD PRIMARY KEY
		// MySQL docs: INPLACE, rebuilds table, concurrent DML permitted
		// Note: INPLACE not permitted if columns need NULL→NOT NULL conversion
		{
			ActionType:   meta.ActionAddPrimaryKey,
			Description:  "ADD PRIMARY KEY",
			Condition:    alwaysMatch,
			Algorithm:    meta.AlgorithmInplace,
			Lock:         meta.LockNone,
			TableRebuild: true,
			Notes: []string{
				"Table rebuild required — clustered index recreation",
				"ALGORITHM=INPLACE is not permitted if columns must be converted to NOT NULL",
			},
			Warnings: []string{"Table rebuild required — expensive operation for large tables"},
		},
		// DROP PRIMARY KEY
		// MySQL docs: only ALGORITHM=COPY, no concurrent DML (LOCK=NONE not permitted)
		{
			ActionType:   meta.ActionDropPrimaryKey,
			Description:  "DROP PRIMARY KEY",
			Condition:    alwaysMatch,
			Algorithm:    meta.AlgorithmCopy,
			Lock:         meta.LockShared,
			TableRebuild: true,
			Warnings: []string{
				"SHARED lock — DML writes blocked during execution",
				"Table rebuild required — full table copy",
				"Consider dropping and adding primary key in a single ALTER TABLE statement for INPLACE support",
			},
		},

		// ============================================================
		// FOREIGN KEY rules
		// ============================================================

		// ADD FOREIGN KEY
		// MySQL docs: INPLACE only when foreign_key_checks=OFF
		// When foreign_key_checks=ON (default), only ALGORITHM=COPY
		{
			ActionType:   meta.ActionAddForeignKey,
			Description:  "ADD FOREIGN KEY",
			Condition:    alwaysMatch,
			Algorithm:    meta.AlgorithmCopy,
			Lock:         meta.LockShared,
			TableRebuild: false,
			Notes: []string{
				"Default behavior with foreign_key_checks=ON (default): ALGORITHM=COPY",
				"ALGORITHM=INPLACE with LOCK=NONE is available only when foreign_key_checks=OFF",
			},
			Warnings: []string{"SHARED lock — DML writes blocked during execution; set foreign_key_checks=OFF for INPLACE operation"},
		},
		// DROP FOREIGN KEY
		{
			ActionType:   meta.ActionDropForeignKey,
			Description:  "DROP FOREIGN KEY",
			Condition:    alwaysMatch,
			Algorithm:    meta.AlgorithmInplace,
			Lock:         meta.LockNone,
			TableRebuild: false,
			Notes:        []string{"Metadata-only change"},
		},

		// ============================================================
		// TABLE operations
		// ============================================================

		// RENAME TABLE
		{
			ActionType:   meta.ActionRenameTable,
			Description:  "RENAME TABLE",
			Condition:    alwaysMatch,
			Algorithm:    meta.AlgorithmInstant,
			Lock:         meta.LockNone,
			TableRebuild: false,
			Notes:        []string{"Metadata-only change"},
		},
		// CHANGE ENGINE (same engine — null rebuild)
		// MySQL docs: INPLACE, rebuilds table, concurrent DML permitted
		{
			ActionType:  meta.ActionChangeEngine,
			Description: "CHANGE ENGINE (same engine — null rebuild)",
			Condition: func(a meta.AlterAction, tm *meta.TableMeta) bool {
				if tm == nil {
					return false
				}
				return strings.EqualFold(a.Detail.Engine, tm.Engine)
			},
			Algorithm:    meta.AlgorithmInplace,
			Lock:         meta.LockNone,
			TableRebuild: true,
			Notes:        []string{"Same engine — table rebuild for defragmentation (equivalent to ALTER TABLE ... FORCE)"},
		},
		// CHANGE ENGINE (different engine)
		// Full table copy to new engine format
		{
			ActionType:  meta.ActionChangeEngine,
			Description: "CHANGE ENGINE (different engine)",
			Condition: func(a meta.AlterAction, tm *meta.TableMeta) bool {
				if tm == nil {
					return true
				}
				return !strings.EqualFold(a.Detail.Engine, tm.Engine)
			},
			Algorithm:    meta.AlgorithmCopy,
			Lock:         meta.LockShared,
			TableRebuild: true,
			Warnings: []string{
				"SHARED lock — DML writes blocked during execution",
				"Engine conversion requires full table copy",
			},
		},
		// CONVERT CHARACTER SET
		// MySQL docs: INPLACE, rebuilds table, concurrent DML NOT permitted
		{
			ActionType:   meta.ActionConvertCharset,
			Description:  "CONVERT CHARACTER SET",
			Condition:    alwaysMatch,
			Algorithm:    meta.AlgorithmInplace,
			Lock:         meta.LockShared,
			TableRebuild: true,
			Notes:        []string{"INPLACE algorithm with table rebuild when character encoding differs"},
			Warnings: []string{
				"SHARED lock — DML writes blocked during execution",
				"Table rebuild required if new character encoding differs from current",
			},
		},
		// CHANGE ROW_FORMAT
		// MySQL docs: INPLACE, rebuilds table, concurrent DML permitted
		{
			ActionType:   meta.ActionChangeRowFormat,
			Description:  "CHANGE ROW_FORMAT",
			Condition:    alwaysMatch,
			Algorithm:    meta.AlgorithmInplace,
			Lock:         meta.LockNone,
			TableRebuild: true,
			Notes:        []string{"ROW_FORMAT change requires table rebuild"},
		},
		// CHANGE KEY_BLOCK_SIZE
		// MySQL docs: INPLACE, rebuilds table, concurrent DML permitted
		{
			ActionType:   meta.ActionChangeKeyBlockSize,
			Description:  "CHANGE KEY_BLOCK_SIZE",
			Condition:    alwaysMatch,
			Algorithm:    meta.AlgorithmInplace,
			Lock:         meta.LockNone,
			TableRebuild: true,
			Notes:        []string{"KEY_BLOCK_SIZE change requires table rebuild"},
		},
		// CHANGE AUTO_INCREMENT value
		// MySQL docs: INPLACE, no rebuild, concurrent DML permitted
		{
			ActionType:   meta.ActionChangeAutoIncrement,
			Description:  "CHANGE AUTO_INCREMENT value",
			Condition:    alwaysMatch,
			Algorithm:    meta.AlgorithmInplace,
			Lock:         meta.LockNone,
			TableRebuild: false,
			Notes:        []string{"Only modifies the in-memory auto-increment counter, not the data file"},
		},
		// FORCE REBUILD (ALTER TABLE ... FORCE)
		// MySQL docs: INPLACE, rebuilds table, concurrent DML permitted
		{
			ActionType:   meta.ActionForceRebuild,
			Description:  "ALTER TABLE ... FORCE (rebuild)",
			Condition:    alwaysMatch,
			Algorithm:    meta.AlgorithmInplace,
			Lock:         meta.LockNone,
			TableRebuild: true,
			Notes:        []string{"Online table rebuild — equivalent to ALTER TABLE ... ENGINE=InnoDB"},
			Warnings:     []string{"Table rebuild required — may take significant time for large tables"},
		},

		// ============================================================
		// PARTITION operations
		// ============================================================

		// ADD PARTITION (HASH/KEY — requires data redistribution)
		// MySQL docs: INPLACE, no concurrent DML, LOCK=SHARED minimum
		// https://dev.mysql.com/doc/refman/8.0/en/innodb-online-ddl-operations.html#online-ddl-partitioning-operations
		{
			ActionType:  meta.ActionAddPartition,
			Description: "ADD PARTITION (HASH/KEY)",
			Condition: func(_ meta.AlterAction, tm *meta.TableMeta) bool {
				if tm == nil {
					return false
				}
				pt := strings.ToUpper(tm.PartitionType)
				return pt == "HASH" || pt == "KEY" || pt == "LINEAR HASH" || pt == "LINEAR KEY"
			},
			Algorithm:    meta.AlgorithmInplace,
			Lock:         meta.LockShared,
			TableRebuild: false,
			Notes:        []string{"HASH/KEY partition — data is copied between partitions"},
			Warnings:     []string{"SHARED lock — DML writes blocked during partition addition"},
		},
		// ADD PARTITION (RANGE/LIST — the common case)
		// MySQL docs: INPLACE, concurrent DML permitted, LOCK=NONE allowed
		// https://dev.mysql.com/doc/refman/8.0/en/innodb-online-ddl-operations.html#online-ddl-partitioning-operations
		{
			ActionType:   meta.ActionAddPartition,
			Description:  "ADD PARTITION",
			Condition:    alwaysMatch,
			Algorithm:    meta.AlgorithmInplace,
			Lock:         meta.LockNone,
			TableRebuild: false,
			Notes: []string{
				"INPLACE for RANGE/LIST partitions — no data copying",
				"For HASH/KEY partitions: data is copied between partitions and requires LOCK=SHARED",
			},
		},
		// DROP PARTITION (HASH/KEY — requires data redistribution)
		// MySQL docs: INPLACE, no concurrent DML, LOCK=SHARED minimum
		// https://dev.mysql.com/doc/refman/8.0/en/innodb-online-ddl-operations.html#online-ddl-partitioning-operations
		{
			ActionType:  meta.ActionDropPartition,
			Description: "DROP PARTITION (HASH/KEY)",
			Condition: func(_ meta.AlterAction, tm *meta.TableMeta) bool {
				if tm == nil {
					return false
				}
				pt := strings.ToUpper(tm.PartitionType)
				return pt == "HASH" || pt == "KEY" || pt == "LINEAR HASH" || pt == "LINEAR KEY"
			},
			Algorithm:    meta.AlgorithmInplace,
			Lock:         meta.LockShared,
			TableRebuild: false,
			Notes:        []string{"HASH/KEY partition — data is redistributed between remaining partitions"},
			Warnings:     []string{"SHARED lock — DML writes blocked during partition drop", "Data in the partition will be permanently deleted"},
		},
		// DROP PARTITION (RANGE/LIST)
		// MySQL docs: INPLACE, concurrent DML permitted
		// https://dev.mysql.com/doc/refman/8.0/en/innodb-online-ddl-operations.html#online-ddl-partitioning-operations
		{
			ActionType:   meta.ActionDropPartition,
			Description:  "DROP PARTITION",
			Condition:    alwaysMatch,
			Algorithm:    meta.AlgorithmInplace,
			Lock:         meta.LockNone,
			TableRebuild: false,
			Notes:        []string{"Deletes data stored in the partition and drops it"},
			Warnings:     []string{"Data in the partition will be permanently deleted"},
		},
		// TRUNCATE PARTITION
		// MySQL docs: INPLACE, concurrent DML permitted
		{
			ActionType:   meta.ActionTruncatePartition,
			Description:  "TRUNCATE PARTITION",
			Condition:    alwaysMatch,
			Algorithm:    meta.AlgorithmInplace,
			Lock:         meta.LockNone,
			TableRebuild: false,
			Notes:        []string{"Truncates data in the partition without dropping it"},
		},
		// EXCHANGE PARTITION
		// MySQL docs: INPLACE, concurrent DML permitted
		{
			ActionType:   meta.ActionExchangePartition,
			Description:  "EXCHANGE PARTITION",
			Condition:    alwaysMatch,
			Algorithm:    meta.AlgorithmInplace,
			Lock:         meta.LockNone,
			TableRebuild: false,
			Notes:        []string{"Exchanges partition data with a non-partitioned table"},
		},
		// COALESCE PARTITION
		// MySQL docs: INPLACE, no concurrent DML (LOCK=SHARED minimum)
		{
			ActionType:   meta.ActionCoalescePartition,
			Description:  "COALESCE PARTITION",
			Condition:    alwaysMatch,
			Algorithm:    meta.AlgorithmInplace,
			Lock:         meta.LockShared,
			TableRebuild: false,
			Notes:        []string{"Data is copied between partitions"},
			Warnings:     []string{"SHARED lock — DML writes blocked during partition coalescing"},
		},
		// REORGANIZE PARTITION
		// MySQL docs: INPLACE, no concurrent DML (LOCK=SHARED minimum)
		{
			ActionType:   meta.ActionReorganizePartition,
			Description:  "REORGANIZE PARTITION",
			Condition:    alwaysMatch,
			Algorithm:    meta.AlgorithmInplace,
			Lock:         meta.LockShared,
			TableRebuild: false,
			Notes:        []string{"Data is copied between partitions"},
			Warnings:     []string{"SHARED lock — DML writes blocked during partition reorganization"},
		},
		// REBUILD PARTITION
		// MySQL docs: INPLACE, no concurrent DML (LOCK=SHARED minimum)
		{
			ActionType:   meta.ActionRebuildPartition,
			Description:  "REBUILD PARTITION",
			Condition:    alwaysMatch,
			Algorithm:    meta.AlgorithmInplace,
			Lock:         meta.LockShared,
			TableRebuild: false,
			Warnings:     []string{"SHARED lock — DML writes blocked during partition rebuild"},
		},
		// PARTITION BY
		// MySQL docs: only ALGORITHM=COPY, no concurrent DML
		{
			ActionType:   meta.ActionPartitionBy,
			Description:  "PARTITION BY",
			Condition:    alwaysMatch,
			Algorithm:    meta.AlgorithmCopy,
			Lock:         meta.LockShared,
			TableRebuild: true,
			Warnings: []string{
				"SHARED lock — DML writes blocked during operation",
				"Table rebuild required — partitioning structure change",
			},
		},
		// REMOVE PARTITIONING
		// MySQL docs: only ALGORITHM=COPY, no concurrent DML
		// https://dev.mysql.com/doc/refman/8.0/en/innodb-online-ddl-operations.html#online-ddl-partitioning-operations
		{
			ActionType:   meta.ActionRemovePartitioning,
			Description:  "REMOVE PARTITIONING",
			Condition:    alwaysMatch,
			Algorithm:    meta.AlgorithmCopy,
			Lock:         meta.LockShared,
			TableRebuild: true,
			Warnings: []string{
				"SHARED lock — DML writes blocked during operation",
				"Table rebuild required — removing partitioning structure",
			},
		},

		// ============================================================
		// SPECIFY CHARACTER SET (ALTER TABLE ... CHARACTER SET = xxx)
		// Different from CONVERT TO CHARACTER SET
		// ============================================================

		// SPECIFY CHARACTER SET
		// MySQL docs: INPLACE, concurrent DML permitted, rebuilds if encoding differs
		// https://dev.mysql.com/doc/refman/8.0/en/innodb-online-ddl-operations.html#online-ddl-table-operations
		{
			ActionType:   meta.ActionSpecifyCharset,
			Description:  "SPECIFY CHARACTER SET",
			Condition:    alwaysMatch,
			Algorithm:    meta.AlgorithmInplace,
			Lock:         meta.LockNone,
			TableRebuild: true,
			Notes: []string{
				"Changes the default character set for the table (does not convert existing columns)",
				"Rebuilds table if new character encoding differs from current",
				"Different from CONVERT TO CHARACTER SET which converts all existing columns",
			},
		},

		// ============================================================
		// SET TABLE STATISTICS
		// ============================================================

		// SET TABLE STATISTICS (STATS_PERSISTENT, STATS_AUTO_RECALC, STATS_SAMPLE_PAGES)
		// MySQL docs: INPLACE, concurrent DML permitted, no rebuild
		// https://dev.mysql.com/doc/refman/8.0/en/innodb-online-ddl-operations.html#online-ddl-table-operations
		{
			ActionType:   meta.ActionSetTableStats,
			Description:  "SET TABLE STATISTICS",
			Condition:    alwaysMatch,
			Algorithm:    meta.AlgorithmInplace,
			Lock:         meta.LockNone,
			TableRebuild: false,
			Notes:        []string{"Metadata-only change — modifies persistent statistics settings"},
		},

		// ============================================================
		// TABLE ENCRYPTION
		// ============================================================

		// TABLE ENCRYPTION (file-per-table)
		// MySQL docs: COPY, no concurrent DML, rebuilds table
		// https://dev.mysql.com/doc/refman/8.0/en/innodb-online-ddl-operations.html#online-ddl-table-operations
		{
			ActionType:   meta.ActionTableEncryption,
			Description:  "TABLE ENCRYPTION",
			Condition:    alwaysMatch,
			Algorithm:    meta.AlgorithmCopy,
			Lock:         meta.LockShared,
			TableRebuild: true,
			Warnings: []string{
				"SHARED lock — DML writes blocked during encryption change",
				"Table rebuild required — full table copy for encryption/decryption",
			},
		},

		// ============================================================
		// Additional PARTITION operations
		// ============================================================

		// CHECK PARTITION
		// MySQL docs: INPLACE, concurrent DML permitted, no rebuild
		// https://dev.mysql.com/doc/refman/8.0/en/innodb-online-ddl-operations.html#online-ddl-partitioning-operations
		{
			ActionType:   meta.ActionCheckPartition,
			Description:  "CHECK PARTITION",
			Condition:    alwaysMatch,
			Algorithm:    meta.AlgorithmInplace,
			Lock:         meta.LockNone,
			TableRebuild: false,
			Notes:        []string{"Partition validation — read-only operation"},
		},
		// OPTIMIZE PARTITION
		// MySQL docs: ALGORITHM and LOCK clauses ignored, rebuilds entire table
		// https://dev.mysql.com/doc/refman/8.0/en/innodb-online-ddl-operations.html#online-ddl-partitioning-operations
		{
			ActionType:   meta.ActionOptimizePartition,
			Description:  "OPTIMIZE PARTITION",
			Condition:    alwaysMatch,
			Algorithm:    meta.AlgorithmCopy,
			Lock:         meta.LockShared,
			TableRebuild: true,
			Notes:        []string{"Rebuilds entire table — ALGORITHM and LOCK clauses are ignored by MySQL"},
			Warnings: []string{
				"SHARED lock — DML writes blocked during optimization",
				"Table rebuild required — entire table is rebuilt regardless of partition scope",
			},
		},
		// REPAIR PARTITION
		// MySQL docs: INPLACE, concurrent DML permitted, no rebuild
		// https://dev.mysql.com/doc/refman/8.0/en/innodb-online-ddl-operations.html#online-ddl-partitioning-operations
		{
			ActionType:   meta.ActionRepairPartition,
			Description:  "REPAIR PARTITION",
			Condition:    alwaysMatch,
			Algorithm:    meta.AlgorithmInplace,
			Lock:         meta.LockNone,
			TableRebuild: false,
			Notes:        []string{"Partition repair operation"},
		},
		// DISCARD PARTITION TABLESPACE
		// MySQL docs: Instant=No, In Place=No, only ALGORITHM=DEFAULT, LOCK=DEFAULT permitted
		// No online DDL support — neither INSTANT nor INPLACE is available
		// https://dev.mysql.com/doc/refman/8.0/en/innodb-online-ddl-operations.html#online-ddl-partitioning-operations
		{
			ActionType:   meta.ActionDiscardPartitionTablespace,
			Description:  "DISCARD PARTITION TABLESPACE",
			Condition:    alwaysMatch,
			Algorithm:    meta.AlgorithmCopy,
			Lock:         meta.LockExclusive,
			TableRebuild: false,
			Notes:        []string{"Only ALGORITHM=DEFAULT and LOCK=DEFAULT are permitted by MySQL"},
			Warnings:     []string{"EXCLUSIVE lock — no concurrent read or write access during tablespace discard"},
		},
		// IMPORT PARTITION TABLESPACE
		// MySQL docs: Instant=No, In Place=No, only ALGORITHM=DEFAULT, LOCK=DEFAULT permitted
		// No online DDL support — neither INSTANT nor INPLACE is available
		// https://dev.mysql.com/doc/refman/8.0/en/innodb-online-ddl-operations.html#online-ddl-partitioning-operations
		{
			ActionType:   meta.ActionImportPartitionTablespace,
			Description:  "IMPORT PARTITION TABLESPACE",
			Condition:    alwaysMatch,
			Algorithm:    meta.AlgorithmCopy,
			Lock:         meta.LockExclusive,
			TableRebuild: false,
			Notes:        []string{"Only ALGORITHM=DEFAULT and LOCK=DEFAULT are permitted by MySQL"},
			Warnings:     []string{"EXCLUSIVE lock — no concurrent read or write access during tablespace import"},
		},
	}
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
