package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

type Client struct {
	baseURL    string
	httpClient *http.Client
}

func New(httpClient *http.Client, baseURL string) *Client {
	if httpClient == nil {
		httpClient = &http.Client{}
	}
	return &Client{
		baseURL:    baseURL,
		httpClient: httpClient,
	}
}

func (c *Client) Get(path string, resp any) error {
	return c.do("GET", path, nil, resp)
}

func (c *Client) Post(path string, body, resp any) error {
	return c.do("POST", path, body, resp)
}

func (c *Client) do(method, path string, reqBody, resp any) error {
	var bodyReader io.Reader
	if reqBody != nil {
		data, err := json.Marshal(reqBody)
		if err != nil {
			return fmt.Errorf("marshal body: %w", err)
		}
		bodyReader = bytes.NewReader(data)
	}

	req, err := http.NewRequest(method, c.baseURL+path, bodyReader)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	if reqBody != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Accept", "application/json")

	httpResp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer httpResp.Body.Close()

	raw, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return fmt.Errorf("read response: %w", err)
	}

	if httpResp.StatusCode >= 400 {
		return fmt.Errorf("api error: %s (%d)", string(raw), httpResp.StatusCode)
	}

	if resp != nil && len(raw) > 0 {
		if err := json.Unmarshal(raw, resp); err != nil {
			return fmt.Errorf("unmarshal response: %w", err)
		}
	}

	return nil
}
