package auth

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/nexastudio/nexacloud/pkg/model"
	"gorm.io/gorm"
)

func (s *Service) verifyEmail(w http.ResponseWriter, r *http.Request) {
	if !validOrigin(r) {
		writeError(w, http.StatusForbidden, "origine refusée")
		return
	}
	var input struct {
		Token string `json:"token"`
	}
	if json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&input) != nil || len(input.Token) != 64 {
		writeError(w, http.StatusBadRequest, "Lien de vérification invalide.")
		return
	}
	err := s.db.Transaction(func(tx *gorm.DB) error {
		var verification model.EmailVerification
		if err := tx.Where("token_hash = ? AND expires_at > ?", tokenHash(input.Token), model.Now()).First(&verification).Error; err != nil {
			return err
		}
		now := model.Now()
		if err := tx.Model(&model.User{}).Where("id = ? AND email_verified_at IS NULL", verification.UserID).Update("email_verified_at", now).Error; err != nil {
			return err
		}
		return tx.Delete(&model.EmailVerification{}, "user_id = ?", verification.UserID).Error
	})
	if err != nil {
		writeError(w, http.StatusGone, "Ce lien est invalide ou a expiré.")
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"verified": true})
}

func (s *Service) resendVerification(w http.ResponseWriter, r *http.Request) {
	if !validOrigin(r) {
		writeError(w, http.StatusForbidden, "origine refusée")
		return
	}
	if !s.registerLimit.allow("resend:"+requestIP(r), model.Now()) {
		w.Header().Set("Retry-After", "3600")
		writeError(w, http.StatusTooManyRequests, "Trop de demandes. Réessayez plus tard.")
		return
	}
	var input struct {
		Email string `json:"email"`
	}
	if json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&input) != nil {
		writeError(w, http.StatusBadRequest, "requête invalide")
		return
	}
	email := strings.ToLower(strings.TrimSpace(input.Email))
	var user model.User
	if s.mailer == nil || !s.mailer.Enabled() || s.db.Where("LOWER(email) = ? AND email_verified_at IS NULL", email).First(&user).Error != nil {
		writeJSON(w, http.StatusOK, map[string]bool{"sent": true})
		return
	}
	token, err := randomHex(32)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "envoi impossible")
		return
	}
	now := model.Now()
	err = s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Delete(&model.EmailVerification{}, "user_id = ?", user.ID).Error; err != nil {
			return err
		}
		return tx.Create(&model.EmailVerification{TokenHash: tokenHash(token), UserID: user.ID, ExpiresAt: now.Add(30 * time.Minute), CreatedAt: now}).Error
	})
	if err == nil {
		err = s.mailer.SendVerification(user.Email, user.Username, s.publicURL+"/verify#token="+token)
	}
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "Le service e-mail est temporairement indisponible.")
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"sent": true})
}
