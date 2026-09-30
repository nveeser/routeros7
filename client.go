// client.go

package routerosv7

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"net/url"
	"time"
)

// Client represents a VyOS API client.
type Client struct {
	httpClient *http.Client // HTTP client used to communicate with the API.
	baseURL    *url.URL
	username   string // username for the request to Mikrotik Router
	password   string // password for the request to Mikrotik Router
	userAgent  string
}

// NewClient creates a new VyOS API client. Host must be
// a value url string (http://host:port) or a host:port string. Otherwise
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
		o(c)
	}
	return c, nil
}

// Auth authenticates to the RouterOS device by performing a GET on system/resource.
func (c *Client) Auth(ctx context.Context) (interface{}, error) {
	r := requestConfig{
		path:   "system/resource", // Set the URL
		method: MethodGet,         // Use MethodGet
	}
	return c.doRequest(ctx, r)
}

// Print creates a new GET request and executes it, returning the result and error.
func (c *Client) Print(ctx context.Context, command string) (interface{}, error) {
	r := requestConfig{
		path:   command,   // Set the URL
		method: MethodGet, // Set the method
	}
	return c.doRequest(ctx, r)
}

// Add creates a new PUT request and executes it, returning the result and error.
func (c *Client) Add(ctx context.Context, command string, payload []byte) (interface{}, error) {
	r := requestConfig{
		path:    command,   // Set the UR
		method:  MethodPut, // Set the method
		payload: payload,   // Set the payload
	}
	return c.doRequest(ctx, r)
}

// Set creates a new PATCH request and executes it, returning the result and error.
func (c *Client) Set(ctx context.Context, command string, payload []byte) (interface{}, error) {
	r := requestConfig{
		path:    command,     // Set the URL
		method:  MethodPatch, // Set the method
		payload: payload,     // Set the payload
	}
	return c.doRequest(ctx, r)
}

// Remove creates a new DELETE request and executes it, returning the result and error.
func (c *Client) Remove(ctx context.Context, command string) (interface{}, error) {
	r := requestConfig{
		path:   command,      // Set the URL
		method: MethodDelete, // Set the method
	}
	return c.doRequest(ctx, r)
}

// Run creates a new POST request and executes it, returning the result and error.
func (c *Client) Run(ctx context.Context, command string, payload []byte) (interface{}, error) {
	r := requestConfig{
		path:    command,    // Set the URL
		method:  MethodPost, // Set the method
		payload: payload,    // Set the payload
	}
	return c.doRequest(ctx, r)
}

// Option is the type passed to NewClient to configure the client
type Option func(*Client)

// Insecure enables http.Transport.TLSClientConfig.InsecureSkipVerify
// for calling hosts with self-signed certificates.
func Insecure() Option {
	return func(c *Client) {
		transport := c.httpClient.Transport
	loop:
		switch x := transport.(type) {
		case *http.Transport:
			if x.TLSClientConfig == nil {
				x.TLSClientConfig = &tls.Config{}
			}
			x.TLSClientConfig.InsecureSkipVerify = true

		case nil:
			c.httpClient.Transport = &http.Transport{}
			goto loop
		}
	}
}

// UserAgent sets the User-Agent.
func UserAgent(s string) Option {
	return func(c *Client) {
		c.userAgent = s
	}
}

// Timeout sets the default timeout for the http.Client.
func Timeout(d time.Duration) Option {
	return func(c *Client) {
		c.httpClient.Timeout = d
	}
}

// WithHTTPClient updates the Client to use the provided HTTP client.
func WithHTTPClient(httpClient *http.Client) Option {
	return func(c *Client) {
		c.httpClient = httpClient
	}
}

// WithCredentials sets username and password credentials for basic authentication.
func WithCredentials(username, password string) Option {
	return func(c *Client) {
		c.username = username
		c.password = password
	}
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
