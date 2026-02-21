package main

import (
	"os"

	"github.com/muramatsuryo/ddl-lock-analyzer/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
