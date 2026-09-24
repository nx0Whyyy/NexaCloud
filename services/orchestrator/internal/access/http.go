package access

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strings"

	"github.com/google/uuid"
	"github.com/nexastudio/nexacloud/pkg/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type UserResolver func(*http.Request) (*model.User, error)

func (s *Service) RegisterRoutes(mux *http.ServeMux, currentUser UserResolver) {
	mux.HandleFunc("GET /api/v1/organizations/current", s.currentOrganization(currentUser))
	mux.HandleFunc("GET /api/v1/entitlements", s.entitlements(currentUser))
	mux.HandleFunc("POST /api/v1/networks", s.createNetwork(currentUser))
	mux.HandleFunc("POST /api/v1/licenses", s.createLicense(currentUser))
	mux.HandleFunc("DELETE /api/v1/licenses/{id}", s.revokeLicense(currentUser))
	mux.HandleFunc("POST /api/v1/nodes/enroll", s.enrollNode)
	mux.HandleFunc("POST /api/v1/device/code", s.createDeviceCode)
	mux.HandleFunc("POST /api/v1/device/token", s.claimDeviceEnrollment)
	mux.HandleFunc("POST /api/v1/device/approve", s.approveDeviceEnrollment(currentUser))
	mux.HandleFunc("DELETE /api/v1/nodes/{id}", s.revokeNode(currentUser))
}

func (s *Service) currentOrganization(resolve UserResolver) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		_, org, role, ok := s.authorize(w, r, resolve)
		if !ok {
			return
		}
		var networks []model.Network
		var licenses []model.License
		var nodes []model.Node
		var services []model.Service
		var instances []model.Instance
		var activity []model.AuditEntry
		s.db.Where("organization_id = ?", org.ID).Order("created_at").Find(&networks)
		s.db.Where("organization_id = ?", org.ID).Order("created_at").Find(&licenses)
		s.db.Where("organization_id = ?", org.ID).Order("created_at").Find(&nodes)
		s.db.Where("organization_id = ?", org.ID).Order("created_at").Find(&services)
		s.db.Where("organization_id = ?", org.ID).Order("created_at").Find(&instances)
		s.db.Where("organization_id = ?", org.ID).Order("created_at DESC").Limit(20).Find(&activity)
		writeJSON(w, http.StatusOK, map[string]any{"organization": org, "role": role, "networks": networks, "nodes": nodes, "services": services, "instances": instances, "licenses": licenses, "activity": activity})
	}
}

func (s *Service) entitlements(resolve UserResolver) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		_, org, _, ok := s.authorize(w, r, resolve)
		if !ok {
			return
		}
		rights, subscription, err := s.EntitlementsFor(org.ID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "entitlements unavailable")
			return
		}
		var active int64
		var plan model.Plan
		s.db.First(&plan, "id = ?", subscription.PlanID)
		s.db.Model(&model.InstanceSlot{}).Where("organization_id = ?", org.ID).Count(&active)
		writeJSON(w, http.StatusOK, map[string]any{"subscription": subscription, "plan": plan, "entitlements": rights, "usage": map[string]int64{"active_instances": active}})
	}
}

func (s *Service) createNetwork(resolve UserResolver) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !validMutationOrigin(r) {
			writeError(w, http.StatusForbidden, "origine refusée")
			return
		}
		user, org, role, ok := s.authorize(w, r, resolve)
		if !ok {
			return
		}
		if role != "owner" && role != "admin" && role != "infrastructure_admin" {
			writeError(w, http.StatusForbidden, "network.manage permission required")
			return
		}
		var input struct {
			Name string `json:"name"`
		}
		if json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&input) != nil || strings.TrimSpace(input.Name) == "" {
			writeError(w, http.StatusBadRequest, "valid network name required")
			return
		}
		rights, _, err := s.EntitlementsFor(org.ID)
		var count int64
		s.db.Model(&model.Network{}).Where("organization_id = ?", org.ID).Count(&count)
		if err != nil || int(count) >= rights.Limits["max_networks"] {
			writeError(w, http.StatusConflict, "network quota reached")
			return
		}
		now := model.Now()
		network := model.Network{ID: model.NewID(), OrganizationID: org.ID, Name: strings.TrimSpace(input.Name), Status: "ACTIVE", CreatedAt: now, UpdatedAt: now}
		if err := s.db.Create(&network).Error; err != nil {
			writeError(w, http.StatusConflict, "network could not be created")
			return
		}
		s.audit(org.ID, user.ID.String(), "NETWORK_CREATE", "network", network.ID.String(), clientIP(r))
		writeJSON(w, http.StatusCreated, network)
	}
}

func (s *Service) createLicense(resolve UserResolver) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !validMutationOrigin(r) {
			writeError(w, http.StatusForbidden, "origine refusée")
			return
		}
		user, org, role, ok := s.authorize(w, r, resolve)
		if !ok {
			return
		}
		if role != "owner" && role != "admin" {
			writeError(w, http.StatusForbidden, "license.manage permission required")
			return
		}
		key, hash, err := NewLicenseKey()
		if err != nil {
			writeError(w, http.StatusInternalServerError, "license could not be created")
			return
		}
		license := model.License{ID: model.NewID(), OrganizationID: org.ID, KeyPrefix: key[:7], KeyHash: hash, Status: "ACTIVE", CreatedAt: model.Now()}
		if err := s.db.Create(&license).Error; err != nil {
			writeError(w, http.StatusInternalServerError, "license could not be created")
			return
		}
		s.audit(org.ID, user.ID.String(), "LICENSE_CREATE", "license", license.ID.String(), clientIP(r))
		writeJSON(w, http.StatusCreated, map[string]any{"license": license, "key": key})
	}
}

func (s *Service) revokeLicense(resolve UserResolver) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !validMutationOrigin(r) {
			writeError(w, http.StatusForbidden, "origine refusée")
			return
		}
		user, org, role, ok := s.authorize(w, r, resolve)
		if !ok {
			return
		}
		if role != "owner" && role != "admin" {
			writeError(w, http.StatusForbidden, "license.manage permission required")
			return
		}
		id, err := uuid.Parse(r.PathValue("id"))
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid license id")
			return
		}
		now := model.Now()
		result := s.db.Model(&model.License{}).Where("id = ? AND organization_id = ? AND revoked_at IS NULL", id, org.ID).Updates(map[string]any{"status": "REVOKED", "revoked_at": now})
		if result.Error != nil || result.RowsAffected == 0 {
			writeError(w, http.StatusNotFound, "license not found")
			return
		}
		s.audit(org.ID, user.ID.String(), "LICENSE_REVOKE", "license", id.String(), clientIP(r))
		w.WriteHeader(http.StatusNoContent)
	}
}

func (s *Service) audit(orgID uuid.UUID, actor, action, resourceType, resourceID, ip string) {
	_ = s.db.Create(&model.AuditEntry{ID: model.NewID(), OrganizationID: &orgID, Actor: actor, Action: action, ResourceType: resourceType, ResourceID: resourceID, IPAddress: ip, Result: "success", CreatedAt: model.Now()}).Error
}

func validMutationOrigin(r *http.Request) bool {
	if r.Header.Get("Sec-Fetch-Site") == "cross-site" {
		return false
	}
	origin := r.Header.Get("Origin")
	if origin == "" {
		return r.Header.Get("Sec-Fetch-Site") == "" || r.Header.Get("Sec-Fetch-Site") == "same-origin"
	}
	parsed, err := url.Parse(origin)
	return err == nil && strings.EqualFold(parsed.Host, r.Host)
}

func (s *Service) enrollNode(w http.ResponseWriter, r *http.Request) {
	if !s.enrollLimit.allow(clientIP(r)) {
		writeError(w, http.StatusTooManyRequests, "enrollment rate limit reached")
		return
	}
	var input struct {
		LicenseKey string          `json:"license_key"`
		NetworkID  uuid.UUID       `json:"network_id"`
		Name       string          `json:"name"`
		PublicKey  string          `json:"public_key"`
		Resources  model.Resources `json:"resources"`
	}
	if json.NewDecoder(http.MaxBytesReader(w, r.Body, 8192)).Decode(&input) != nil || input.LicenseKey == "" || input.NetworkID == uuid.Nil || strings.TrimSpace(input.Name) == "" || len(strings.TrimSpace(input.PublicKey)) < 32 {
		writeError(w, http.StatusBadRequest, "invalid enrollment request")
		return
	}
	sum := sha256.Sum256([]byte(strings.ToUpper(strings.TrimSpace(input.LicenseKey))))
	hash := hex.EncodeToString(sum[:])
	var created model.Node
	var credential model.NodeCredential
	err := s.db.Transaction(func(tx *gorm.DB) error {
		var license model.License
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("key_hash = ?", hash).First(&license).Error; err != nil || !LicenseUsable(license, model.Now()) {
			return errors.New("invalid license")
		}
		var network model.Network
		if err := tx.Where("id = ? AND organization_id = ? AND status = ?", input.NetworkID, license.OrganizationID, "ACTIVE").First(&network).Error; err != nil {
			return errors.New("invalid network")
		}
		var subscription model.Subscription
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("organization_id = ?", license.OrganizationID).First(&subscription).Error; err != nil {
			return err
		}
		var rights Entitlements
		if err := json.Unmarshal([]byte(subscription.EntitlementData), &rights); err != nil {
			return err
		}
		var count int64
		if err := tx.Model(&model.Node{}).Where("organization_id = ? AND status <> ?", license.OrganizationID, "REVOKED").Count(&count).Error; err != nil {
			return err
		}
		if int(count) >= rights.Limits["max_nodes"] {
			return errors.New("node quota reached")
		}
		now := model.Now()
		created = model.Node{ID: model.NewID(), OrganizationID: &license.OrganizationID, NetworkID: &network.ID, Name: strings.TrimSpace(input.Name), Status: model.NodeOffline, Labels: model.Labels{}, Resources: input.Resources, CreatedAt: now, UpdatedAt: now}
		if err := tx.Create(&created).Error; err != nil {
			return err
		}
		fingerprint := sha256.Sum256([]byte(strings.TrimSpace(input.PublicKey)))
		credential = model.NodeCredential{ID: model.NewID(), NodeID: created.ID, PublicKey: strings.TrimSpace(input.PublicKey), Fingerprint: hex.EncodeToString(fingerprint[:]), Status: "ACTIVE", CreatedAt: now}
		if err := tx.Create(&credential).Error; err != nil {
			return err
		}
		return tx.Model(&license).Update("last_used_at", now).Error
	})
	if err != nil {
		writeError(w, http.StatusForbidden, safeAccessError(err))
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"node": created, "credential_id": credential.ID, "fingerprint": credential.Fingerprint})
}

func (s *Service) authorize(w http.ResponseWriter, r *http.Request, resolve UserResolver) (*model.User, model.Organization, string, bool) {
	user, err := resolve(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "session required")
		return nil, model.Organization{}, "", false
	}
	var membership model.OrganizationMember
	if err := s.db.Where("user_id = ?", user.ID).Order("created_at").First(&membership).Error; err != nil {
		writeError(w, http.StatusForbidden, "organization membership required")
		return nil, model.Organization{}, "", false
	}
	var org model.Organization
	if err := s.db.First(&org, "id = ?", membership.OrganizationID).Error; err != nil {
		writeError(w, http.StatusNotFound, "organization not found")
		return nil, model.Organization{}, "", false
	}
	return user, org, membership.Role, true
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

func safeAccessError(err error) string {
	message := err.Error()
	allowed := map[string]bool{"invalid license": true, "invalid network": true, "node quota reached": true, "device code is invalid or expired": true, "network not found": true}
	if allowed[message] {
		return message
	}
	return "request could not be completed"
}
