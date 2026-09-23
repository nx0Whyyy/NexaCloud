package main

import (
	"fmt"
	"log/slog"

	"github.com/nexastudio/nexacloud/cli/internal/api"
	"github.com/nexastudio/nexacloud/cli/internal/config"
	"github.com/spf13/cobra"
)

func statusCmd(cfg *config.Config, logger *slog.Logger) *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show network status",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Println("NexaCloud - Network Status")
			fmt.Println("=========================")
			client := api.New(nil, cfg.APIUrl)

			var result map[string]any
			if err := client.Get("/api/v1/status", &result); err != nil {
				fmt.Printf("API unavailable: %v\n", err)
				fmt.Println("  (Is the control plane running?)")
				return nil
			}

			network, _ := result["network"].(string)
			nodes, _ := result["nodes"].(string)
			instances, _ := result["instances"].(string)
			players, _ := result["players"].(float64)
			status, _ := result["status"].(string)

			fmt.Printf("  Network:   %s\n", network)
			fmt.Printf("  Nodes:     %s\n", nodes)
			fmt.Printf("  Instances: %s\n", instances)
			fmt.Printf("  Players:   %.0f\n", players)
			fmt.Printf("  Status:    %s\n", status)
			return nil
		},
	}
}

func serviceCmd(cfg *config.Config, logger *slog.Logger) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "service",
		Short: "Manage services",
	}

	scaleCmd := &cobra.Command{
		Use:   "scale <name> <count>",
		Short: "Scale a service to N instances",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			count, err := parseCount(args[1])
			if err != nil {
				return err
			}
			fmt.Printf("Scaling service '%s' to %d instances...\n", name, count)
			client := api.New(nil, cfg.APIUrl)
			var result map[string]any
			err = client.Post("/api/v1/services/"+name+"/scale",
				map[string]any{"count": count}, &result)
			if err != nil {
				fmt.Printf("Error: %v\n", err)
				return nil
			}
			fmt.Printf("  Status: %v\n", result["status"])
			return nil
		},
	}
	cmd.AddCommand(scaleCmd)
	return cmd
}

func deployCmd(cfg *config.Config, logger *slog.Logger) *cobra.Command {
	var serviceFlag string
	var strategy string

	cmd := &cobra.Command{
		Use:   "deploy <file>",
		Short: "Deploy a plugin or update",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Printf("Deploying %s...\n", args[0])
			if serviceFlag != "" {
				fmt.Printf("  Service:  %s\n", serviceFlag)
			}
			fmt.Printf("  Strategy: %s\n", strategy)
			client := api.New(nil, cfg.APIUrl)
			var result map[string]any
			err := client.Post("/api/v1/deployments",
				map[string]any{"file": args[0], "service": serviceFlag, "strategy": strategy}, &result)
			if err != nil {
				fmt.Printf("Error: %v\n", err)
				return nil
			}
			fmt.Printf("  Status: %v\n", result["status"])
			return nil
		},
	}
	cmd.Flags().StringVarP(&serviceFlag, "service", "s", "", "target service")
	cmd.Flags().StringVarP(&strategy, "strategy", "t", "rolling", "deployment strategy")
	return cmd
}

func nodeCmd(cfg *config.Config, logger *slog.Logger) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "node",
		Short: "Manage nodes",
	}

	drainCmd := &cobra.Command{
		Use:   "drain <name>",
		Short: "Drain a node (evacuate instances, prepare for shutdown)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Printf("Draining node '%s'...\n", args[0])
			client := api.New(nil, cfg.APIUrl)
			var result map[string]any
			err := client.Post("/api/v1/nodes/"+args[0]+"/drain", map[string]any{}, &result)
			if err != nil {
				fmt.Printf("Error: %v\n", err)
				return nil
			}
			fmt.Printf("  Status: %v\n", result["status"])
			return nil
		},
	}
	cmd.AddCommand(drainCmd)
	return cmd
}

func backupCmd(cfg *config.Config, logger *slog.Logger) *cobra.Command {
	return &cobra.Command{
		Use:   "backup",
		Short: "Trigger a network backup",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Println("Triggering network backup...")
			client := api.New(nil, cfg.APIUrl)
			var result map[string]any
			err := client.Post("/api/v1/backups", map[string]any{}, &result)
			if err != nil {
				fmt.Printf("Error: %v\n", err)
				return nil
			}
			fmt.Printf("  Status: %v\n", result["status"])
			return nil
		},
	}
}

func instanceCmd(cfg *config.Config, logger *slog.Logger) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "instance",
		Short: "Manage instances",
	}

	restartCmd := &cobra.Command{
		Use:   "restart <name>",
		Short: "Restart an instance",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Printf("Restarting instance '%s'...\n", args[0])
			client := api.New(nil, cfg.APIUrl)
			var result map[string]any
			err := client.Post("/api/v1/instances/"+args[0]+"/restart", map[string]any{}, &result)
			if err != nil {
				fmt.Printf("Error: %v\n", err)
				return nil
			}
			fmt.Printf("  Status: %v\n", result["status"])
			return nil
		},
	}
	cmd.AddCommand(restartCmd)
	return cmd
}

func playerCmd(cfg *config.Config, logger *slog.Logger) *cobra.Command {
	return &cobra.Command{
		Use:   "player <name>",
		Short: "Look up a player",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Printf("Looking up player '%s'...\n", args[0])
			client := api.New(nil, cfg.APIUrl)
			var result map[string]any
			err := client.Get("/api/v1/players/"+args[0], &result)
			if err != nil {
				fmt.Printf("Error: %v\n", err)
				return nil
			}
			fmt.Printf("  Server:  %v\n", result["server"])
			fmt.Printf("  Status:  %v\n", result["status"])
			return nil
		},
	}
}

func parseCount(s string) (int, error) {
	var n int
	_, err := fmt.Sscanf(s, "%d", &n)
	return n, err
}
