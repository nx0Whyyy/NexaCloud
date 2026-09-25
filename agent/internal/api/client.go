package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/nexastudio/nexacloud/pkg/model"
)

type Client struct {
	baseURL string
	http    *http.Client
}
type DeviceAuthorization struct {
	DeviceCode      string `json:"device_code"`
	UserCode        string `json:"user_code"`
	VerificationURI string `json:"verification_uri"`
	ExpiresIn       int    `json:"expires_in"`
}
type DeviceToken struct {
	Status      string `json:"status"`
	NodeID      string `json:"node_id"`
	AgentToken  string `json:"agent_token"`
	Fingerprint string `json:"fingerprint"`
}

func New(baseURL string) *Client {
	return &Client{baseURL: strings.TrimRight(baseURL, "/"), http: &http.Client{Timeout: 15 * time.Second}}
}
func (c *Client) CreateDeviceCode(ctx context.Context, name, publicKey string, resources model.Resources) (*DeviceAuthorization, error) {
	var result DeviceAuthorization
	err := c.request(ctx, http.MethodPost, "/api/v1/device/code", "", map[string]any{"name": name, "public_key": publicKey, "resources": resources}, &result)
	return &result, err
}
func (c *Client) ClaimDevice(ctx context.Context, code string) (*DeviceToken, int, error) {
	var result DeviceToken
	status, err := c.requestStatus(ctx, http.MethodPost, "/api/v1/device/token", "", map[string]string{"device_code": code}, &result)
	return &result, status, err
}
func (c *Client) Heartbeat(ctx context.Context, token string, heartbeat model.AgentHeartbeat) (*model.AgentHeartbeatResponse, error) {
	var result model.AgentHeartbeatResponse
	err := c.request(ctx, http.MethodPost, "/api/v1/agent/heartbeat", token, heartbeat, &result)
	return &result, err
}
func (c *Client) NextCommand(ctx context.Context, token string) (*model.AgentCommand, error) {
	var result model.AgentCommand
	status, err := c.requestStatus(ctx, http.MethodPost, "/api/v1/agent/commands/next", token, map[string]string{}, &result)
	if status == http.StatusNoContent {
		return nil, nil
	}
	return &result, err
}
func (c *Client) CompleteCommand(ctx context.Context, token string, id string, success bool, result, commandError string) error {
	return c.request(ctx, http.MethodPost, "/api/v1/agent/commands/"+id+"/result", token, map[string]any{"success": success, "result": result, "error": commandError}, nil)
}
func (c *Client) request(ctx context.Context, method, path, token string, input, output any) error {
	_, err := c.requestStatus(ctx, method, path, token, input, output)
	return err
}
func (c *Client) requestStatus(ctx context.Context, method, path, token string, input, output any) (int, error) {
	body, err := json.Marshal(input)
	if err != nil {
		return 0, err
	}
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, bytes.NewReader(body))
	if err != nil {
		return 0, err
	}
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var failure struct {
			Error string `json:"error"`
		}
		_ = json.NewDecoder(resp.Body).Decode(&failure)
		if failure.Error == "" {
			failure.Error = resp.Status
		}
		return resp.StatusCode, fmt.Errorf("control plane: %s", failure.Error)
	}
	if resp.StatusCode == http.StatusNoContent {
		return resp.StatusCode, nil
	}
	if output != nil {
		return resp.StatusCode, json.NewDecoder(resp.Body).Decode(output)
	}
	return resp.StatusCode, nil
}
