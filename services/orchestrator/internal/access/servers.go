package access

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"path"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/nexastudio/nexacloud/pkg/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var serverNamePattern = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9 _-]{2,31}$`)
var memoryPattern = regexp.MustCompile(`^[1-9][0-9]{0,2}[GM]$`)

func canManageInfrastructure(role string) bool {
	return role == "owner" || role == "admin" || role == "infrastructure_admin"
}

func (s *Service) listServers(resolve UserResolver) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		_, org, _, ok := s.authorize(w, r, resolve)
		if !ok {
			return
		}
		var instances []model.Instance
		s.db.Where("organization_id = ?", org.ID).Order("created_at DESC").Find(&instances)
		writeJSON(w, http.StatusOK, instances)
	}
}

func (s *Service) createServer(resolve UserResolver) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !validMutationOrigin(r) {
			writeError(w, http.StatusForbidden, "origine refusée")
			return
		}
		user, org, role, ok := s.authorize(w, r, resolve)
		if !ok {
			return
		}
		if !canManageInfrastructure(role) {
			writeError(w, http.StatusForbidden, "server.manage permission required")
			return
		}
		var input struct {
			Name   string    `json:"name"`
			NodeID uuid.UUID `json:"node_id"`
			Port   int       `json:"port"`
			Memory string    `json:"memory"`
		}
		if json.NewDecoder(http.MaxBytesReader(w, r.Body, 8192)).Decode(&input) != nil {
			writeError(w, http.StatusBadRequest, "requête invalide")
			return
		}
		input.Name, input.Memory = strings.TrimSpace(input.Name), strings.ToUpper(strings.TrimSpace(input.Memory))
		if !serverNamePattern.MatchString(input.Name) || input.NodeID == uuid.Nil || input.Port < 1024 || input.Port > 65535 || !memoryPattern.MatchString(input.Memory) {
			writeError(w, http.StatusBadRequest, "nom, node, port ou mémoire invalide")
			return
		}
		var node model.Node
		if err := s.db.Where("id = ? AND organization_id = ? AND status = ?", input.NodeID, org.ID, model.NodeOnline).First(&node).Error; err != nil {
			writeError(w, http.StatusConflict, "node indisponible")
			return
		}
		var used int64
		s.db.Model(&model.Instance{}).Where("node_id = ? AND port = ?", node.ID, input.Port).Count(&used)
		if used > 0 {
			writeError(w, http.StatusConflict, "ce port est déjà utilisé sur le node")
			return
		}
		now := model.Now()
		instanceID := model.NewID()
		if err := s.ReserveInstanceSlot(org.ID, instanceID); err != nil {
			writeError(w, http.StatusConflict, err.Error())
			return
		}
		instance := model.Instance{ID: instanceID, OrganizationID: &org.ID, NetworkID: node.NetworkID, Name: input.Name, ServiceName: "managed", NodeID: &node.ID, Status: model.InstanceCreating, Pulse: model.PulseOffline, Port: input.Port, Address: fmt.Sprintf("%s:%d", node.Name, input.Port), Resources: model.Resources{Memory: input.Memory}, Metadata: model.InstanceMeta{Software: model.SoftwareSpec{Type: model.SoftwarePaper, Version: "latest", Java: model.JavaSpec{Version: 25}}}, CreatedAt: now, UpdatedAt: now}
		command := model.AgentCommand{ID: model.NewID(), OrganizationID: org.ID, NodeID: node.ID, InstanceID: &instance.ID, Command: "CREATE_SERVER", Status: "PENDING", Params: map[string]string{"name": "server-" + instance.ID.String()[:8], "port": strconv.Itoa(input.Port), "memory": input.Memory}, CreatedAt: now}
		if err := s.db.Transaction(func(tx *gorm.DB) error {
			if err := tx.Create(&instance).Error; err != nil {
				return err
			}
			return tx.Create(&command).Error
		}); err != nil {
			_ = s.ReleaseInstanceSlot(instance.ID)
			writeError(w, http.StatusConflict, "création impossible")
			return
		}
		_ = s.db.Create(&model.AuditEntry{ID: model.NewID(), OrganizationID: &org.ID, Actor: user.ID.String(), Action: "SERVER_CREATE", ResourceType: "instance", ResourceID: instance.ID.String(), IPAddress: clientIP(r), Result: "queued", CreatedAt: now}).Error
		writeJSON(w, http.StatusAccepted, map[string]any{"instance": instance, "command": command})
	}
}

func (s *Service) serverAction(resolve UserResolver) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !validMutationOrigin(r) {
			writeError(w, http.StatusForbidden, "origine refusée")
			return
		}
		user, org, role, instance, ok := s.authorizeInstance(w, r, resolve)
		if !ok {
			return
		}
		if !canManageInfrastructure(role) {
			writeError(w, http.StatusForbidden, "server.manage permission required")
			return
		}
		var input struct {
			Action string `json:"action"`
		}
		if json.NewDecoder(http.MaxBytesReader(w, r.Body, 2048)).Decode(&input) != nil {
			writeError(w, http.StatusBadRequest, "requête invalide")
			return
		}
		commands := map[string]string{"start": "START_SERVER", "stop": "STOP_SERVER", "restart": "RESTART_SERVER", "kill": "KILL_SERVER", "delete": "DELETE_SERVER", "logs": "SERVER_LOGS"}
		commandName, exists := commands[strings.ToLower(input.Action)]
		if !exists {
			writeError(w, http.StatusBadRequest, "action invalide")
			return
		}
		command := s.queueCommand(org.ID, *instance.NodeID, instance.ID, commandName, nil)
		_ = s.db.Create(&model.AuditEntry{ID: model.NewID(), OrganizationID: &org.ID, Actor: user.ID.String(), Action: commandName, ResourceType: "instance", ResourceID: instance.ID.String(), IPAddress: clientIP(r), Result: "queued", CreatedAt: model.Now()}).Error
		writeJSON(w, http.StatusAccepted, command)
	}
}

func (s *Service) serverConsole(resolve UserResolver) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !validMutationOrigin(r) {
			writeError(w, http.StatusForbidden, "origine refusée")
			return
		}
		_, org, role, instance, ok := s.authorizeInstance(w, r, resolve)
		if !ok {
			return
		}
		if !canManageInfrastructure(role) {
			writeError(w, http.StatusForbidden, "server.console permission required")
			return
		}
		var input struct {
			Command string `json:"command"`
		}
		if json.NewDecoder(http.MaxBytesReader(w, r.Body, 8192)).Decode(&input) != nil {
			writeError(w, http.StatusBadRequest, "requête invalide")
			return
		}
		input.Command = strings.TrimSpace(input.Command)
		if input.Command == "" || len(input.Command) > 500 || strings.ContainsAny(input.Command, "\r\n\x00") {
			writeError(w, http.StatusBadRequest, "commande invalide")
			return
		}
		writeJSON(w, http.StatusAccepted, s.queueCommand(org.ID, *instance.NodeID, instance.ID, "CONSOLE", map[string]string{"command": input.Command}))
	}
}

func (s *Service) serverFiles(resolve UserResolver) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !validMutationOrigin(r) {
			writeError(w, http.StatusForbidden, "origine refusée")
			return
		}
		_, org, role, instance, ok := s.authorizeInstance(w, r, resolve)
		if !ok {
			return
		}
		if !canManageInfrastructure(role) {
			writeError(w, http.StatusForbidden, "server.files permission required")
			return
		}
		var input struct {
			Operation string `json:"operation"`
			Path      string `json:"path"`
			Content   string `json:"content"`
		}
		if json.NewDecoder(http.MaxBytesReader(w, r.Body, 300*1024)).Decode(&input) != nil {
			writeError(w, http.StatusBadRequest, "requête invalide")
			return
		}
		clean := path.Clean("/" + strings.TrimSpace(input.Path))
		if clean == "/" {
			clean = "."
		} else {
			clean = strings.TrimPrefix(clean, "/")
		}
		commands := map[string]string{"list": "FILE_LIST", "read": "FILE_READ", "write": "FILE_WRITE"}
		commandName, exists := commands[input.Operation]
		if !exists || len(input.Content) > 256*1024 {
			writeError(w, http.StatusBadRequest, "opération fichier invalide")
			return
		}
		writeJSON(w, http.StatusAccepted, s.queueCommand(org.ID, *instance.NodeID, instance.ID, commandName, map[string]string{"path": clean, "content": input.Content}))
	}
}

func (s *Service) commandStatus(resolve UserResolver) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		_, org, _, ok := s.authorize(w, r, resolve)
		if !ok {
			return
		}
		id, err := uuid.Parse(r.PathValue("id"))
		if err != nil {
			writeError(w, http.StatusBadRequest, "commande invalide")
			return
		}
		var command model.AgentCommand
		if s.db.Where("id = ? AND organization_id = ?", id, org.ID).First(&command).Error != nil {
			writeError(w, http.StatusNotFound, "commande introuvable")
			return
		}
		writeJSON(w, http.StatusOK, command)
	}
}

func (s *Service) authorizeInstance(w http.ResponseWriter, r *http.Request, resolve UserResolver) (*model.User, model.Organization, string, model.Instance, bool) {
	user, org, role, ok := s.authorize(w, r, resolve)
	if !ok {
		return nil, org, role, model.Instance{}, false
	}
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "serveur invalide")
		return nil, org, role, model.Instance{}, false
	}
	var instance model.Instance
	if s.db.Where("id = ? AND organization_id = ?", id, org.ID).First(&instance).Error != nil || instance.NodeID == nil {
		writeError(w, http.StatusNotFound, "serveur introuvable")
		return nil, org, role, model.Instance{}, false
	}
	return user, org, role, instance, true
}

func (s *Service) queueCommand(orgID, nodeID, instanceID uuid.UUID, command string, params map[string]string) model.AgentCommand {
	queued := model.AgentCommand{ID: model.NewID(), OrganizationID: orgID, NodeID: nodeID, InstanceID: &instanceID, Command: command, Status: "PENDING", Params: params, CreatedAt: model.Now()}
	s.db.Create(&queued)
	return queued
}

func (s *Service) nextAgentCommand(w http.ResponseWriter, r *http.Request) {
	credential, ok := s.authenticateAgent(w, r)
	if !ok {
		return
	}
	var command model.AgentCommand
	err := s.db.Transaction(func(tx *gorm.DB) error {
		stale := model.Now().Add(-2 * time.Minute)
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE", Options: "SKIP LOCKED"}).Where("node_id = ? AND (status = ? OR (status = ? AND claimed_at < ?))", credential.NodeID, "PENDING", "RUNNING", stale).Order("created_at").First(&command).Error; err != nil {
			return err
		}
		now := model.Now()
		command.Status = "RUNNING"
		command.ClaimedAt = &now
		return tx.Save(&command).Error
	})
	if errors.Is(err, gorm.ErrRecordNotFound) {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "commande indisponible")
		return
	}
	writeJSON(w, http.StatusOK, command)
}

func (s *Service) completeAgentCommand(w http.ResponseWriter, r *http.Request) {
	credential, ok := s.authenticateAgent(w, r)
	if !ok {
		return
	}
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "commande invalide")
		return
	}
	var input struct {
		Success bool   `json:"success"`
		Result  string `json:"result"`
		Error   string `json:"error"`
	}
	if json.NewDecoder(http.MaxBytesReader(w, r.Body, 512*1024)).Decode(&input) != nil {
		writeError(w, http.StatusBadRequest, "résultat invalide")
		return
	}
	var command model.AgentCommand
	if s.db.Where("id = ? AND node_id = ? AND status = ?", id, credential.NodeID, "RUNNING").First(&command).Error != nil {
		writeError(w, http.StatusNotFound, "commande introuvable")
		return
	}
	now := model.Now()
	command.CompletedAt = &now
	command.Result = input.Result
	command.Error = input.Error
	if input.Success {
		command.Status = "COMPLETED"
	} else {
		command.Status = "FAILED"
	}
	s.db.Save(&command)
	if command.InstanceID != nil {
		updates := map[string]any{"updated_at": now}
		switch command.Command {
		case "CREATE_SERVER", "START_SERVER", "RESTART_SERVER":
			if input.Success {
				updates["status"] = model.InstanceActive
				updates["pulse"] = model.PulseHealthy
				if command.Command == "CREATE_SERVER" {
					updates["container_id"] = input.Result
				}
			} else {
				updates["status"] = model.InstanceCrashed
				if command.Command == "CREATE_SERVER" {
					_ = s.ReleaseInstanceSlot(*command.InstanceID)
				}
			}
		case "STOP_SERVER", "KILL_SERVER":
			if input.Success {
				updates["status"] = model.InstanceStopped
				updates["pulse"] = model.PulseOffline
			}
		case "DELETE_SERVER":
			if input.Success {
				_ = s.ReleaseInstanceSlot(*command.InstanceID)
				s.db.Delete(&model.Instance{}, "id = ?", *command.InstanceID)
				w.WriteHeader(http.StatusNoContent)
				return
			}
		}
		s.db.Model(&model.Instance{}).Where("id = ?", *command.InstanceID).Updates(updates)
	}
	w.WriteHeader(http.StatusNoContent)
}
