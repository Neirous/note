package main

import (
	"github.com/spf13/cobra"
)

func seedCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "seed",
		Short: "Generate demo notes with RAG indexing",
		RunE: func(cmd *cobra.Command, args []string) error {
			// TODO: Extract seed logic from cmd/seednotes/main.go
			return nil
		},
	}
}
