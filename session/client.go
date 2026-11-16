package session

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"golang.org/x/sync/errgroup"
)

// Client represents a RouterOS API client.
type Client struct {
	httpClient *http.Client // HTTP client used to communicate with the API.
	baseURL    *url.URL
	username   string // username for the request to Mikrotik Router
	password   string // password for the request to Mikrotik Router
	userAgent  string
}

// NewClient creates a new RouterOS API client. Host must be
// a valid url string (http://host:port) or a host:port string. Otherwise
// an error is returned.
func NewClient(host string, opts ...Option) (*Client, error) {
	baseURL, ok := buildURL(host)
	if !ok {
		return nil, fmt.Errorf("could not parse as URL or host:port: %s", host)
	}
	c := &Client{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout:   5 * time.Second,
			Transport: http.DefaultTransport,
		},
		userAgent: defaultUserAgent,
	}
	for _, o := range opts {
		o.apply(c)
	}
	return c, nil
}

// Auth authenticates to the RouterOS device by performing a GET on system/resource.
func (c *Client) Auth(ctx context.Context) error {
	r := &Request[any]{
		Command: "system/resource",
	}
	return c.doRequest(ctx, http.MethodGet, r)
}

func (c *Client) Export(ctx context.Context) ([]string, error) {
	r := &Request[any]{
		Command: "export",
		Payload: map[string]string{
			"compact": "",
			"file":    "backup.rsc",
		},
	}
	if err := c.doRequest(ctx, http.MethodPost, r); err != nil {
		return nil, err
	}
	type export struct {
		Data string `json:"data"`
	}

	r2 := &Request[[]export]{
		Command: "file/read",
		Payload: map[string]string{
			"file":       "backup.rsc",
			"offset":     "0",
			"chunk-size": "32768",
		},
	}
	if err := c.doRequest(ctx, http.MethodPost, r2); err != nil {
		return nil, err
	}
	if len(r2.Result) > 0 {
		config := r2.Result[0].Data
		return strings.Split(config, "\r\n"), nil
	}
	return nil, fmt.Errorf("invalid result: %v", r2.Result)
}

// Print creates a new GET request and executes it, returning the result and error.
func (c *Client) Print(ctx context.Context, r Requester) error {
	return c.doRequest(ctx, http.MethodGet, r)
}

// Add creates a new PUT request and executes it, returning the result and error.
func (c *Client) Add(ctx context.Context, r Requester) error {
	return c.doRequest(ctx, http.MethodPut, r)
}

// Set creates a new PATCH request and executes it, returning the result and error.
func (c *Client) Set(ctx context.Context, r Requester) error {
	return c.doRequest(ctx, http.MethodPatch, r)
}

// Remove creates a new DELETE request and executes it, returning the result and error.
func (c *Client) Remove(ctx context.Context, r Requester) error {
	return c.doRequest(ctx, http.MethodDelete, r)
}

// Run creates a new POST request and executes it, returning the result and error.
func (c *Client) Run(ctx context.Context, r Requester) error {
	return c.doRequest(ctx, http.MethodPost, r)
}

// Execute runs a sequence of requests concurrently using the given HTTP method.
func (c *Client) execute(ctx context.Context, method string, reqs ...Requester) error {
	g, cctx := errgroup.WithContext(ctx)
	g.SetLimit(10)
	for _, req := range reqs {
		req := req
		g.Go(func() error {
			return c.doRequest(cctx, method, req)
		})
	}
	return g.Wait()
}

func buildURL(host string) (*url.URL, bool) {
	u, err := url.Parse(host)
	if err == nil && u.Scheme != "" && u.Host != "" {
		return u, true
	}
	return &url.URL{
		Scheme: "https",
		Host:   host,
	}, true
}
