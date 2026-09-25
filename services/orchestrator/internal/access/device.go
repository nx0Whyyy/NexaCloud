package access

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/nexastudio/nexacloud/pkg/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const deviceCodeLifetime = 10 * time.Minute

func (s *Service) createDeviceCode(w http.ResponseWriter, r *http.Request) {
	if !s.deviceLimit.allow("code:" + clientIP(r)) {
		writeError(w, http.StatusTooManyRequests, "device code rate limit reached")
		return
	}
	var input struct {
		Name      string          `json:"name"`
		PublicKey string          `json:"public_key"`
		Resources model.Resources `json:"resources"`
	}
	if json.NewDecoder(http.MaxBytesReader(w, r.Body, 16384)).Decode(&input) != nil || strings.TrimSpace(input.Name) == "" || len(strings.TrimSpace(input.PublicKey)) < 32 {
		writeError(w, http.StatusBadRequest, "node name and public key are required")
		return
	}
	code, err := randomDisplayCode()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "device code unavailable")
		return
	}
	token, err := randomToken(32)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "device code unavailable")
		return
	}
	now := model.Now()
	enrollment := model.DeviceEnrollment{
		ID: model.NewID(), CodeHash: hashSecret(code), DeviceTokenHash: hashSecret(token),
		NodeName: strings.TrimSpace(input.Name), PublicKey: strings.TrimSpace(input.PublicKey),
		Resources: input.Resources, Status: "PENDING", CreatedAt: now, ExpiresAt: now.Add(deviceCodeLifetime),
	}
	if err := s.db.Create(&enrollment).Error; err != nil {
		writeError(w, http.StatusInternalServerError, "device code unavailable")
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{
		"device_code": token, "user_code": code, "verification_uri": "https://cloud.nexastudio.dev/device", "expires_in": int(deviceCodeLifetime.Seconds()),
	})
}

func (s *Service) approveDeviceEnrollment(resolve UserResolver) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !validMutationOrigin(r) {
			writeError(w, http.StatusForbidden, "origine refusée")
			return
		}
		user, org, role, ok := s.authorize(w, r, resolve)
		if !ok {
			return
		}
		if !s.deviceLimit.allow("approve:" + user.ID.String()) {
			writeError(w, http.StatusTooManyRequests, "activation rate limit reached")
			return
		}
		if role != "owner" && role != "admin" && role != "infrastructure_admin" {
			writeError(w, http.StatusForbidden, "node.manage permission required")
			return
		}
		var input struct {
			Code          string    `json:"code"`
			NetworkID     uuid.UUID `json:"network_id"`
			PublicAddress string    `json:"public_address"`
		}
		if json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&input) != nil {
			writeError(w, http.StatusBadRequest, "invalid enrollment request")
			return
		}
		input.PublicAddress = strings.TrimSpace(input.PublicAddress)
		if input.Code == "" || input.NetworkID == uuid.Nil || !validNodeAddress(input.PublicAddress) {
			writeError(w, http.StatusBadRequest, "code, network_id and a valid server IP are required")
			return
		}
		var node model.Node
		err := s.db.Transaction(func(tx *gorm.DB) error {
			var enrollment model.DeviceEnrollment
			if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("code_hash = ? AND status = ? AND expires_at > ?", hashSecret(strings.ToUpper(strings.TrimSpace(input.Code))), "PENDING", model.Now()).First(&enrollment).Error; err != nil {
				return errors.New("device code is invalid or expired")
			}
			var network model.Network
			if err := tx.Where("id = ? AND organization_id = ? AND status = ?", input.NetworkID, org.ID, "ACTIVE").First(&network).Error; err != nil {
				return errors.New("network not found")
			}
			var subscription model.Subscription
			if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("organization_id = ?", org.ID).First(&subscription).Error; err != nil {
				return err
			}
			var rights Entitlements
			if err := json.Unmarshal([]byte(subscription.EntitlementData), &rights); err != nil {
				return err
			}
			var count int64
			if err := tx.Model(&model.Node{}).Where("organization_id = ? AND status <> ?", org.ID, model.NodeRevoked).Count(&count).Error; err != nil {
				return err
			}
			if int(count) >= rights.Limits["max_nodes"] {
				return errors.New("node quota reached")
			}
			now := model.Now()
			node = model.Node{ID: model.NewID(), OrganizationID: &org.ID, NetworkID: &network.ID, Name: enrollment.NodeName, PublicAddress: input.PublicAddress, Status: model.NodeOffline, Labels: model.Labels{}, Resources: enrollment.Resources, CreatedAt: now, UpdatedAt: now}
			if err := tx.Create(&node).Error; err != nil {
				return err
			}
			fingerprint := sha256.Sum256([]byte(enrollment.PublicKey))
			credential := model.NodeCredential{ID: model.NewID(), NodeID: node.ID, PublicKey: enrollment.PublicKey, Fingerprint: hex.EncodeToString(fingerprint[:]), Status: "ACTIVE", CreatedAt: now}
			if err := tx.Create(&credential).Error; err != nil {
				return err
			}
			enrollment.Status, enrollment.NodeID, enrollment.ApprovedAt = "APPROVED", &node.ID, &now
			if err := tx.Save(&enrollment).Error; err != nil {
				return err
			}
			return tx.Create(&model.AuditEntry{ID: model.NewID(), OrganizationID: &org.ID, Actor: user.ID.String(), Action: "NODE_ENROLL", ResourceType: "node", ResourceID: node.ID.String(), IPAddress: clientIP(r), Result: "success", Details: model.JSON(map[string]any{"network_id": network.ID}), CreatedAt: now}).Error
		})
		if err != nil {
			writeError(w, http.StatusConflict, safeAccessError(err))
			return
		}
		writeJSON(w, http.StatusCreated, node)
	}
}

func validNodeAddress(value string) bool {
	ip := net.ParseIP(strings.TrimSpace(value))
	return ip != nil && !ip.IsLoopback() && !ip.IsUnspecified() && !ip.IsMulticast()
}

func (s *Service) claimDeviceEnrollment(w http.ResponseWriter, r *http.Request) {
	var input struct {
		DeviceCode string `json:"device_code"`
	}
	if json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&input) != nil || input.DeviceCode == "" {
		writeError(w, http.StatusBadRequest, "device_code is required")
		return
	}
	var enrollment model.DeviceEnrollment
	var credential model.NodeCredential
	var agentToken string
	err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("device_token_hash = ? AND expires_at > ?", hashSecret(input.DeviceCode), model.Now()).First(&enrollment).Error; err != nil {
			return err
		}
		if enrollment.Status != "APPROVED" || enrollment.NodeID == nil || enrollment.ClaimedAt != nil {
			return nil
		}
		if err := tx.Where("node_id = ? AND status = ?", *enrollment.NodeID, "ACTIVE").First(&credential).Error; err != nil {
			return err
		}
		var err error
		agentToken, err = randomToken(32)
		if err != nil {
			return err
		}
		if err := tx.Model(&credential).Update("secret_hash", hashSecret(agentToken)).Error; err != nil {
			return err
		}
		now := model.Now()
		enrollment.Status, enrollment.ClaimedAt = "CLAIMED", &now
		return tx.Save(&enrollment).Error
	})
	if err != nil {
		writeError(w, http.StatusGone, "device code is invalid or expired")
		return
	}
	if enrollment.Status == "PENDING" {
		writeJSON(w, http.StatusAccepted, map[string]string{"status": "authorization_pending"})
		return
	}
	if enrollment.Status != "CLAIMED" || enrollment.NodeID == nil || credential.ID == uuid.Nil {
		writeError(w, http.StatusGone, "device enrollment is no longer available")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "approved", "node_id": enrollment.NodeID, "credential_id": credential.ID, "fingerprint": credential.Fingerprint, "agent_token": agentToken})
}

func (s *Service) agentHeartbeat(w http.ResponseWriter, r *http.Request) {
	credential, ok := s.authenticateAgent(w, r)
	if !ok {
		return
	}
	if !s.heartbeatLimit.allow("heartbeat:" + credential.ID.String()) {
		writeError(w, http.StatusTooManyRequests, "heartbeat rate limit reached")
		return
	}
	var heartbeat model.AgentHeartbeat
	if json.NewDecoder(http.MaxBytesReader(w, r.Body, 1024*1024)).Decode(&heartbeat) != nil {
		writeError(w, http.StatusBadRequest, "heartbeat invalid")
		return
	}
	now := model.Now()
	var node model.Node
	if err := s.db.Where("id = ? AND status <> ?", credential.NodeID, model.NodeRevoked).First(&node).Error; err != nil {
		writeError(w, http.StatusGone, "node revoked or unavailable")
		return
	}
	node.Status = model.NodeOnline
	node.Usage = heartbeat.Usage
	node.Containers = heartbeat.Containers
	node.Minecraft = heartbeat.Minecraft
	node.LastHeartbeat = now
	node.UpdatedAt = now
	if heartbeat.AgentVersion != "" {
		node.AgentVersion = heartbeat.AgentVersion
	}
	if heartbeat.Resources.Memory != "" || heartbeat.Resources.CPU > 0 {
		node.Resources = heartbeat.Resources
	}
	if err := s.db.Save(&node).Error; err != nil {
		writeError(w, http.StatusGone, "node revoked or unavailable")
		return
	}
	writeJSON(w, http.StatusOK, model.AgentHeartbeatResponse{Status: "ok", NodeID: credential.NodeID, ReceivedAt: now})
}

func (s *Service) authenticateAgent(w http.ResponseWriter, r *http.Request) (model.NodeCredential, bool) {
	authorization := strings.TrimSpace(r.Header.Get("Authorization"))
	if !strings.HasPrefix(authorization, "Bearer ") {
		writeError(w, http.StatusUnauthorized, "agent credential required")
		return model.NodeCredential{}, false
	}
	token := strings.TrimSpace(strings.TrimPrefix(authorization, "Bearer "))
	if len(token) < 32 {
		writeError(w, http.StatusUnauthorized, "agent credential invalid")
		return model.NodeCredential{}, false
	}
	var credential model.NodeCredential
	if err := s.db.Where("secret_hash = ? AND status = ? AND revoked_at IS NULL", hashSecret(token), "ACTIVE").First(&credential).Error; err != nil {
		writeError(w, http.StatusUnauthorized, "agent credential invalid")
		return model.NodeCredential{}, false
	}
	return credential, true
}

func (s *Service) revokeNode(resolve UserResolver) http.HandlerFunc {
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
			writeError(w, http.StatusForbidden, "node.manage permission required")
			return
		}
		nodeID, err := uuid.Parse(r.PathValue("id"))
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid node id")
			return
		}
		err = s.db.Transaction(func(tx *gorm.DB) error {
			result := tx.Model(&model.Node{}).Where("id = ? AND organization_id = ?", nodeID, org.ID).Update("status", model.NodeRevoked)
			if result.Error != nil || result.RowsAffected == 0 {
				return gorm.ErrRecordNotFound
			}
			now := model.Now()
			if err := tx.Model(&model.NodeCredential{}).Where("node_id = ? AND revoked_at IS NULL", nodeID).Updates(map[string]any{"status": "REVOKED", "revoked_at": now}).Error; err != nil {
				return err
			}
			return tx.Create(&model.AuditEntry{ID: model.NewID(), OrganizationID: &org.ID, Actor: user.ID.String(), Action: "NODE_REVOKE", ResourceType: "node", ResourceID: nodeID.String(), IPAddress: clientIP(r), Result: "success", CreatedAt: now}).Error
		})
		if err != nil {
			writeError(w, http.StatusNotFound, "node not found")
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func randomDisplayCode() (string, error) {
	const alphabet = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"
	raw := make([]byte, 6)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	for i := range raw {
		raw[i] = alphabet[int(raw[i])%len(alphabet)]
	}
	return "NXA-" + string(raw[:3]) + "-" + string(raw[3:]), nil
}

func randomToken(size int) (string, error) {
	raw := make([]byte, size)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(raw), nil
}

func hashSecret(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}

func clientIP(r *http.Request) string {
	forwarded := strings.Split(r.Header.Get("X-Forwarded-For"), ",")
	for i := len(forwarded) - 1; i >= 0; i-- {
		if value := strings.TrimSpace(forwarded[i]); net.ParseIP(value) != nil {
			return value
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil {
		return host
	}
	return strings.TrimSpace(r.RemoteAddr)
}
