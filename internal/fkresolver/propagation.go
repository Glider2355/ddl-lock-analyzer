package fkresolver

import (
	"fmt"
	"strings"

	"github.com/Glider2355/ddl-lock-analyzer/internal/meta"
)

// DetermineLockImpact は関連テーブルへのMDLロック影響を判定する。
func DetermineLockImpact(direction FKDirection, actions []meta.AlterAction, fk meta.ForeignKeyMeta) FKLockImpact {
	// アクションがFKカラムに直接関与するか確認
	for _, action := range actions {
		if action.Type == meta.ActionDropColumn {
			if isFKColumn(action.Detail.ColumnName, fk) {
				return FKLockImpact{
					MetadataLock: true,
					LockLevel:    meta.LockExclusive,
					Reason:       "DROP COLUMN on FK column — implicit FK constraint change",
				}
			}
		}
		if action.Type == meta.ActionModifyColumn || action.Type == meta.ActionChangeColumn {
			if isFKColumn(action.Detail.ColumnName, fk) {
				return FKLockImpact{
					MetadataLock: true,
					LockLevel:    meta.LockExclusive,
					Reason:       "Column type change on FK column — FK validation required",
				}
			}
		}
	}

	// デフォルトのMDL伝播（PARENT/CHILDどちらも同じ構造）
	if direction == FKDirectionParent || direction == FKDirectionChild {
		return FKLockImpact{
			MetadataLock: true,
			LockLevel:    meta.LockShared,
			Reason: fmt.Sprintf("FK: %s.%s → %s.%s",
				fk.SourceTable, strings.Join(fk.SourceColumns, ", "),
				fk.ReferencedTable, strings.Join(fk.ReferencedColumns, ", ")),
		}
	}

	return FKLockImpact{}
}

func isFKColumn(colName string, fk meta.ForeignKeyMeta) bool {
	for _, c := range fk.SourceColumns {
		if c == colName {
			return true
		}
	}
	for _, c := range fk.ReferencedColumns {
		if c == colName {
			return true
		}
	}
	return false
}
