package predictor

import (
	"strings"

	"github.com/Glider2355/ddl-lock-analyzer/internal/meta"
)

// tableRules は RENAME TABLE, ENGINE, CHARSET 等のテーブル操作ルールを返す。
func tableRules() []PredictionRule {
	return []PredictionRule{
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
		// SPECIFY CHARACTER SET (ALTER TABLE ... CHARACTER SET = xxx)
		// Different from CONVERT TO CHARACTER SET
		// ============================================================
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
	}
}
