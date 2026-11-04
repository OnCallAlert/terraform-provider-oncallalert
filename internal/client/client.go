// Package client is a small GraphQL client for the OnCallAlert server API.
package client

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Client sends GraphQL requests to one OnCallAlert server.
type Client struct {
	endpoint  string
	token     string
	userAgent string
	http      *http.Client
}

// Config holds the settings needed to build a Client.
type Config struct {
	URL                string
	Token              string
	UserAgent          string
	Timeout            time.Duration
	InsecureSkipVerify bool
}

// New returns a Client for the server at cfg.URL.
func New(cfg Config) (*Client, error) {
	base := strings.TrimRight(strings.TrimSpace(cfg.URL), "/")
	if base == "" {
		return nil, errors.New("server URL is required")
	}
	if !strings.HasPrefix(base, "http://") && !strings.HasPrefix(base, "https://") {
		return nil, fmt.Errorf("server URL %q must start with http:// or https://", base)
	}
	if cfg.Token == "" {
		return nil, errors.New("API token is required")
	}
	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	tr := http.DefaultTransport.(*http.Transport).Clone()
	if cfg.InsecureSkipVerify {
		tr.TLSClientConfig = &tls.Config{InsecureSkipVerify: true} //nolint:gosec // opt-in for self-signed test servers
	}
	return &Client{
		endpoint:  base + "/api/graphql",
		token:     cfg.Token,
		userAgent: cfg.UserAgent,
		http:      &http.Client{Timeout: timeout, Transport: tr},
	}, nil
}

type gqlRequest struct {
	Query     string         `json:"query"`
	Variables map[string]any `json:"variables,omitempty"`
}

type gqlError struct {
	Message string                `json:"message"`
	Path    []any                 `json:"path"`
	Ext     struct{ Code string } `json:"extensions"`
}

type gqlResponse struct {
	Data   json.RawMessage `json:"data"`
	Errors []gqlError      `json:"errors"`
}

// Error is a GraphQL error returned by the server.
type Error struct {
	Messages []string
}

func (e *Error) Error() string { return "oncallalert API: " + strings.Join(e.Messages, "; ") }

// IsNotFound reports whether err means the requested object does not exist.
func IsNotFound(err error) bool {
	var ge *Error
	if !errors.As(err, &ge) {
		return false
	}
	for _, m := range ge.Messages {
		l := strings.ToLower(m)
		if strings.Contains(l, "not found") || strings.Contains(l, "does not exist") || strings.Contains(l, "no rows") {
			return true
		}
	}
	return false
}

// Do runs query with vars and decodes the "data" object into out (if out is not nil).
func (c *Client) Do(ctx context.Context, query string, vars map[string]any, out any) error {
	body, err := json.Marshal(gqlRequest{Query: query, Variables: vars})
	if err != nil {
		return fmt.Errorf("encode request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.token)
	if c.userAgent != "" {
		req.Header.Set("User-Agent", c.userAgent)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("request %s: %w", c.endpoint, err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 10<<20))
	if err != nil {
		return fmt.Errorf("read response: %w", err)
	}
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return fmt.Errorf("oncallalert API: %s: check the token and its scope (needs an Admin token)", resp.Status)
	}
	var gr gqlResponse
	if err := json.Unmarshal(raw, &gr); err != nil {
		return fmt.Errorf("oncallalert API: %s: unexpected response: %.200s", resp.Status, raw)
	}
	if len(gr.Errors) > 0 {
		e := &Error{}
		for _, ge := range gr.Errors {
			e.Messages = append(e.Messages, ge.Message)
		}
		return e
	}
	if out == nil || len(gr.Data) == 0 {
		return nil
	}
	if err := json.Unmarshal(gr.Data, out); err != nil {
		return fmt.Errorf("decode response data: %w", err)
	}
	return nil
}

// DeleteAll deletes the objects of one target type by ID. Missing objects are ignored.
func (c *Client) DeleteAll(ctx context.Context, targetType string, ids ...string) error {
	in := make([]map[string]string, 0, len(ids))
	for _, id := range ids {
		in = append(in, map[string]string{"id": id, "type": targetType})
	}
	err := c.Do(ctx, `mutation($in:[TargetInput!]){deleteAll(input:$in)}`, map[string]any{"in": in}, nil)
	if IsNotFound(err) {
		return nil
	}
	return err
}
