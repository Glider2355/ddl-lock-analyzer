package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

// Version はビルド時に ldflags で設定される。
var Version = "dev"

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version information",
	Run: func(_ *cobra.Command, _ []string) {
		fmt.Printf("ddl-lock-analyzer version %s\n", Version)
	},
}
