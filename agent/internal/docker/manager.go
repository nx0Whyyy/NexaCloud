package docker

import (
	"context"
	"fmt"
	"log/slog"
	"os/exec"
	"strings"
)

type Manager struct {
	logger *slog.Logger
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
