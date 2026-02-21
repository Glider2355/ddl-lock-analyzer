package predictor

import (
	"github.com/Glider2355/ddl-lock-analyzer/internal/meta"
)

// partitionRules は全パーティション操作のルールを返す。
func partitionRules() []PredictionRule {
	return []PredictionRule{
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
				return tm != nil && isHashOrKeyPartition(tm.PartitionType)
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
				return tm != nil && isHashOrKeyPartition(tm.PartitionType)
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
		// Additional PARTITION operations
		// ============================================================

		// CHECK PARTITION
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
