package main

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"time"
)

type agentClient struct {
	baseURL *url.URL
	http    *http.Client
}

func newAgentClient(cfg config) (*agentClient, error) {
	baseURL, err := url.Parse(cfg.AgentURL)
	if err != nil {
		return nil, err
	}
	if baseURL.Scheme != "https" {
		return nil, errors.New("AGENT_URL must use https")
	}
	caPEM, err := os.ReadFile(cfg.AgentCAFile)
	if err != nil {
		return nil, fmt.Errorf("read agent CA: %w", err)
	}
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(caPEM) {
		return nil, errors.New("agent CA contains no certificate")
	}
	certificate, err := tls.LoadX509KeyPair(cfg.AgentCertFile, cfg.AgentKeyFile)
	if err != nil {
		return nil, fmt.Errorf("load controller mTLS certificate: %w", err)
	}
	transport := &http.Transport{
		TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS13, RootCAs: pool, Certificates: []tls.Certificate{certificate}},
		MaxIdleConns:    20, IdleConnTimeout: 30 * time.Second,
	}
	return &agentClient{baseURL: baseURL, http: &http.Client{Transport: transport, Timeout: 15 * time.Minute}}, nil
}

func (c *agentClient) health(ctx context.Context) error {
	return c.do(ctx, http.MethodGet, "/v1/health", nil, nil)
}

func (c *agentClient) nodeMetrics(ctx context.Context) (map[string]any, error) {
	var out map[string]any
	err := c.do(ctx, http.MethodGet, "/v1/node/metrics", nil, &out)
	return out, err
}

func (c *agentClient) serverAction(ctx context.Context, serverID, action string, input, output any) error {
	return c.do(ctx, http.MethodPost, "/v1/servers/"+url.PathEscape(serverID)+"/"+action, input, output)
}

func (c *agentClient) state(ctx context.Context, serverID string) (agentState, error) {
	return c.serverState(ctx, serverID, true)
}

func (c *agentClient) readiness(ctx context.Context, serverID string) (agentState, error) {
	return c.serverState(ctx, serverID, false)
}

func (c *agentClient) serverState(ctx context.Context, serverID string, includeMetrics bool) (agentState, error) {
	var out agentState
	query := url.Values{"metrics": {fmt.Sprint(includeMetrics)}}
	err := c.do(ctx, http.MethodGet, "/v1/servers/"+url.PathEscape(serverID)+"/state?"+query.Encode(), nil, &out)
	return out, err
}

func (c *agentClient) logs(ctx context.Context, serverID string, since time.Time, tail int) (string, error) {
	var out struct {
		Logs string `json:"logs"`
	}
	query := url.Values{"tail": {fmt.Sprint(tail)}}
	if !since.IsZero() {
		query.Set("since", since.UTC().Format(time.RFC3339Nano))
	}
	err := c.do(ctx, http.MethodGet, "/v1/servers/"+url.PathEscape(serverID)+"/logs?"+query.Encode(), nil, &out)
	return out.Logs, err
}

func (c *agentClient) followLogs(ctx context.Context, serverID string, since time.Time) (io.ReadCloser, error) {
	query := url.Values{"follow": {"true"}}
	if !since.IsZero() {
		query.Set("since", since.UTC().Format(time.RFC3339Nano))
	}
	relative, err := url.Parse("/v1/servers/" + url.PathEscape(serverID) + "/logs?" + query.Encode())
	if err != nil {
		return nil, err
	}
	u := *c.baseURL
	u.Path = path.Join(c.baseURL.Path, relative.Path)
	u.RawQuery = relative.RawQuery
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}
	streamClient := *c.http
	streamClient.Timeout = 0
	resp, err := streamClient.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		defer resp.Body.Close()
		data, readErr := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
		if readErr != nil {
			return nil, readErr
		}
		var apiErr apiError
		if json.Unmarshal(data, &apiErr) == nil && apiErr.Error != "" {
			return nil, fmt.Errorf("agent: %s", apiErr.Error)
		}
		return nil, fmt.Errorf("agent returned HTTP %d", resp.StatusCode)
	}
	return resp.Body, nil
}

func (c *agentClient) command(ctx context.Context, serverID, command string) (string, error) {
	var out struct {
		Output string `json:"output"`
	}
	err := c.serverAction(ctx, serverID, "command", map[string]string{"command": command}, &out)
	return out.Output, err
}

func (c *agentClient) files(ctx context.Context, serverID, filePath string) (json.RawMessage, error) {
	var out json.RawMessage
	query := url.Values{"path": []string{filePath}}
	err := c.do(ctx, http.MethodGet, "/v1/servers/"+url.PathEscape(serverID)+"/files?"+query.Encode(), nil, &out)
	return out, err
}

func (c *agentClient) writeFile(ctx context.Context, serverID string, input any) (json.RawMessage, error) {
	var out json.RawMessage
	err := c.do(ctx, http.MethodPut, "/v1/servers/"+url.PathEscape(serverID)+"/files", input, &out)
	return out, err
}

func (c *agentClient) deleteFile(ctx context.Context, serverID, filePath string) error {
	query := url.Values{"path": []string{filePath}}
	return c.do(ctx, http.MethodDelete, "/v1/servers/"+url.PathEscape(serverID)+"/files?"+query.Encode(), nil, nil)
}

func (c *agentClient) createFolder(ctx context.Context, serverID, filePath string) error {
	return c.do(ctx, http.MethodPost, "/v1/servers/"+url.PathEscape(serverID)+"/files/folders", map[string]string{"path": filePath}, nil)
}

func (c *agentClient) moveFile(ctx context.Context, serverID, from, to string) error {
	return c.do(ctx, http.MethodPost, "/v1/servers/"+url.PathEscape(serverID)+"/files/move", map[string]string{"from": from, "to": to}, nil)
}

func (c *agentClient) deleteBackup(ctx context.Context, serverID, backupID string) error {
	return c.do(ctx, http.MethodDelete, "/v1/servers/"+url.PathEscape(serverID)+"/backups/"+url.PathEscape(backupID), nil, nil)
}

func (c *agentClient) downloadBackup(ctx context.Context, serverID, backupID string) (io.ReadCloser, int64, error) {
	relative, err := url.Parse("/v1/servers/" + url.PathEscape(serverID) + "/backups/" + url.PathEscape(backupID) + "/download")
	if err != nil {
		return nil, 0, err
	}
	u := *c.baseURL
	u.Path = path.Join(c.baseURL.Path, relative.Path)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, 0, err
	}
	response, err := c.http.Do(req)
	if err != nil {
		return nil, 0, err
	}
	if response.StatusCode != http.StatusOK {
		response.Body.Close()
		return nil, 0, fmt.Errorf("agent returned HTTP %d", response.StatusCode)
	}
	return response.Body, response.ContentLength, nil
}

func (c *agentClient) do(ctx context.Context, method, requestPath string, input, output any) error {
	var body io.Reader
	if input != nil {
		data, err := json.Marshal(input)
		if err != nil {
			return err
		}
		body = bytes.NewReader(data)
	}
	relative, err := url.Parse(requestPath)
	if err != nil {
		return err
	}
	u := *c.baseURL
	u.Path = path.Join(c.baseURL.Path, relative.Path)
	u.RawQuery = relative.RawQuery
	req, err := http.NewRequestWithContext(ctx, method, u.String(), body)
	if err != nil {
		return err
	}
	if input != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		return err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var apiErr apiError
		if json.Unmarshal(data, &apiErr) == nil && apiErr.Error != "" {
			return fmt.Errorf("agent: %s", apiErr.Error)
		}
		return fmt.Errorf("agent returned HTTP %d", resp.StatusCode)
	}
	if output != nil && len(data) > 0 {
		return json.Unmarshal(data, output)
	}
	return nil
}
