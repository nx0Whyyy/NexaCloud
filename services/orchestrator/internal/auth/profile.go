package auth

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/nexastudio/nexacloud/pkg/model"
)

func (s *Service) updateProfile(w http.ResponseWriter, r *http.Request) {
	if !validOrigin(r) {
		writeError(w, http.StatusForbidden, "origine refusée")
		return
	}
	user, err := s.currentUser(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "session requise")
		return
	}
	var input struct {
		Username string `json:"username"`
	}
	if json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&input) != nil {
		writeError(w, http.StatusBadRequest, "requête invalide")
		return
	}
	input.Username = strings.TrimSpace(input.Username)
	if !usernamePattern.MatchString(input.Username) {
		writeError(w, http.StatusBadRequest, "Le pseudo doit contenir 3 à 32 caractères alphanumériques, tirets ou underscores.")
		return
	}
	previous := user.Username
	if err := s.db.Model(user).Updates(map[string]any{"username": input.Username, "updated_at": model.Now()}).Error; err != nil {
		writeError(w, http.StatusConflict, "Ce pseudo est déjà utilisé.")
		return
	}
	_ = s.db.Create(&model.AuditEntry{ID: model.NewID(), Actor: user.ID.String(), Action: "PROFILE_UPDATE", ResourceType: "user", ResourceID: user.ID.String(), IPAddress: requestIP(r), Result: "success", Details: model.JSON(map[string]string{"previous_username": previous}), CreatedAt: model.Now()}).Error
	user.Username = input.Username
	writeJSON(w, http.StatusOK, user)
}

func (s *Service) changePassword(w http.ResponseWriter, r *http.Request) {
	if !validOrigin(r) {
		writeError(w, http.StatusForbidden, "origine refusée")
		return
	}
	user, err := s.currentUser(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "session requise")
		return
	}
	var input struct {
		CurrentPassword      string `json:"current_password"`
		NewPassword          string `json:"new_password"`
		PasswordConfirmation string `json:"password_confirmation"`
	}
	if json.NewDecoder(http.MaxBytesReader(w, r.Body, 8192)).Decode(&input) != nil {
		writeError(w, http.StatusBadRequest, "requête invalide")
		return
	}
	valid, _ := verifyPassword(user.PasswordHash, input.CurrentPassword)
	if !valid {
		writeError(w, http.StatusUnauthorized, "Le mot de passe actuel est incorrect.")
		return
	}
	if input.NewPassword != input.PasswordConfirmation {
		writeError(w, http.StatusBadRequest, "Les deux nouveaux mots de passe ne correspondent pas.")
		return
	}
	if message := passwordPolicy(input.NewPassword, user.Username, user.Email); message != "" {
		writeError(w, http.StatusBadRequest, message)
		return
	}
	hash, err := hashPassword(input.NewPassword)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "mise à jour impossible")
		return
	}
	if err := s.db.Model(user).Updates(map[string]any{"password_hash": hash, "updated_at": model.Now()}).Error; err != nil {
		writeError(w, http.StatusInternalServerError, "mise à jour impossible")
		return
	}
	if cookie, err := r.Cookie(cookieName); err == nil {
		s.db.Where("user_id = ? AND token_hash <> ?", user.ID, tokenHash(cookie.Value)).Delete(&model.UserSession{})
	}
	_ = s.db.Create(&model.AuditEntry{ID: model.NewID(), Actor: user.ID.String(), Action: "PASSWORD_UPDATE", ResourceType: "user", ResourceID: user.ID.String(), IPAddress: requestIP(r), Result: "success", CreatedAt: model.Now()}).Error
	w.WriteHeader(http.StatusNoContent)
}
