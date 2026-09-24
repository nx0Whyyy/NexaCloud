package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/nexastudio/nexacloud/pkg/model"
	"gorm.io/gorm"
)

const (
	cookieName      = "nexa_session"
	sessionDuration = 7 * 24 * time.Hour
)

var usernamePattern = regexp.MustCompile(`^[a-zA-Z0-9_-]{3,32}$`)

type Provisioner func(*gorm.DB, *model.User) error

type Service struct {
	db        *gorm.DB
	provision Provisioner
}

type credentials struct {
	Username string `json:"username"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

func New(db *gorm.DB, provision Provisioner) *Service {
	return &Service{db: db, provision: provision}
}

func (s *Service) Migrate() error {
	return s.db.AutoMigrate(&model.User{}, &model.UserSession{})
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
	mux.HandleFunc("GET /api/v1/auth/me", s.me)
	mux.HandleFunc("GET /api/v1/dashboard", s.dashboard)
	mux.HandleFunc("GET /api/v1/staff/overview", s.staffOverview)
}

func (s *Service) register(w http.ResponseWriter, r *http.Request) {
	if !validOrigin(r) {
		writeError(w, http.StatusForbidden, "origine refusée")
		return
	}
	var input credentials
	if json.NewDecoder(http.MaxBytesReader(w, r.Body, 8192)).Decode(&input) != nil {
		writeError(w, http.StatusBadRequest, "requête invalide")
		return
	}
	input.Username = strings.TrimSpace(input.Username)
	input.Email = strings.ToLower(strings.TrimSpace(input.Email))
	if !usernamePattern.MatchString(input.Username) || !strings.Contains(input.Email, "@") || len(input.Password) < 10 {
		writeError(w, http.StatusBadRequest, "identifiants invalides")
		return
	}
	hash, err := hashPassword(input.Password)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "inscription impossible")
		return
	}
	user := model.User{ID: model.NewID(), Username: input.Username, Email: input.Email, PasswordHash: hash, Role: "user", CreatedAt: model.Now(), UpdatedAt: model.Now()}
	if err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&user).Error; err != nil {
			return err
		}
		if s.provision != nil {
			return s.provision(tx, &user)
		}
		return nil
	}); err != nil {
		writeError(w, http.StatusConflict, "email ou pseudo déjà utilisé")
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
	var user model.User
	if err := s.db.Where("LOWER(email) = ? OR LOWER(username) = ?", identifier, identifier).First(&user).Error; err != nil {
		writeError(w, http.StatusUnauthorized, "identifiants incorrects")
		return
	}
	valid, legacy := verifyPassword(user.PasswordHash, input.Password)
	if !valid {
		writeError(w, http.StatusUnauthorized, "identifiants incorrects")
		return
	}
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
	if err != nil || (user.Role != "staff" && user.Role != "admin") {
		writeError(w, http.StatusForbidden, "accès staff requis")
		return
	}
	var users []model.User
	s.db.Order("created_at DESC").Limit(50).Find(&users)
	writeJSON(w, http.StatusOK, map[string]any{"user": user, "counts": s.counts(), "users": users, "activity": s.activity(20)})
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

func tokenHash(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func validOrigin(r *http.Request) bool {
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
