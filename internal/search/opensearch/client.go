// Package opensearch provides the small connectivity and naming foundation
// BookDB uses for its rebuildable search projection.
package opensearch

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// Client is a minimal OpenSearch HTTP client.
type Client struct {
	baseURL    *url.URL
	httpClient *http.Client
}

// New constructs a client with the provided base URL and timeout.
func New(rawURL string, timeout time.Duration) (*Client, error) {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("opensearch: parse url: %w", err)
	}
	if parsed.Scheme == "" || parsed.Host == "" {
		return nil, fmt.Errorf("opensearch: url must include scheme and host")
	}
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	return &Client{
		baseURL: parsed,
		httpClient: &http.Client{
			Timeout: timeout,
		},
	}, nil
}

// Check probes the cluster health endpoint.
func (c *Client) Check(ctx context.Context) error {
	if c == nil || c.baseURL == nil || c.httpClient == nil {
		return fmt.Errorf("opensearch: client is nil")
	}
	reqURL := c.endpoint("_cluster/health")
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return fmt.Errorf("opensearch: create request: %w", err)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("opensearch: health check: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("opensearch: health check returned %s", resp.Status)
	}
	return nil
}

// IndexName returns the versioned physical index name used by BookDB.
func IndexName(entity string, version int) string {
	return "bookdb_" + normalize(entity) + "_v" + strconv.Itoa(version)
}

// AliasName returns the stable alias pointing to a versioned index.
func AliasName(entity string) string {
	return "bookdb_" + normalize(entity)
}

func (c *Client) endpoint(path string) string {
	clone := *c.baseURL
	clone.Path = strings.TrimRight(c.baseURL.Path, "/") + "/" + strings.TrimLeft(path, "/")
	return clone.String()
}

func normalize(s string) string {
	s = strings.TrimSpace(strings.ToLower(s))
	var b strings.Builder
	b.Grow(len(s))
	for i := 0; i < len(s); i++ {
		ch := s[i]
		switch {
		case ch >= 'a' && ch <= 'z':
			b.WriteByte(ch)
		case ch >= '0' && ch <= '9':
			b.WriteByte(ch)
		case ch == ' ' || ch == '_' || ch == '-' || ch == '.':
			b.WriteByte('_')
		}
	}
	return strings.Trim(b.String(), "_")
}
