package main

import (
	"github.com/spf13/cobra"

	grpcserver "note/internal/grpc"
)

func grpcCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "grpc",
		Short: "Start the gRPC API server",
		Run: func(cmd *cobra.Command, args []string) {
			addr, _ := cmd.Flags().GetString("addr")
			dsn, _ := cmd.Flags().GetString("dsn")
			provider, _ := cmd.Flags().GetString("llm-provider")
			grpcserver.Run(addr, dsn, provider)
		},
	}
	cmd.Flags().String("addr", ":9090", "listen address")
	cmd.Flags().String("dsn", "", "SQLite DSN (env: APP_DSN)")
	cmd.Flags().String("llm-provider", "", "LLM provider: dashscope or ollama")
	return cmd
}
