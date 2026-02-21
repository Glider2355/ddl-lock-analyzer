package cmd

import (
	"database/sql"
	"fmt"

	_ "github.com/go-sql-driver/mysql"
	"github.com/spf13/cobra"

	"github.com/Glider2355/ddl-lock-analyzer/internal/fkresolver"
	"github.com/Glider2355/ddl-lock-analyzer/internal/meta"
	"github.com/Glider2355/ddl-lock-analyzer/internal/parser"
	"github.com/Glider2355/ddl-lock-analyzer/internal/predictor"
	"github.com/Glider2355/ddl-lock-analyzer/internal/reporter"
)

var (
	flagSQL      string
	flagHost     string
	flagPort     int
	flagUser     string
	flagPassword string
	flagDatabase string
	flagFormat   string
)

var analyzeCmd = &cobra.Command{
	Use:   "analyze",
	Short: "Analyze ALTER TABLE statements and predict lock impact",
	RunE:  runAnalyze,
}

func init() {
	f := analyzeCmd.Flags()
	f.StringVar(&flagSQL, "sql", "", "ALTER TABLE statement to analyze")
	f.StringVar(&flagHost, "host", "localhost", "MySQL host")
	f.IntVar(&flagPort, "port", 3306, "MySQL port")
	f.StringVar(&flagUser, "user", "", "MySQL user")
	f.StringVar(&flagPassword, "password", "", "MySQL password")
	f.StringVar(&flagDatabase, "database", "", "Database name")
	f.StringVar(&flagFormat, "format", "text", "Output format: text|json")
}

func runAnalyze(_ *cobra.Command, _ []string) error {
	// SQL入力を取得
	sqlText, err := getSQLInput()
	if err != nil {
		return err
	}

	// SQLをパース
	ops, err := parser.Parse(sqlText)
	if err != nil {
		return fmt.Errorf("parse error: %w", err)
	}

	// コレクターを初期化
	collector, err := initCollector()
	if err != nil {
		return err
	}

	// レポートを構築
	pred := predictor.New()
	report := &reporter.Report{}

	for _, op := range ops {
		tableName := op.Table
		if op.Schema != "" {
			tableName = op.Schema + "." + op.Table
		}

		// テーブルメタデータを取得
		schema := op.Schema
		if schema == "" {
			schema = flagDatabase
		}
		tableMeta, _ := collector.GetTableMeta(schema, op.Table)

		// ロック動作を予測
		predictions := pred.PredictAll(op, tableMeta)

		// FK依存関係を解決
		var fkGraph *fkresolver.FKGraph
		fkProvider := &collectorAdapter{collector: collector}
		resolver := fkresolver.NewResolver(fkProvider, 5, true)
		fkGraph, _ = resolver.Resolve(schema, op.Table, op.Actions)

		analysis := reporter.AnalysisResult{
			Table:       tableName,
			SQL:         op.RawSQL,
			Predictions: predictions,
			FKGraph:     fkGraph,
			TableMeta:   tableMeta,
		}
		report.Analyses = append(report.Analyses, analysis)
	}

	// 出力をレンダリング
	var rep reporter.Reporter
	switch flagFormat {
	case "json":
		rep = reporter.NewJSONReporter()
	default:
		rep = reporter.NewTextReporter()
	}

	output, err := rep.Render(report)
	if err != nil {
		return fmt.Errorf("render error: %w", err)
	}

	fmt.Println(output)
	return nil
}

func getSQLInput() (string, error) {
	if flagSQL != "" {
		return flagSQL, nil
	}
	return "", fmt.Errorf("--sql must be specified")
}

func initCollector() (meta.Collector, error) {
	if flagUser == "" || flagDatabase == "" {
		return nil, fmt.Errorf("--user and --database must be specified")
	}
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s", flagUser, flagPassword, flagHost, flagPort, flagDatabase)

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to MySQL: %w", err)
	}
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping MySQL: %w", err)
	}

	return meta.NewDBCollector(db, flagDatabase)
}

// collectorAdapter は meta.Collector を fkresolver.MetaProvider に適合させるアダプター。
type collectorAdapter struct {
	collector meta.Collector
}

func (a *collectorAdapter) GetTableMeta(schema, table string) (*meta.TableMeta, error) {
	return a.collector.GetTableMeta(schema, table)
}
