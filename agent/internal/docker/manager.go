package docker

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os/exec"
	"strings"

	"github.com/nexastudio/nexacloud/pkg/model"
)

type Manager struct {
	logger *slog.Logger
}

func (m *Manager) List(ctx context.Context) ([]model.ContainerInfo, error) {
	out, err := docker(ctx, "ps", "-a", "--format", "{{json .}}")
	if err != nil {
		return nil, err
	}
	var containers []model.ContainerInfo
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		if line == "" {
			continue
		}
		var row struct {
			ID    string
			Names string
			Image string
			State string
		}
		if err := json.Unmarshal([]byte(line), &row); err != nil {
			return nil, err
		}
		containers = append(containers, model.ContainerInfo{ID: row.ID, Name: row.Names, Image: row.Image, State: row.State})
	}
	return containers, nil
}

func New(logger *slog.Logger) *Manager {
	return &Manager{logger: logger}
}

type CreateOptions struct {
	Image     string
	Name      string
	Env       []string
	Resources Resources
}

type Resources struct {
	Memory string
	CPU    int
}

func (m *Manager) Create(ctx context.Context, opts CreateOptions) (string, error) {
	args := []string{"create", "--name", opts.Name}
	if opts.Resources.Memory != "" {
		args = append(args, "--memory", opts.Resources.Memory)
	}
	if opts.Resources.CPU > 0 {
		args = append(args, "--cpus", fmt.Sprintf("%d", opts.Resources.CPU))
	}
	for _, env := range opts.Env {
		args = append(args, "-e", env)
	}
	args = append(args, opts.Image)

	out, err := docker(ctx, args...)
	if err != nil {
		return "", fmt.Errorf("container create: %w", err)
	}
	return strings.TrimSpace(out), nil
}

func (m *Manager) CreateMinecraft(ctx context.Context, name string, port int, memory string) (string, error) {
	containerName := "nexacloud-" + name
	volumeName := containerName + "-data"
	args := []string{"run", "-d", "--name", containerName, "--restart", "unless-stopped", "-p", fmt.Sprintf("%d:25565", port), "-e", "EULA=TRUE", "-e", "TYPE=PAPER", "-e", "MEMORY=" + memory, "-v", volumeName + ":/data", "itzg/minecraft-server:java25"}
	out, err := docker(ctx, args...)
	if err != nil {
		return "", fmt.Errorf("minecraft container: %w", err)
	}
	lines := strings.Fields(out)
	if len(lines) == 0 {
		return "", fmt.Errorf("docker returned an empty container id")
	}
	return lines[len(lines)-1], nil
}

func (m *Manager) Start(ctx context.Context, containerID string) error {
	m.logger.Info("starting container", "id", containerID)
	_, err := docker(ctx, "start", containerID)
	return err
}

func (m *Manager) Stop(ctx context.Context, containerID string) error {
	m.logger.Info("stopping container", "id", containerID)
	_, err := docker(ctx, "stop", containerID)
	return err
}

func (m *Manager) Remove(ctx context.Context, containerID string) error {
	_, err := docker(ctx, "rm", "-f", containerID)
	return err
}

func (m *Manager) Stats(ctx context.Context) (map[string]float64, error) {
	return map[string]float64{}, nil
}

func docker(ctx context.Context, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "docker", args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("docker %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(string(out)))
	}
	return string(out), nil
}
