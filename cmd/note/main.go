package main

import (
	"os"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var logLevel string

func main() {
	root := &cobra.Command{
		Use:   "note",
		Short: "RAG Note - Markdown note-taking app with AI",
	}
	root.PersistentFlags().StringVar(&logLevel, "log-level", "info", "log level {debug, info, warn, error}")

	root.AddCommand(serverCmd())
	root.AddCommand(grpcCmd())
	root.AddCommand(seedCmd())
	root.AddCommand(e2eCmd())

	cobra.OnInitialize(initLogger)
	if err := root.Execute(); err != nil {
		os.Exit(1)
	}
}

func initLogger() {
	level, err := log.ParseLevel(logLevel)
	if err != nil {
		log.Fatal(err)
	}
	log.SetLevel(level)
	log.SetFormatter(&log.TextFormatter{FullTimestamp: true})
}
