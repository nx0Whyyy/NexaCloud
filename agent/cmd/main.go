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
	fmt.Printf("\n============================================================\n")
	fmt.Printf(" INSTALLATION TERMINEE - VOICI LE CODE NEXAAGENT\n")
	fmt.Printf("\n CODE D'ACTIVATION : %s\n", authorization.UserCode)
	fmt.Printf(" DASHBOARD         : %s/dashboard/infrastructure\n", cfg.ControlPlaneURL)
	fmt.Printf(" EXPIRATION        : %d minutes\n", authorization.ExpiresIn/60)
	fmt.Printf("============================================================\n\n")
	fmt.Println("NexaAgent attend la validation du code...")
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
			fmt.Printf("\nNode active avec succes: %s\n", token.NodeID)
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
	go commandLoop(ctx, state, client, logger)
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

func commandLoop(ctx context.Context, state *config.State, client *api.Client, logger *slog.Logger) {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			command, err := client.NextCommand(ctx, state.AgentToken)
			if err != nil {
				logger.Warn("command poll failed", "error", err)
				continue
			}
			if command == nil {
				continue
			}
			result, executeErr := executeCommand(ctx, command, logger)
			errorMessage := ""
			if executeErr != nil {
				errorMessage = executeErr.Error()
			}
			if err := client.CompleteCommand(ctx, state.AgentToken, command.ID.String(), executeErr == nil, result, errorMessage); err != nil {
				logger.Error("command result failed", "command_id", command.ID, "error", err)
			}
		}
	}
}

func executeCommand(ctx context.Context, command *model.AgentCommand, logger *slog.Logger) (string, error) {
	manager := docker.New(logger)
	container := command.Params["container"]
	if container == "" && command.InstanceID != nil {
		container = "nexacloud-server-" + command.InstanceID.String()[:8]
	}
	switch command.Command {
	case "CREATE_SERVER":
		return manager.CreateMinecraft(ctx, command.Params["name"], mustPort(command.Params["port"]), command.Params["memory"])
	case "START_SERVER":
		return "", manager.Start(ctx, container)
	case "STOP_SERVER":
		return "", manager.Stop(ctx, container)
	case "RESTART_SERVER":
		return "", manager.Restart(ctx, container)
	case "KILL_SERVER":
		return "", manager.Kill(ctx, container)
	case "DELETE_SERVER":
		return "", manager.Remove(ctx, container)
	case "SERVER_LOGS":
		return manager.Logs(ctx, container, 300)
	case "CONSOLE":
		return manager.Console(ctx, container, command.Params["command"])
	case "REPAIR_NEXALINK":
		return manager.RepairNexaLink(ctx, container)
	case "FILE_LIST":
		return manager.ListFiles(ctx, container, command.Params["path"])
	case "FILE_READ":
		return manager.ReadFile(ctx, container, command.Params["path"])
	case "FILE_WRITE":
		return manager.WriteFile(ctx, container, command.Params["path"], command.Params["content"])
	case "FILE_MKDIR":
		return manager.MakeDirectory(ctx, container, command.Params["path"])
	case "FILE_DELETE":
		return manager.DeleteFile(ctx, container, command.Params["path"])
	case "FILE_MOVE":
		return manager.MoveFile(ctx, container, command.Params["path"], command.Params["content"])
	case "ENABLE_SFTP":
		return manager.EnableSFTP(ctx, container, mustPort(command.Params["port"]), command.Params["public_key"])
	case "DISABLE_SFTP":
		return manager.DisableSFTP(ctx, container)
	default:
		return "", fmt.Errorf("unsupported command %s", command.Command)
	}
}

func mustPort(value string) int { port, _ := strconv.Atoi(value); return port }

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
