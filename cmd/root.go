package cmd

import (
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "ddl-lock-analyzer",
	Short: "Predict MySQL ALTER TABLE lock impact",
	Long:  "ddl-lock-analyzer analyzes ALTER TABLE statements and predicts ALGORITHM, LOCK level, table rebuild, FK propagation, and estimated duration.",
}

// Execute はルートコマンドを実行する。
func Execute() error {
	return rootCmd.Execute()
}

func init() {
	rootCmd.AddCommand(analyzeCmd)
	rootCmd.AddCommand(versionCmd)
}
