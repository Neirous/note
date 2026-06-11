package main

import (
	"fmt"
	"os"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"note/internal/e2e"
)

func e2eCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "e2e",
		Short: "Run RAG end-to-end check (requires OPENAI_API_KEY)",
		Run: func(cmd *cobra.Command, args []string) {
			dsn, _ := cmd.Flags().GetString("dsn")
			if dsn == "" {
				dsn = "file:e2e-check.db?_pragma=busy_timeout(5000)"
			}
			cleanup, _ := cmd.Flags().GetBool("cleanup")

			results, err := e2e.Run(dsn, cleanup)
			if err != nil {
				log.Fatal(err)
			}

			printResults(results)
			if hasFailure(results) {
				os.Exit(1)
			}
		},
	}
	cmd.Flags().String("dsn", "", "SQLite DSN (env: E2E_DSN)")
	cmd.Flags().Bool("cleanup", true, "remove test database after run")
	return cmd
}

func printResults(results []e2e.CheckResult) {
	fmt.Println("E2E CHECK RESULTS")
	for _, r := range results {
		status := "PASS"
		if !r.OK {
			status = "FAIL"
		}
		fmt.Printf("- [%s] %s -> %s\n", status, r.Step, r.Msg)
	}
}

func hasFailure(results []e2e.CheckResult) bool {
	for _, r := range results {
		if !r.OK {
			return true
		}
	}
	return false
}
