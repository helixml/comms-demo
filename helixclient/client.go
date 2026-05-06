// Package helixclient sends Message envelopes to Helix Org's
// /webhooks/<stream_id> endpoint.
package helixclient

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/helixml/comms-demo/domain"
)

// Client posts Message envelopes to Helix Org. Construct via New.
type Client struct {
	http *http.Client
}

// New returns a Client with the supplied http client (or http.DefaultClient
// equivalent if nil — tests can wire a custom RoundTripper).
func New(h *http.Client) *Client {
	if h == nil {
		h = &http.Client{Timeout: 10 * time.Second}
	}
	return &Client{http: h}
}

// Send POSTs m to <baseURL>/webhooks/<streamID>. A 2xx response means Helix
// Org appended the event. Returns an error on non-2xx including the response
// body (truncated).
func (c *Client) Send(ctx context.Context, baseURL, streamID string, m *domain.Message) error {
	if baseURL == "" {
		return errors.New("helixclient: empty base URL")
	}
	if streamID == "" {
		return errors.New("helixclient: empty stream id")
	}
	if m == nil {
		return errors.New("helixclient: nil message")
	}

	u, err := buildURL(baseURL, streamID)
	if err != nil {
		return err
	}

	body, err := json.Marshal(m)
	if err != nil {
		return fmt.Errorf("helixclient: marshal: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("helixclient: new request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("helixclient: post %s: %w", u, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		snippet, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("helixclient: %s returned %d: %s", u, resp.StatusCode, strings.TrimSpace(string(snippet)))
	}
	return nil
}

func buildURL(base, streamID string) (string, error) {
	u, err := url.Parse(strings.TrimRight(base, "/"))
	if err != nil {
		return "", fmt.Errorf("helixclient: parse base url: %w", err)
	}
	if u.Scheme == "" || u.Host == "" {
		return "", fmt.Errorf("helixclient: base url %q must include scheme and host", base)
	}
	u.Path = strings.TrimRight(u.Path, "/") + "/webhooks/" + url.PathEscape(streamID)
	return u.String(), nil
}
