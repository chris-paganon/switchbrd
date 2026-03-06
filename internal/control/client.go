package control

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"path"
	"time"

	"dev-switchboard/internal/app"
)

type Client struct {
	httpClient *http.Client
	baseURL    string
}

func NewClient(socket string) *Client {
	transport := &http.Transport{
		DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
			var d net.Dialer
			return d.DialContext(ctx, "unix", socket)
		},
	}
	return &Client{
		httpClient: &http.Client{Timeout: 3 * time.Second, Transport: transport},
		baseURL:    "http://switchboard",
	}
}

func (c *Client) Health(ctx context.Context) error {
	resp, err := c.do(ctx, http.MethodGet, "/health", nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status: %s", resp.Status)
	}
	return nil
}

func (c *Client) Add(ctx context.Context, port int, name string) (app.App, error) {
	resp, err := c.do(ctx, http.MethodPost, "/apps", addRequest{Name: name, Port: port})
	if err != nil {
		return app.App{}, err
	}
	defer resp.Body.Close()
	if err := decodeError(resp); err != nil {
		return app.App{}, err
	}
	var payload appResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return app.App{}, err
	}
	return payload.App, nil
}

func (c *Client) List(ctx context.Context) ([]app.App, string, error) {
	resp, err := c.do(ctx, http.MethodGet, "/apps", nil)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()
	if err := decodeError(resp); err != nil {
		return nil, "", err
	}
	var payload listResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, "", err
	}
	return payload.Apps, payload.ActiveName, nil
}

func (c *Client) Activate(ctx context.Context, target string, name string) (*app.App, error) {
	resp, err := c.do(ctx, http.MethodPut, "/active", activateRequest{Target: target, Name: name})
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if err := decodeError(resp); err != nil {
		return nil, err
	}
	var payload activeResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, err
	}
	return payload.App, nil
}

func (c *Client) Rename(ctx context.Context, oldName string, newName string) (app.App, error) {
	resp, err := c.do(ctx, http.MethodPut, "/rename", renameRequest{OldName: oldName, NewName: newName})
	if err != nil {
		return app.App{}, err
	}
	defer resp.Body.Close()
	if err := decodeError(resp); err != nil {
		return app.App{}, err
	}
	var payload appResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return app.App{}, err
	}
	return payload.App, nil
}

func (c *Client) Active(ctx context.Context) (*app.App, error) {
	resp, err := c.do(ctx, http.MethodGet, "/active", nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if err := decodeError(resp); err != nil {
		return nil, err
	}
	var payload activeResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, err
	}
	return payload.App, nil
}

func (c *Client) Remove(ctx context.Context, name string) error {
	resp, err := c.do(ctx, http.MethodDelete, path.Join("/apps", url.PathEscape(name)), nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return decodeError(resp)
}

func (c *Client) do(ctx context.Context, method string, requestPath string, payload any) (*http.Response, error) {
	var body *bytes.Reader
	if payload == nil {
		body = bytes.NewReader(nil)
	} else {
		encoded, err := json.Marshal(payload)
		if err != nil {
			return nil, err
		}
		body = bytes.NewReader(encoded)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+requestPath, body)
	if err != nil {
		return nil, err
	}
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("switchboard is not running: %w", err)
	}
	return resp, nil
}

func decodeError(resp *http.Response) error {
	if resp.StatusCode < 400 {
		return nil
	}
	var payload errorResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err == nil && payload.Error != "" {
		return errors.New(payload.Error)
	}
	return fmt.Errorf("unexpected status: %s", resp.Status)
}
