package main

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/nexastudio/nexacloud/agent/internal/api"
	"github.com/nexastudio/nexacloud/agent/internal/config"
	"github.com/nexastudio/nexacloud/agent/internal/docker"
	"github.com/nexastudio/nexacloud/agent/internal/nexalink"
	"github.com/nexastudio/nexacloud/pkg/model"
)

var version = "dev"

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	cfg, err := config.Load()
	if err != nil {
		logger.Error("config error", "error", err)
		os.Exit(1)
	}
	command := "run"
	if len(os.Args) > 1 {
		command = os.Args[1]
	}
	client := api.New(cfg.ControlPlaneURL)
	switch command {
	case "register":
		err = register(context.Background(), cfg, client)
	case "run":
		err = run(cfg, client, logger)
	case "check":
		err = check(context.Background(), cfg, client, logger)
	case "minecraft":
		err = minecraftCommand(context.Background(), os.Args[2:], logger)
	default:
		err = fmt.Errorf("unknown command %q (use register, run or check)", command)
	}
	if err != nil {
		logger.Error("agent stopped", "error", err)
		os.Exit(1)
	}
}

func minecraftCommand(ctx context.Context, args []string, logger *slog.Logger) error {
	if len(args) < 2 {
		return fmt.Errorf("usage: nexa-agent minecraft <create|start|stop|remove> <name> [port] [memory]")
	}
	action, name := args[0], args[1]
	for _, char := range name {
		if !((char >= 'a' && char <= 'z') || (char >= '0' && char <= '9') || char == '-') {
			return fmt.Errorf("name must contain lowercase letters, digits or hyphens")
		}
	}
	manager := docker.New(logger)
	containerName := "nexacloud-" + name
	switch action {
	case "create":
		port, memory := 25565, "2G"
		if len(args) > 2 {
			parsed, err := strconv.Atoi(args[2])
			if err != nil || parsed < 1024 || parsed > 65535 {
				return fmt.Errorf("port must be between 1024 and 65535")
			}
			port = parsed
		}
		if len(args) > 3 {
			memory = args[3]
		}
		id, err := manager.CreateMinecraft(ctx, name, port, memory)
		if err != nil {
			return err
		}
		fmt.Printf("Serveur Minecraft créé: %s (%s), port %d\n", containerName, id[:12], port)
		return nil
	case "start":
		return manager.Start(ctx, containerName)
	case "stop":
		return manager.Stop(ctx, containerName)
	case "remove":
		return manager.Remove(ctx, containerName)
	default:
		return fmt.Errorf("unknown minecraft action %q", action)
	}
}

func register(ctx context.Context, cfg *config.Config, client *api.Client) error {
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return err
	}
	authorization, err := client.CreateDeviceCode(ctx, cfg.NodeName, base64.RawURLEncoding.EncodeToString(key), hostResources())
	if err != nil {
		return err
	}
	fmt.Printf("\nNexaAgent attend votre autorisation.\n\n  Code: %s\n  Ouvrez: %s/dashboard/infrastructure\n\n", authorization.UserCode, cfg.ControlPlaneURL)
	deadline := time.Now().Add(time.Duration(authorization.ExpiresIn) * time.Second)
	for time.Now().Before(deadline) {
		token, status, err := client.ClaimDevice(ctx, authorization.DeviceCode)
		if err == nil && status == 200 && token.Status == "approved" {
			if token.AgentToken == "" {
				return fmt.Errorf("control plane returned an empty agent token")
			}
			if err := config.SaveState(cfg.StatePath, config.State{NodeID: token.NodeID, AgentToken: token.AgentToken, Fingerprint: token.Fingerprint}); err != nil {
				return err
			}
			fmt.Printf("Node autorisé: %s\nConfiguration enregistrée dans %s\n", token.NodeID, cfg.StatePath)
			return nil
		}
		if err != nil && status != 202 {
			return err
		}
		time.Sleep(5 * time.Second)
	}
	return fmt.Errorf("device authorization expired")
}

func run(cfg *config.Config, client *api.Client, logger *slog.Logger) error {
	state, err := config.LoadState(cfg.StatePath)
	if err != nil {
		return fmt.Errorf("load state (run nexa-agent register first): %w", err)
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	logger.Info("nexa-agent started", "version", version, "node_id", state.NodeID)
	if err := sendHeartbeat(ctx, cfg, state, client, logger); err != nil {
		logger.Warn("initial heartbeat failed", "error", err)
	}
	ticker := time.NewTicker(cfg.Interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			if err := sendHeartbeat(ctx, cfg, state, client, logger); err != nil {
				logger.Warn("heartbeat failed", "error", err)
			}
		}
	}
}

func check(ctx context.Context, cfg *config.Config, client *api.Client, logger *slog.Logger) error {
	state, err := config.LoadState(cfg.StatePath)
	if err != nil {
		return err
	}
	if err := sendHeartbeat(ctx, cfg, state, client, logger); err != nil {
		return err
	}
	fmt.Println("NexaAgent, Docker et control plane: opérationnels")
	return nil
}

func sendHeartbeat(ctx context.Context, cfg *config.Config, state *config.State, client *api.Client, logger *slog.Logger) error {
	dockerManager := docker.New(logger)
	containers, err := dockerManager.List(ctx)
	if err != nil {
		return fmt.Errorf("docker inventory: %w", err)
	}
	minecraft, err := nexalink.Status(ctx, cfg.MinecraftAddress)
	if err != nil {
		logger.Debug("minecraft endpoint unavailable", "address", cfg.MinecraftAddress, "error", err)
		minecraft = nil
	}
	heartbeat := model.AgentHeartbeat{AgentVersion: version, Resources: hostResources(), Containers: containers, Minecraft: minecraft}
	response, err := client.Heartbeat(ctx, state.AgentToken, heartbeat)
	if err != nil {
		return err
	}
	logger.Info("heartbeat accepted", "node_id", response.NodeID, "containers", len(containers), "minecraft", minecraft != nil)
	return nil
}

func hostResources() model.Resources {
	resources := model.Resources{CPU: runtime.NumCPU()}
	if data, err := os.ReadFile("/proc/meminfo"); err == nil {
		for _, line := range strings.Split(string(data), "\n") {
			if strings.HasPrefix(line, "MemTotal:") {
				fields := strings.Fields(line)
				if len(fields) >= 2 {
					if kb, err := strconv.ParseInt(fields[1], 10, 64); err == nil {
						resources.Memory = fmt.Sprintf("%dMiB", kb/1024)
					}
				}
				break
			}
		}
	}
	return resources
}
