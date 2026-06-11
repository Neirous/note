package main

import (
	"github.com/spf13/cobra"

	"note/internal/gateway"
)

func serverCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "server",
		Short: "Start the HTTP API server",
		Run: func(cmd *cobra.Command, args []string) {
			addr, _ := cmd.Flags().GetString("addr")
			dsn, _ := cmd.Flags().GetString("dsn")
			provider, _ := cmd.Flags().GetString("llm-provider")
			gateway.RunServer(addr, dsn, provider)
		},
	}

	cmd.Flags().String("addr", "", "listen address (env: APP_ADDR, default :8080)")
	cmd.Flags().String("dsn", "", "SQLite DSN (env: APP_DSN)")
	cmd.Flags().String("llm-provider", "", "LLM provider: dashscope or ollama (env: LLM_PROVIDER)")

	return cmd
}
