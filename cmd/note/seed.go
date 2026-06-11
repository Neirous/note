package main

import (
	"context"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"note/internal/seed"
)

func seedCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "seed",
		Short: "Generate demo notes with RAG indexing",
		Run: func(cmd *cobra.Command, args []string) {
			dsn, _ := cmd.Flags().GetString("dsn")
			if dsn == "" {
				dsn = "file:notes.db?_pragma=busy_timeout(5000)"
			}
			if err := seed.Run(context.Background(), dsn); err != nil {
				log.Fatal(err)
			}
		},
	}
	cmd.Flags().String("dsn", "", "SQLite DSN (env: APP_DSN)")
	return cmd
}
