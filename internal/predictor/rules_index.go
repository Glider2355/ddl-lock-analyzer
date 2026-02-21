package predictor

import (
	"github.com/Glider2355/ddl-lock-analyzer/internal/meta"
)

// indexRules は INDEX, PRIMARY KEY, FOREIGN KEY のルールを返す。
func indexRules() []PredictionRule {
	return []PredictionRule{
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
				return tm != nil && !hasFulltextIndex(tm)
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
	}
}
