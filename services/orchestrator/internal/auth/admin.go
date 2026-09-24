package auth

import (
	"encoding/json"
	"net/http"

	"github.com/google/uuid"
	"github.com/nexastudio/nexacloud/pkg/model"
)

var assignableRoles = map[string]bool{"user": true, "support": true, "moderator": true, "admin": true, "owner": true}

func (s *Service) updateUserRole(w http.ResponseWriter, r *http.Request) {
	if !validOrigin(r) {
		writeError(w, http.StatusForbidden, "origine refusée")
		return
	}
	actor, err := s.currentUser(r)
	if err != nil || (actor.Role != "admin" && actor.Role != "owner") {
		writeError(w, http.StatusForbidden, "accès administrateur requis")
		return
	}
	targetID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "utilisateur invalide")
		return
	}
	if targetID == actor.ID {
		writeError(w, http.StatusConflict, "Vous ne pouvez pas modifier votre propre rôle.")
		return
	}
	var input struct {
		Role string `json:"role"`
	}
	if json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&input) != nil || !assignableRoles[input.Role] {
		writeError(w, http.StatusBadRequest, "rôle invalide")
		return
	}
	var target model.User
	if err := s.db.First(&target, "id = ?", targetID).Error; err != nil {
		writeError(w, http.StatusNotFound, "utilisateur introuvable")
		return
	}
	previous := target.Role
	if err := s.db.Model(&target).Updates(map[string]any{"role": input.Role, "updated_at": model.Now()}).Error; err != nil {
		writeError(w, http.StatusInternalServerError, "mise à jour impossible")
		return
	}
	_ = s.db.Create(&model.AuditEntry{ID: model.NewID(), Actor: actor.ID.String(), Action: "USER_ROLE_UPDATE", ResourceType: "user", ResourceID: target.ID.String(), IPAddress: requestIP(r), Result: "success", Details: model.JSON(map[string]string{"previous": previous, "current": input.Role}), CreatedAt: model.Now()}).Error
	target.Role = input.Role
	writeJSON(w, http.StatusOK, target)
}
