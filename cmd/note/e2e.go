package main

import (
	"github.com/spf13/cobra"
)

func e2eCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "e2e",
		Short: "Run RAG end-to-end check",
		RunE: func(cmd *cobra.Command, args []string) error {
			// TODO: Extract e2e check logic from cmd/e2echeck/main.go
			return nil
		},
	}
}
