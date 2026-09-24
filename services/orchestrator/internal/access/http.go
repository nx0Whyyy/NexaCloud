package access

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/nexastudio/nexacloud/pkg/model"
	"gorm.io/gorm"
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
		s.db.Where("organization_id = ?", org.ID).Order("created_at").Find(&networks)
		s.db.Where("organization_id = ?", org.ID).Order("created_at").Find(&licenses)
		s.db.Where("organization_id = ?", org.ID).Order("created_at").Find(&nodes)
		writeJSON(w, http.StatusOK, map[string]any{"organization": org, "role": role, "networks": networks, "nodes": nodes, "licenses": licenses})
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
		s.db.Model(&model.InstanceSlot{}).Where("organization_id = ?", org.ID).Count(&active)
		writeJSON(w, http.StatusOK, map[string]any{"subscription": subscription, "entitlements": rights, "usage": map[string]int64{"active_instances": active}})
	}
}

func (s *Service) createNetwork(resolve UserResolver) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		_, org, role, ok := s.authorize(w, r, resolve)
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
		writeJSON(w, http.StatusCreated, network)
	}
}

func (s *Service) createLicense(resolve UserResolver) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		_, org, role, ok := s.authorize(w, r, resolve)
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
		writeJSON(w, http.StatusCreated, map[string]any{"license": license, "key": key})
	}
}

func (s *Service) revokeLicense(resolve UserResolver) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		_, org, role, ok := s.authorize(w, r, resolve)
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
		w.WriteHeader(http.StatusNoContent)
	}
}

func (s *Service) enrollNode(w http.ResponseWriter, r *http.Request) {
	var input struct {
		LicenseKey string          `json:"license_key"`
		NetworkID  uuid.UUID       `json:"network_id"`
		Name       string          `json:"name"`
		Resources  model.Resources `json:"resources"`
	}
	if json.NewDecoder(http.MaxBytesReader(w, r.Body, 8192)).Decode(&input) != nil || input.LicenseKey == "" || input.NetworkID == uuid.Nil || strings.TrimSpace(input.Name) == "" {
		writeError(w, http.StatusBadRequest, "invalid enrollment request")
		return
	}
	sum := sha256.Sum256([]byte(strings.ToUpper(strings.TrimSpace(input.LicenseKey))))
	hash := hex.EncodeToString(sum[:])
	var created model.Node
	err := s.db.Transaction(func(tx *gorm.DB) error {
		var license model.License
		if err := tx.Where("key_hash = ?", hash).First(&license).Error; err != nil || !LicenseUsable(license, model.Now()) {
			return errors.New("invalid license")
		}
		var network model.Network
		if err := tx.Where("id = ? AND organization_id = ? AND status = ?", input.NetworkID, license.OrganizationID, "ACTIVE").First(&network).Error; err != nil {
			return errors.New("invalid network")
		}
		rights, _, err := entitlementsFor(tx, license.OrganizationID)
		if err != nil {
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
		return tx.Model(&license).Update("last_used_at", now).Error
	})
	if err != nil {
		writeError(w, http.StatusForbidden, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, created)
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
