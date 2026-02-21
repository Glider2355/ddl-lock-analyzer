package predictor

import (
	"fmt"

	"github.com/Glider2355/ddl-lock-analyzer/internal/meta"
)

// DurationEstimate represents an estimated duration range.
type DurationEstimate struct {
	MinSeconds float64 `json:"min"`
	MaxSeconds float64 `json:"max"`
	Label      string  `json:"label"`
}

// EstimateDuration calculates estimated duration based on table metadata and operation type.
func EstimateDuration(algorithm meta.Algorithm, tableRebuild bool, tableMeta *meta.TableMeta) DurationEstimate {
	if tableMeta == nil {
		return DurationEstimate{Label: "N/A (offline mode)"}
	}

	switch algorithm {
	case meta.AlgorithmInstant:
		return DurationEstimate{
			MinSeconds: 0,
			MaxSeconds: 0,
			Label:      "~0s (metadata only)",
		}
	case meta.AlgorithmInplace:
		return estimateInplace(tableRebuild, tableMeta)
	case meta.AlgorithmCopy:
		return estimateCopy(tableMeta)
	default:
		return DurationEstimate{Label: "unknown"}
	}
}

func estimateInplace(rebuild bool, tm *meta.TableMeta) DurationEstimate {
	if !rebuild {
		// No rebuild: proportional to data length but fast
		dataMB := float64(tm.DataLength) / (1024 * 1024)
		minSec := dataMB * 0.01
		maxSec := dataMB * 0.05
		return DurationEstimate{
			MinSeconds: minSec,
			MaxSeconds: maxSec,
			Label:      formatDuration(minSec, maxSec, tm),
		}
	}

	// Rebuild: proportional to data + index length
	totalMB := float64(tm.DataLength+tm.IndexLength) / (1024 * 1024)
	minSec := totalMB * 0.05
	maxSec := totalMB * 0.2
	return DurationEstimate{
		MinSeconds: minSec,
		MaxSeconds: maxSec,
		Label:      formatDuration(minSec, maxSec, tm),
	}
}

func estimateCopy(tm *meta.TableMeta) DurationEstimate {
	totalMB := float64(tm.DataLength+tm.IndexLength) / (1024 * 1024)
	// COPY is slower than INPLACE
	minSec := totalMB * 0.1
	maxSec := totalMB * 0.4
	return DurationEstimate{
		MinSeconds: minSec,
		MaxSeconds: maxSec,
		Label:      formatDuration(minSec, maxSec, tm),
	}
}

func formatDuration(minSec, maxSec float64, tm *meta.TableMeta) string {
	sizeStr := formatSize(tm.DataLength + tm.IndexLength)
	if maxSec < 1 {
		return fmt.Sprintf("~0s (rows: ~%s, size: %s)", formatCount(tm.RowCount), sizeStr)
	}
	return fmt.Sprintf("~%ss - ~%ss (rows: ~%s, size: %s)",
		formatSeconds(minSec), formatSeconds(maxSec),
		formatCount(tm.RowCount), sizeStr)
}

func formatSeconds(sec float64) string {
	if sec < 60 {
		return fmt.Sprintf("%.0f", sec)
	}
	if sec < 3600 {
		return fmt.Sprintf("%.0fm", sec/60)
	}
	return fmt.Sprintf("%.1fh", sec/3600)
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
