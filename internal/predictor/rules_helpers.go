package predictor

import (
	"strings"

	"github.com/Glider2355/ddl-lock-analyzer/internal/meta"
)

// findColumn はTableMetaから指定名のカラムを検索する。
// 見つからない場合はnilを返す。
func findColumn(tm *meta.TableMeta, name string) *meta.ColumnMeta {
	if tm == nil {
		return nil
	}
	for i := range tm.Columns {
		if strings.EqualFold(tm.Columns[i].Name, name) {
			return &tm.Columns[i]
		}
	}
	return nil
}

// isGeneratedColumn はカラムが生成列（STORED or VIRTUAL）かを判定する。
func isGeneratedColumn(col *meta.ColumnMeta) bool {
	return strings.Contains(strings.ToUpper(col.Extra), "GENERATED")
}

// isStoredGenerated はカラムがSTORED生成列かを判定する。
func isStoredGenerated(col *meta.ColumnMeta) bool {
	return strings.Contains(strings.ToUpper(col.Extra), "STORED GENERATED")
}

// isVirtualGenerated はカラムがVIRTUAL生成列かを判定する。
func isVirtualGenerated(col *meta.ColumnMeta) bool {
	return strings.Contains(strings.ToUpper(col.Extra), "VIRTUAL GENERATED")
}

// isNullablePtr はActionDetailのIsNullableポインタからnullable判定を行う。
// nilの場合はtrueを返す（デフォルトnullable）。
func isNullablePtr(isNullable *bool) bool {
	return isNullable == nil || *isNullable
}

// isHashOrKeyPartition はパーティションタイプがHASHまたはKEY系かを判定する。
func isHashOrKeyPartition(partitionType string) bool {
	pt := strings.ToUpper(partitionType)
	return pt == "HASH" || pt == "KEY" || pt == "LINEAR HASH" || pt == "LINEAR KEY"
}

// hasFulltextIndex はテーブルにFULLTEXTインデックスが存在するかを判定する。
func hasFulltextIndex(tm *meta.TableMeta) bool {
	if tm == nil {
		return false
	}
	for _, idx := range tm.Indexes {
		if strings.EqualFold(idx.IndexType, "FULLTEXT") {
			return true
		}
	}
	return false
}

// isColumnReferencedByFK はカラムがReferencedByのFK参照対象かを判定する。
func isColumnReferencedByFK(colName string, tm *meta.TableMeta) bool {
	if tm == nil {
		return false
	}
	for _, fk := range tm.ReferencedBy {
		for _, refCol := range fk.ReferencedColumns {
			if strings.EqualFold(refCol, colName) {
				return true
			}
		}
	}
	return false
}

// isEnumOrSetType はカラム型がENUMまたはSETで始まるかを判定する。
func isEnumOrSetType(colType string) bool {
	upper := strings.ToUpper(colType)
	return strings.HasPrefix(upper, "ENUM") || strings.HasPrefix(upper, "SET")
}
