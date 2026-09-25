package docker

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/nexastudio/nexacloud/agent/internal/nexalinkplugin"
	"github.com/nexastudio/nexacloud/pkg/model"
)

type Manager struct {
	logger *slog.Logger
}

func (m *Manager) Restart(ctx context.Context, containerID string) error {
	_, err := docker(ctx, "restart", containerID)
	return err
}
func (m *Manager) Kill(ctx context.Context, containerID string) error {
	_, err := docker(ctx, "kill", containerID)
	return err
}
func (m *Manager) Logs(ctx context.Context, containerID string, lines int) (string, error) {
	if lines < 1 || lines > 1000 {
		lines = 200
	}
	return docker(ctx, "logs", "--tail", fmt.Sprint(lines), containerID)
}
func (m *Manager) Console(ctx context.Context, containerID, command string) (string, error) {
	if err := m.waitForRCON(ctx, containerID, 12*time.Second); err != nil {
		return "", err
	}
	return docker(ctx, "exec", containerID, "rcon-cli", command)
}
func (m *Manager) ListFiles(ctx context.Context, containerID, relative string) (string, error) {
	target, err := dataPath(ctx, containerID, relative)
	if err != nil {
		return "", err
	}
	out, err := docker(ctx, "exec", containerID, "find", target, "-maxdepth", "1", "-printf", "%f\t%y\n")
	return boundedOutput(out, err)
}
func (m *Manager) ReadFile(ctx context.Context, containerID, relative string) (string, error) {
	target, err := dataPath(ctx, containerID, relative)
	if err != nil {
		return "", err
	}
	out, err := docker(ctx, "exec", containerID, "head", "-c", "262145", target)
	return boundedOutput(out, err)
}
func (m *Manager) WriteFile(ctx context.Context, containerID, relative, content string) (string, error) {
	target, err := dataPath(ctx, containerID, relative)
	if err != nil {
		return "", err
	}
	if target == "/data" {
		return "", fmt.Errorf("un fichier doit être sélectionné")
	}
	cmd := exec.CommandContext(ctx, "docker", "exec", "-i", containerID, "tee", target)
	cmd.Stdin = strings.NewReader(content)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("write file: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return string(out), nil
}

const maxFileResult = 256 * 1024

func dataPath(ctx context.Context, containerID, relative string) (string, error) {
	target, err := cleanDataTarget(relative)
	if err != nil {
		return "", err
	}
	resolved, err := docker(ctx, "exec", containerID, "realpath", "-m", "--", target)
	if err != nil {
		return "", fmt.Errorf("résolution du chemin: %w", err)
	}
	resolved = strings.TrimSpace(resolved)
	if resolved != "/data" && !strings.HasPrefix(resolved, "/data/") {
		return "", fmt.Errorf("chemin hors du volume serveur")
	}
	return resolved, nil
}

func cleanDataTarget(relative string) (string, error) {
	relative = strings.TrimSpace(relative)
	if len(relative) > 4096 || strings.ContainsAny(relative, "\x00\\") || path.IsAbs(relative) {
		return "", fmt.Errorf("chemin de fichier invalide")
	}
	for _, segment := range strings.Split(relative, "/") {
		if segment == ".." {
			return "", fmt.Errorf("chemin de fichier invalide")
		}
	}
	clean := path.Clean(relative)
	if clean == "." || clean == "" {
		return "/data", nil
	}
	return "/data/" + clean, nil
}

func boundedOutput(output string, err error) (string, error) {
	if err != nil {
		return "", err
	}
	if len(output) > maxFileResult {
		return "", fmt.Errorf("résultat supérieur à 256 Kio")
	}
	return output, nil
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
	if _, err := docker(ctx, "volume", "create", volumeName); err != nil {
		return "", err
	}
	args := []string{"create", "--name", containerName, "--restart", "unless-stopped", "-p", fmt.Sprintf("%d:25565", port), "-e", "EULA=TRUE", "-e", "TYPE=PAPER", "-e", "MEMORY=" + memory, "-e", "ENABLE_RCON=true", "-v", volumeName + ":/data", "itzg/minecraft-server:java25"}
	out, err := docker(ctx, args...)
	if err != nil {
		return "", fmt.Errorf("minecraft container: %w", err)
	}
	lines := strings.Fields(out)
	if len(lines) == 0 {
		return "", fmt.Errorf("docker returned an empty container id")
	}
	containerID := lines[len(lines)-1]
	if err := installNexaLink(ctx, containerID); err != nil {
		_ = m.Remove(context.Background(), containerID)
		return "", err
	}
	if _, err := docker(ctx, "start", containerID); err != nil {
		return "", err
	}
	if err := m.waitForNexaLink(ctx, containerID, 2*time.Minute); err != nil {
		return "", err
	}
	return containerID, nil
}

func installNexaLink(ctx context.Context, containerID string) error {
	root, err := os.MkdirTemp("", "nexalink-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(root)
	plugins := filepath.Join(root, "plugins")
	if err := os.Mkdir(plugins, 0755); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(plugins, "NexaLink.jar"), nexalinkplugin.JAR, 0644); err != nil {
		return err
	}
	if _, err := docker(ctx, "cp", plugins, containerID+":/data/"); err != nil {
		return fmt.Errorf("install NexaLink: %w", err)
	}
	if _, err := docker(ctx, "run", "--rm", "--volumes-from", containerID, "alpine:3.20", "chown", "-R", "1000:1000", "/data/plugins"); err != nil {
		return fmt.Errorf("prepare NexaLink data directory: %w", err)
	}
	return nil
}

func (m *Manager) waitForRCON(ctx context.Context, containerID string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if _, err := docker(ctx, "exec", containerID, "rcon-cli", "list"); err == nil {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(2 * time.Second):
		}
	}
	return fmt.Errorf("console RCON indisponible")
}

func (m *Manager) waitForNexaLink(ctx context.Context, containerID string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		out, err := docker(ctx, "exec", containerID, "rcon-cli", "nexalink")
		if err == nil && strings.Contains(out, "NexaLink ONLINE") {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(3 * time.Second):
		}
	}
	logs, _ := m.Logs(context.Background(), containerID, 80)
	return fmt.Errorf("NexaLink n'a pas validé son démarrage: %s", strings.TrimSpace(logs))
}

func (m *Manager) RepairNexaLink(ctx context.Context, containerID string) (string, error) {
	_ = m.Stop(ctx, containerID)
	if err := installNexaLink(ctx, containerID); err != nil {
		return "", err
	}
	if err := m.Start(ctx, containerID); err != nil {
		return "", err
	}
	if err := m.waitForNexaLink(ctx, containerID, 2*time.Minute); err != nil {
		return "", err
	}
	return "NexaLink ONLINE", nil
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
