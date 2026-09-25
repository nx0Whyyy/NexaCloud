package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/mail"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/nexastudio/nexacloud/pkg/model"
	"github.com/nexastudio/nexacloud/services/orchestrator/internal/mailer"
	"gorm.io/gorm"
)

const (
	cookieName      = "__Host-nexa_session"
	sessionDuration = 7 * 24 * time.Hour
)

var usernamePattern = regexp.MustCompile(`^[a-zA-Z0-9_-]{3,32}$`)

type Provisioner func(*gorm.DB, *model.User) error

type Service struct {
	db            *gorm.DB
	provision     Provisioner
	mailer        *mailer.Service
	publicURL     string
	loginLimit    *limiter
	registerLimit *limiter
}

type credentials struct {
	Username             string `json:"username"`
	Email                string `json:"email"`
	Password             string `json:"password"`
	PasswordConfirmation string `json:"password_confirmation"`
}

func New(db *gorm.DB, provision Provisioner, mailService *mailer.Service, publicURL string) *Service {
	return &Service{db: db, provision: provision, mailer: mailService, publicURL: mailer.NormalizePublicURL(publicURL), loginLimit: newLimiter(5, 15*time.Minute), registerLimit: newLimiter(5, time.Hour)}
}

func (s *Service) Migrate() error {
	hadVerifiedColumn := s.db.Migrator().HasColumn(&model.User{}, "EmailVerifiedAt")
	if err := s.db.AutoMigrate(&model.User{}, &model.UserSession{}, &model.EmailVerification{}); err != nil {
		return err
	}
	if !hadVerifiedColumn {
		if err := s.db.Model(&model.User{}).Where("email_verified_at IS NULL").Update("email_verified_at", gorm.Expr("created_at")).Error; err != nil {
			return err
		}
	}
	return s.db.Model(&model.User{}).Where("role = ?", "staff").Update("role", "support").Error
}

func (s *Service) EnsureAdmin(username, email, password string) error {
	if username == "" || email == "" || password == "" {
		return nil
	}
	var count int64
	if err := s.db.Model(&model.User{}).Where("role = ?", "admin").Count(&count).Error; err != nil || count > 0 {
		return err
	}
	hash, err := hashPassword(password)
	if err != nil {
		return err
	}
	now := model.Now()
	user := model.User{
		ID: model.NewID(), Username: username, Email: strings.ToLower(email),
		PasswordHash: hash, Role: "admin", CreatedAt: now, UpdatedAt: now,
	}
	return s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&user).Error; err != nil {
			return err
		}
		if s.provision != nil {
			return s.provision(tx, &user)
		}
		return nil
	})
}

func (s *Service) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/v1/auth/register", s.register)
	mux.HandleFunc("POST /api/v1/auth/login", s.login)
	mux.HandleFunc("POST /api/v1/auth/logout", s.logout)
	mux.HandleFunc("POST /api/v1/auth/verify", s.verifyEmail)
	mux.HandleFunc("POST /api/v1/auth/resend-verification", s.resendVerification)
	mux.HandleFunc("GET /api/v1/auth/me", s.me)
	mux.HandleFunc("PATCH /api/v1/auth/profile", s.updateProfile)
	mux.HandleFunc("POST /api/v1/auth/password", s.changePassword)
	mux.HandleFunc("GET /api/v1/dashboard", s.dashboard)
	mux.HandleFunc("GET /api/v1/staff/overview", s.staffOverview)
	mux.HandleFunc("PATCH /api/v1/staff/users/{id}/role", s.updateUserRole)
}

func (s *Service) register(w http.ResponseWriter, r *http.Request) {
	if !validOrigin(r) {
		writeError(w, http.StatusForbidden, "origine refusée")
		return
	}
	if !s.registerLimit.allow(requestIP(r), model.Now()) {
		w.Header().Set("Retry-After", "3600")
		writeError(w, http.StatusTooManyRequests, "Trop de tentatives. Réessayez plus tard.")
		return
	}
	var input credentials
	if json.NewDecoder(http.MaxBytesReader(w, r.Body, 8192)).Decode(&input) != nil {
		writeError(w, http.StatusBadRequest, "requête invalide")
		return
	}
	input.Username = strings.TrimSpace(input.Username)
	input.Email = strings.ToLower(strings.TrimSpace(input.Email))
	if !usernamePattern.MatchString(input.Username) || !validEmail(input.Email) {
		writeError(w, http.StatusBadRequest, "identifiants invalides")
		return
	}
	if input.Password != input.PasswordConfirmation {
		writeError(w, http.StatusBadRequest, "Les deux mots de passe ne correspondent pas.")
		return
	}
	if message := passwordPolicy(input.Password, input.Username, input.Email); message != "" {
		writeError(w, http.StatusBadRequest, message)
		return
	}
	hash, err := hashPassword(input.Password)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "inscription impossible")
		return
	}
	now := model.Now()
	user := model.User{ID: model.NewID(), Username: input.Username, Email: input.Email, PasswordHash: hash, Role: "user", CreatedAt: now, UpdatedAt: now}
	var verificationToken string
	if s.mailer == nil || !s.mailer.Enabled() {
		user.EmailVerifiedAt = &now
	} else {
		verificationToken, err = randomHex(32)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "inscription impossible")
			return
		}
	}
	if err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&user).Error; err != nil {
			return err
		}
		if s.provision != nil {
			if err := s.provision(tx, &user); err != nil {
				return err
			}
		}
		if verificationToken != "" {
			return tx.Create(&model.EmailVerification{TokenHash: tokenHash(verificationToken), UserID: user.ID, ExpiresAt: now.Add(30 * time.Minute), CreatedAt: now}).Error
		}
		return nil
	}); err != nil {
		writeError(w, http.StatusConflict, "email ou pseudo déjà utilisé")
		return
	}
	if verificationToken != "" {
		if err := s.mailer.SendVerification(user.Email, user.Username, s.publicURL+"/verify#token="+verificationToken); err != nil {
			slog.Error("failed to send verification email", "error", err)
			writeError(w, http.StatusServiceUnavailable, "Compte créé, mais l'e-mail n'a pas pu être envoyé. Utilisez le renvoi de vérification.")
			return
		}
		writeJSON(w, http.StatusAccepted, map[string]any{"verification_required": true, "email": user.Email})
		return
	}
	if err := s.startSession(w, &user); err != nil {
		writeError(w, http.StatusInternalServerError, "session impossible")
		return
	}
	writeJSON(w, http.StatusCreated, user)
}

func (s *Service) login(w http.ResponseWriter, r *http.Request) {
	if !validOrigin(r) {
		writeError(w, http.StatusForbidden, "origine refusée")
		return
	}
	var input credentials
	if json.NewDecoder(http.MaxBytesReader(w, r.Body, 8192)).Decode(&input) != nil {
		writeError(w, http.StatusBadRequest, "requête invalide")
		return
	}
	identifier := strings.ToLower(strings.TrimSpace(input.Email))
	if identifier == "" {
		identifier = strings.ToLower(strings.TrimSpace(input.Username))
	}
	limitKey := requestIP(r) + ":" + identifier
	if !s.loginLimit.allow(limitKey, model.Now()) {
		w.Header().Set("Retry-After", "900")
		writeError(w, http.StatusTooManyRequests, "Trop de tentatives. Réessayez dans quelques minutes.")
		return
	}
	var user model.User
	if err := s.db.Where("LOWER(email) = ? OR LOWER(username) = ?", identifier, identifier).First(&user).Error; err != nil {
		verifyPassword(dummyPasswordHash, input.Password)
		writeError(w, http.StatusUnauthorized, "identifiants incorrects")
		return
	}
	valid, legacy := verifyPassword(user.PasswordHash, input.Password)
	if !valid {
		writeError(w, http.StatusUnauthorized, "identifiants incorrects")
		return
	}
	if user.DisabledAt != nil {
		writeError(w, http.StatusForbidden, "Ce compte est désactivé.")
		return
	}
	if user.EmailVerifiedAt == nil {
		writeError(w, http.StatusForbidden, "Vérifiez votre adresse e-mail avant de vous connecter.")
		return
	}
	s.loginLimit.clear(limitKey)
	if legacy {
		if hash, err := hashPassword(input.Password); err == nil {
			s.db.Model(&user).Update("password_hash", hash)
		}
	}
	if err := s.startSession(w, &user); err != nil {
		writeError(w, http.StatusInternalServerError, "session impossible")
		return
	}
	writeJSON(w, http.StatusOK, user)
}

func (s *Service) logout(w http.ResponseWriter, r *http.Request) {
	if !validOrigin(r) {
		writeError(w, http.StatusForbidden, "origine refusée")
		return
	}
	if cookie, err := r.Cookie(cookieName); err == nil {
		s.db.Delete(&model.UserSession{}, "token_hash = ?", tokenHash(cookie.Value))
	}
	http.SetCookie(w, &http.Cookie{Name: cookieName, Value: "", Path: "/", MaxAge: -1, HttpOnly: true, Secure: true, SameSite: http.SameSiteLaxMode})
	w.WriteHeader(http.StatusNoContent)
}

func (s *Service) me(w http.ResponseWriter, r *http.Request) {
	user, err := s.currentUser(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "session requise")
		return
	}
	writeJSON(w, http.StatusOK, user)
}

func (s *Service) dashboard(w http.ResponseWriter, r *http.Request) {
	user, err := s.currentUser(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "session requise")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"user": user, "counts": s.counts(), "activity": s.activity(8)})
}

func (s *Service) staffOverview(w http.ResponseWriter, r *http.Request) {
	user, err := s.currentUser(r)
	if err != nil || !staffRole(user.Role) {
		writeError(w, http.StatusForbidden, "accès staff requis")
		return
	}
	var users []model.User
	s.db.Order("created_at DESC").Limit(50).Find(&users)
	writeJSON(w, http.StatusOK, map[string]any{"user": user, "counts": s.counts(), "users": users, "activity": s.activity(20)})
}

func staffRole(role string) bool {
	return role == "support" || role == "moderator" || role == "admin" || role == "owner" || role == "staff"
}

func (s *Service) counts() map[string]int64 {
	result := map[string]int64{}
	for key, entity := range map[string]any{"nodes": &model.Node{}, "services": &model.Service{}, "instances": &model.Instance{}, "users": &model.User{}} {
		var count int64
		s.db.Model(entity).Count(&count)
		result[key] = count
	}
	return result
}

func (s *Service) activity(limit int) []model.TimelineEvent {
	var events []model.TimelineEvent
	s.db.Order("created_at DESC").Limit(limit).Find(&events)
	return events
}

func (s *Service) currentUser(r *http.Request) (*model.User, error) {
	cookie, err := r.Cookie(cookieName)
	if err != nil {
		return nil, err
	}
	var session model.UserSession
	if err := s.db.Where("token_hash = ? AND expires_at > ?", tokenHash(cookie.Value), model.Now()).First(&session).Error; err != nil {
		return nil, err
	}
	var user model.User
	if err := s.db.First(&user, "id = ?", session.UserID).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

func (s *Service) CurrentUser(r *http.Request) (*model.User, error) {
	return s.currentUser(r)
}

func (s *Service) startSession(w http.ResponseWriter, user *model.User) error {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return err
	}
	token := hex.EncodeToString(raw)
	expires := model.Now().Add(sessionDuration)
	if err := s.db.Create(&model.UserSession{TokenHash: tokenHash(token), UserID: user.ID, ExpiresAt: expires, CreatedAt: model.Now()}).Error; err != nil {
		return err
	}
	http.SetCookie(w, &http.Cookie{Name: cookieName, Value: token, Path: "/", Expires: expires, MaxAge: int(sessionDuration.Seconds()), HttpOnly: true, Secure: true, SameSite: http.SameSiteLaxMode})
	return nil
}

func randomHex(size int) (string, error) {
	raw := make([]byte, size)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return hex.EncodeToString(raw), nil
}

func validEmail(value string) bool {
	if len(value) > 255 || strings.ContainsAny(value, "\r\n") {
		return false
	}
	address, err := mail.ParseAddress(value)
	return err == nil && strings.EqualFold(address.Address, value) && strings.Contains(strings.Split(address.Address, "@")[1], ".")
}

func tokenHash(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func validOrigin(r *http.Request) bool {
	if r.Header.Get("Sec-Fetch-Site") == "cross-site" {
		return false
	}
	origin := r.Header.Get("Origin")
	if origin == "" {
		return true
	}
	parsed, err := url.Parse(origin)
	return err == nil && strings.EqualFold(parsed.Host, r.Host)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}
func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
