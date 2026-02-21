package cmd

import (
	"database/sql"
	"fmt"
	"os"

	_ "github.com/go-sql-driver/mysql"
	"github.com/spf13/cobra"

	"github.com/muramatsuryo/ddl-lock-analyzer/internal/fkresolver"
	"github.com/muramatsuryo/ddl-lock-analyzer/internal/meta"
	"github.com/muramatsuryo/ddl-lock-analyzer/internal/parser"
	"github.com/muramatsuryo/ddl-lock-analyzer/internal/predictor"
	"github.com/muramatsuryo/ddl-lock-analyzer/internal/reporter"
)

var (
	flagSQL          string
	flagFile         string
	flagDSN          string
	flagHost         string
	flagPort         int
	flagUser         string
	flagPassword     string
	flagDatabase     string
	flagMySQLVersion string
	flagFormat       string
	flagFKChecks     bool
	flagFKDepth      int
	flagOffline      bool
	flagMetaFile     string
)

var analyzeCmd = &cobra.Command{
	Use:   "analyze",
	Short: "Analyze ALTER TABLE statements and predict lock impact",
	RunE:  runAnalyze,
}

func init() {
	f := analyzeCmd.Flags()
	f.StringVar(&flagSQL, "sql", "", "ALTER TABLE statement to analyze")
	f.StringVar(&flagFile, "file", "", "SQL file path to analyze")
	f.StringVar(&flagDSN, "dsn", "", "MySQL DSN (user:pass@tcp(host:port)/dbname)")
	f.StringVar(&flagHost, "host", "localhost", "MySQL host")
	f.IntVar(&flagPort, "port", 3306, "MySQL port")
	f.StringVar(&flagUser, "user", "", "MySQL user")
	f.StringVar(&flagPassword, "password", "", "MySQL password")
	f.StringVar(&flagDatabase, "database", "", "Database name")
	f.StringVar(&flagMySQLVersion, "mysql-version", "8.0", "MySQL version (for offline mode)")
	f.StringVar(&flagFormat, "format", "text", "Output format: text|json")
	f.BoolVar(&flagFKChecks, "fk-checks", true, "Assume foreign_key_checks is ON")
	f.IntVar(&flagFKDepth, "fk-depth", 5, "Maximum FK dependency graph depth")
	f.BoolVar(&flagOffline, "offline", false, "Offline mode (no DB connection)")
	f.StringVar(&flagMetaFile, "meta-file", "", "Metadata JSON file path (for offline mode)")
}

func runAnalyze(_ *cobra.Command, _ []string) error {
	// Get SQL input
	sqlText, err := getSQLInput()
	if err != nil {
		return err
	}

	// Parse SQL
	ops, err := parser.Parse(sqlText)
	if err != nil {
		return fmt.Errorf("parse error: %w", err)
	}

	// Initialize collector
	collector, err := initCollector()
	if err != nil {
		return err
	}

	// Close DB connection if applicable
	if dbCollector, ok := collector.(*meta.DBCollector); ok {
		_ = dbCollector
	}

	// Build report
	pred := predictor.New()
	report := &reporter.Report{}

	for _, op := range ops {
		tableName := op.Table
		if op.Schema != "" {
			tableName = op.Schema + "." + op.Table
		}

		// Get table metadata
		schema := op.Schema
		if schema == "" {
			schema = flagDatabase
		}
		tableMeta, _ := collector.GetTableMeta(schema, op.Table)

		// Predict lock behavior
		predictions := pred.PredictAll(op, tableMeta)

		// Resolve FK dependencies
		var fkGraph *fkresolver.FKGraph
		fkProvider := &collectorAdapter{collector: collector}
		resolver := fkresolver.NewResolver(fkProvider, flagFKDepth, flagFKChecks)
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

	// Render output
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
	if flagFile != "" {
		data, err := os.ReadFile(flagFile) //#nosec G304 -- user-provided file path is intentional
		if err != nil {
			return "", fmt.Errorf("failed to read SQL file: %w", err)
		}
		return string(data), nil
	}
	return "", fmt.Errorf("either --sql or --file must be specified")
}

func initCollector() (meta.Collector, error) {
	if flagOffline {
		if flagMetaFile != "" {
			return meta.NewFileCollector(flagMetaFile, flagMySQLVersion)
		}
		return meta.NewOfflineCollector(flagMySQLVersion), nil
	}

	dsn := flagDSN
	if dsn == "" {
		if flagUser == "" || flagDatabase == "" {
			return nil, fmt.Errorf("either --dsn or (--user, --database) must be specified, or use --offline")
		}
		dsn = fmt.Sprintf("%s:%s@tcp(%s:%d)/%s", flagUser, flagPassword, flagHost, flagPort, flagDatabase)
	}

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to MySQL: %w", err)
	}
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping MySQL: %w", err)
	}

	database := flagDatabase
	if database == "" {
		// Extract database from DSN
		if err := db.QueryRow("SELECT DATABASE()").Scan(&database); err != nil {
			return nil, fmt.Errorf("failed to get current database: %w", err)
		}
	}

	return meta.NewDBCollector(db, database)
}

// collectorAdapter adapts meta.Collector to fkresolver.MetaProvider.
type collectorAdapter struct {
	collector meta.Collector
}

func (a *collectorAdapter) GetTableMeta(schema, table string) (*meta.TableMeta, error) {
	return a.collector.GetTableMeta(schema, table)
}
