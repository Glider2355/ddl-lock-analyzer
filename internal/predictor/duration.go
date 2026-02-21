package predictor

import (
	"fmt"

	"github.com/Glider2355/ddl-lock-analyzer/internal/meta"
)

// TableInfo はDBA判断用のテーブルメトリクスを保持する。
type TableInfo struct {
	RowCount   int64  `json:"row_count"`
	DataSize   int64  `json:"data_size_bytes"`
	IndexSize  int64  `json:"index_size_bytes"`
	IndexCount int    `json:"index_count"`
	Label      string `json:"-"`
}

// CollectTableInfo はメタデータからテーブルメトリクスを抽出する。
func CollectTableInfo(tableMeta *meta.TableMeta) TableInfo {
	if tableMeta == nil {
		return TableInfo{Label: "N/A (no table metadata)"}
	}

	info := TableInfo{
		RowCount:   tableMeta.RowCount,
		DataSize:   tableMeta.DataLength,
		IndexSize:  tableMeta.IndexLength,
		IndexCount: len(tableMeta.Indexes),
	}
	info.Label = formatTableInfo(info)
	return info
}

func formatTableInfo(info TableInfo) string {
	return fmt.Sprintf("rows: ~%s, data: %s, indexes: %d",
		formatCount(info.RowCount),
		formatSize(info.DataSize+info.IndexSize),
		info.IndexCount)
}

func formatSize(bytes int64) string {
	const (
		kb = 1024
		mb = kb * 1024
		gb = mb * 1024
	)
	switch {
	case bytes >= gb:
		return fmt.Sprintf("%.1fGB", float64(bytes)/float64(gb))
	case bytes >= mb:
		return fmt.Sprintf("%.0fMB", float64(bytes)/float64(mb))
	case bytes >= kb:
		return fmt.Sprintf("%.0fKB", float64(bytes)/float64(kb))
	default:
		return fmt.Sprintf("%dB", bytes)
	}
}

func formatCount(n int64) string {
	switch {
	case n >= 1_000_000:
		return fmt.Sprintf("%s,%03d,%03d", formatCount(n/1_000_000), (n/1000)%1000, n%1000)
	case n >= 1_000:
		return fmt.Sprintf("%d,%03d", n/1000, n%1000)
	default:
		return fmt.Sprintf("%d", n)
	}
}
