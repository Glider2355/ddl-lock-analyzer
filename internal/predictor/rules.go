package predictor

import (
	"strings"

	"github.com/Glider2355/ddl-lock-analyzer/internal/meta"
)

// PredictionRule defines a rule for predicting DDL lock behavior.
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
		// ADD COLUMN (末尾, NULLABLE)
		{
			ActionType:  meta.ActionAddColumn,
			Description: "ADD COLUMN (末尾, NULLABLE)",
			Condition: func(a meta.AlterAction, _ *meta.TableMeta) bool {
				return a.Detail.Position == "" && (a.Detail.IsNullable == nil || *a.Detail.IsNullable)
			},
			Algorithm:    meta.AlgorithmInstant,
			Lock:         meta.LockNone,
			TableRebuild: false,
			Notes:        []string{"INSTANT algorithm available (MySQL 8.0.12+)", "No table rebuild required", "DML operations are not blocked"},
		},
		// ADD COLUMN (途中/先頭, NULLABLE) — MySQL 8.0.29+
		{
			ActionType:  meta.ActionAddColumn,
			Description: "ADD COLUMN (途中/先頭, NULLABLE)",
			Condition: func(a meta.AlterAction, _ *meta.TableMeta) bool {
				return a.Detail.Position != "" && (a.Detail.IsNullable == nil || *a.Detail.IsNullable)
			},
			Algorithm:    meta.AlgorithmInstant,
			Lock:         meta.LockNone,
			TableRebuild: false,
			Notes:        []string{"INSTANT algorithm available (MySQL 8.0.29+)", "No table rebuild required"},
		},
		// ADD COLUMN (NOT NULL without DEFAULT)
		{
			ActionType:  meta.ActionAddColumn,
			Description: "ADD COLUMN (NOT NULL)",
			Condition: func(a meta.AlterAction, _ *meta.TableMeta) bool {
				return a.Detail.IsNullable != nil && !*a.Detail.IsNullable
			},
			Algorithm:    meta.AlgorithmInplace,
			Lock:         meta.LockNone,
			TableRebuild: true,
			Notes:        []string{"NOT NULL column requires table rebuild"},
			Warnings:     []string{"Table rebuild required — may take significant time for large tables"},
		},
		// DROP COLUMN
		{
			ActionType:   meta.ActionDropColumn,
			Description:  "DROP COLUMN",
			Condition:    alwaysMatch,
			Algorithm:    meta.AlgorithmInstant,
			Lock:         meta.LockNone,
			TableRebuild: false,
			Notes:        []string{"INSTANT algorithm available (MySQL 8.0.29+)"},
		},
		// RENAME COLUMN
		{
			ActionType:   meta.ActionRenameColumn,
			Description:  "RENAME COLUMN",
			Condition:    alwaysMatch,
			Algorithm:    meta.AlgorithmInstant,
			Lock:         meta.LockNone,
			TableRebuild: false,
			Notes:        []string{"Metadata-only change"},
		},
		// ALTER COLUMN SET DEFAULT
		{
			ActionType:   meta.ActionSetDefault,
			Description:  "ALTER COLUMN SET DEFAULT",
			Condition:    alwaysMatch,
			Algorithm:    meta.AlgorithmInstant,
			Lock:         meta.LockNone,
			TableRebuild: false,
			Notes:        []string{"Metadata-only change"},
		},
		// ALTER COLUMN DROP DEFAULT
		{
			ActionType:   meta.ActionDropDefault,
			Description:  "ALTER COLUMN DROP DEFAULT",
			Condition:    alwaysMatch,
			Algorithm:    meta.AlgorithmInstant,
			Lock:         meta.LockNone,
			TableRebuild: false,
			Notes:        []string{"Metadata-only change"},
		},
		// MODIFY COLUMN (type change)
		{
			ActionType:  meta.ActionModifyColumn,
			Description: "MODIFY COLUMN (type change)",
			Condition: func(a meta.AlterAction, tm *meta.TableMeta) bool {
				if tm == nil {
					return true // assume type change in offline mode
				}
				for _, col := range tm.Columns {
					if strings.EqualFold(col.Name, a.Detail.ColumnName) {
						return !strings.EqualFold(col.ColumnType, a.Detail.ColumnType)
					}
				}
				return true
			},
			Algorithm:    meta.AlgorithmCopy,
			Lock:         meta.LockExclusive,
			TableRebuild: true,
			Warnings: []string{
				"EXCLUSIVE lock will block all DML during execution",
				"Table rebuild required — full table copy",
				"Consider using pt-online-schema-change or gh-ost for large tables",
			},
		},
		// MODIFY COLUMN (NULL → NOT NULL only)
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
			Notes:        []string{"INPLACE algorithm with table rebuild (NULL → NOT NULL conversion)"},
			Warnings:     []string{"Table rebuild required — data validation for NOT NULL constraint"},
		},
		// CHANGE COLUMN (rename + type change)
		{
			ActionType:   meta.ActionChangeColumn,
			Description:  "CHANGE COLUMN",
			Condition:    alwaysMatch,
			Algorithm:    meta.AlgorithmCopy,
			Lock:         meta.LockExclusive,
			TableRebuild: true,
			Warnings: []string{
				"EXCLUSIVE lock will block all DML during execution",
				"Table rebuild required — full table copy",
				"Consider using pt-online-schema-change or gh-ost for large tables",
			},
		},
		// ADD INDEX
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
		// ADD FULLTEXT INDEX
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
		// ADD PRIMARY KEY
		{
			ActionType:   meta.ActionAddPrimaryKey,
			Description:  "ADD PRIMARY KEY",
			Condition:    alwaysMatch,
			Algorithm:    meta.AlgorithmInplace,
			Lock:         meta.LockNone,
			TableRebuild: true,
			Notes:        []string{"Table rebuild required — clustered index recreation"},
			Warnings:     []string{"Table rebuild required for large tables"},
		},
		// DROP PRIMARY KEY
		{
			ActionType:   meta.ActionDropPrimaryKey,
			Description:  "DROP PRIMARY KEY",
			Condition:    alwaysMatch,
			Algorithm:    meta.AlgorithmCopy,
			Lock:         meta.LockExclusive,
			TableRebuild: true,
			Warnings: []string{
				"EXCLUSIVE lock will block all DML during execution",
				"Table rebuild required — full table copy",
				"Consider using pt-online-schema-change or gh-ost for large tables",
			},
		},
		// ADD FOREIGN KEY
		{
			ActionType:   meta.ActionAddForeignKey,
			Description:  "ADD FOREIGN KEY",
			Condition:    alwaysMatch,
			Algorithm:    meta.AlgorithmInplace,
			Lock:         meta.LockNone,
			TableRebuild: false,
			Notes:        []string{"INPLACE when foreign_key_checks=OFF"},
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
		// CHANGE ENGINE (same)
		{
			ActionType:  meta.ActionChangeEngine,
			Description: "CHANGE ENGINE (same engine)",
			Condition: func(a meta.AlterAction, tm *meta.TableMeta) bool {
				if tm == nil {
					return false
				}
				return strings.EqualFold(a.Detail.Engine, tm.Engine)
			},
			Algorithm:    meta.AlgorithmInplace,
			Lock:         meta.LockNone,
			TableRebuild: true,
			Notes:        []string{"Same engine — table rebuild for defragmentation"},
		},
		// CHANGE ENGINE (different)
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
			Lock:         meta.LockExclusive,
			TableRebuild: true,
			Warnings: []string{
				"EXCLUSIVE lock will block all DML during execution",
				"Engine conversion requires full table copy",
			},
		},
		// CONVERT CHARACTER SET
		{
			ActionType:   meta.ActionConvertCharset,
			Description:  "CONVERT CHARACTER SET",
			Condition:    alwaysMatch,
			Algorithm:    meta.AlgorithmCopy,
			Lock:         meta.LockExclusive,
			TableRebuild: true,
			Warnings: []string{
				"EXCLUSIVE lock will block all DML during execution",
				"Character set conversion requires full table copy",
			},
		},
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
		// ADD PARTITION
		{
			ActionType:   meta.ActionAddPartition,
			Description:  "ADD PARTITION",
			Condition:    alwaysMatch,
			Algorithm:    meta.AlgorithmInplace,
			Lock:         meta.LockNone,
			TableRebuild: false,
			Notes:        []string{"Partition operation — no table rebuild"},
		},
		// DROP PARTITION
		{
			ActionType:   meta.ActionDropPartition,
			Description:  "DROP PARTITION",
			Condition:    alwaysMatch,
			Algorithm:    meta.AlgorithmInplace,
			Lock:         meta.LockNone,
			TableRebuild: false,
			Notes:        []string{"Partition operation — no table rebuild"},
		},
		// CHANGE ROW_FORMAT
		{
			ActionType:   meta.ActionChangeRowFormat,
			Description:  "CHANGE ROW_FORMAT",
			Condition:    alwaysMatch,
			Algorithm:    meta.AlgorithmInplace,
			Lock:         meta.LockNone,
			TableRebuild: true,
			Notes:        []string{"ROW_FORMAT change requires table rebuild"},
		},
	}
}

func alwaysMatch(_ meta.AlterAction, _ *meta.TableMeta) bool {
	return true
}
