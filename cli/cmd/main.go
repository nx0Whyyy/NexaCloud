package main

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/nexastudio/nexacloud/cli/internal/config"
	"github.com/spf13/cobra"
)

var (
	version = "dev"
	cfg     *config.Config
	logger  *slog.Logger
)

func main() {
	logger = slog.New(slog.NewTextHandler(os.Stderr, nil))

	var err error
	cfg, err = config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "config error: %v\n", err)
		os.Exit(1)
	}

	rootCmd := &cobra.Command{
		Use:     "nexa",
		Short:   "NexaCloud - Minecraft infrastructure on autopilot",
		Version: version,
	}

	rootCmd.AddCommand(statusCmd(cfg, logger))
	rootCmd.AddCommand(serviceCmd(cfg, logger))
	rootCmd.AddCommand(deployCmd(cfg, logger))
	rootCmd.AddCommand(nodeCmd(cfg, logger))
	rootCmd.AddCommand(backupCmd(cfg, logger))
	rootCmd.AddCommand(instanceCmd(cfg, logger))
	rootCmd.AddCommand(playerCmd(cfg, logger))

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
