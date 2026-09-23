package command

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/nats-io/nats.go"
	"github.com/nexastudio/nexacloud/agent/internal/docker"
	"github.com/nexastudio/nexacloud/pkg/model"
	"gorm.io/gorm"
)

type Handler struct {
	db        *gorm.DB
	dockerMgr *docker.Manager
	logger    *slog.Logger
}

func New(db *gorm.DB, dm *docker.Manager, logger *slog.Logger) *Handler {
	return &Handler{db: db, dockerMgr: dm, logger: logger}
}

func (h *Handler) Handle(msg *nats.Msg) {
	var req model.CommandRequest
	if err := json.Unmarshal(msg.Data, &req); err != nil {
		h.logger.Error("invalid command", "error", err)
		h.respond(msg, req.ID, false, "invalid command: "+err.Error(), "")
		return
	}

	h.logger.Info("received command", "command", req.Command, "instance", req.InstanceID)

	var result string
	var err error
	ctx := context.Background()

	switch req.Command {
	case "START_INSTANCE":
		err = h.startInstance(ctx, &req)
	case "STOP_INSTANCE":
		err = h.stopInstance(ctx, &req)
	case "CREATE_INSTANCE":
		err = h.createInstance(ctx, &req)
	case "DELETE_INSTANCE":
		err = h.deleteInstance(ctx, &req)
	case "EXECUTE":
		result = req.Params["command"]
	default:
		err = fmt.Errorf("unknown command: %s", req.Command)
	}

	if err != nil {
		h.respond(msg, req.ID, false, err.Error(), "")
	} else {
		h.respond(msg, req.ID, true, "", result)
	}
}

func (h *Handler) startInstance(ctx context.Context, req *model.CommandRequest) error {
	var inst model.Instance
	if err := h.db.First(&inst, "id = ?", req.InstanceID).Error; err != nil {
		return fmt.Errorf("instance not found: %w", err)
	}
	if inst.ContainerID != "" {
		return h.dockerMgr.Start(ctx, inst.ContainerID)
	}
	return fmt.Errorf("no container id for instance")
}

func (h *Handler) stopInstance(ctx context.Context, req *model.CommandRequest) error {
	var inst model.Instance
	if err := h.db.First(&inst, "id = ?", req.InstanceID).Error; err != nil {
		return fmt.Errorf("instance not found: %w", err)
	}
	if inst.ContainerID != "" {
		return h.dockerMgr.Stop(ctx, inst.ContainerID)
	}
	return nil
}

func (h *Handler) createInstance(ctx context.Context, req *model.CommandRequest) error {
	var inst model.Instance
	if err := h.db.First(&inst, "id = ?", req.InstanceID).Error; err != nil {
		return fmt.Errorf("instance not found: %w", err)
	}

	opts := docker.CreateOptions{
		Image: req.Params["software"] + ":" + req.Params["version"],
		Name:  inst.Name,
		Env: []string{
			"SERVICE=" + req.Params["service"],
			"MEMORY=" + req.Params["memory"],
		},
		Resources: docker.Resources{
			Memory: req.Params["memory"],
		},
	}

	id, err := h.dockerMgr.Create(ctx, opts)
	if err != nil {
		return fmt.Errorf("create container: %w", err)
	}

	inst.ContainerID = id
	inst.Status = model.InstanceStarting
	return h.db.Save(&inst).Error
}

func (h *Handler) deleteInstance(ctx context.Context, req *model.CommandRequest) error {
	var inst model.Instance
	if err := h.db.First(&inst, "id = ?", req.InstanceID).Error; err != nil {
		return fmt.Errorf("instance not found: %w", err)
	}
	if inst.ContainerID != "" {
		if err := h.dockerMgr.Remove(ctx, inst.ContainerID); err != nil {
			return fmt.Errorf("remove container: %w", err)
		}
	}
	return h.db.Delete(&inst).Error
}

func (h *Handler) respond(msg *nats.Msg, id string, success bool, errMsg string, result string) {
	resp := model.CommandResponse{
		ID:      id,
		Success: success,
		Error:   errMsg,
		Result:  result,
	}
	data, _ := json.Marshal(resp)
	_ = msg.Respond(data)
}
