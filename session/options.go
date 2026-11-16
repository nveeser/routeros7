package session

import (
	"crypto/tls"
	"fmt"
	"log"
	"net/http"
	"net/http/httputil"
	"time"
)

const (
	// defaultUserAgent is the default user agent used by the RouterOS API client.
	defaultUserAgent = "go-routeros7-rest"
)

// Option is the type passed to NewClient to configure the client
type Option interface {
	apply(*Client)
}

type OptionFn func(*Client)

func (f OptionFn) apply(c *Client) { f(c) }

// Insecure enables http.Transport.TLSClientConfig.InsecureSkipVerify
// for calling hosts with self-signed certificates.
func Insecure() OptionFn {
	return func(c *Client) {
		transport := c.httpClient.Transport
	loop:
		switch x := transport.(type) {
		case *http.Transport:
			if x.TLSClientConfig == nil {
				x.TLSClientConfig = &tls.Config{}
			}
			x.TLSClientConfig.InsecureSkipVerify = true

		case *loggingTransport:
			transport = x.RoundTripper
			goto loop

		case nil:
			c.httpClient.Transport = &http.Transport{}
			goto loop
		}
	}
}

// UserAgent sets the User-Agent.
func UserAgent(s string) OptionFn {
	return func(c *Client) {
		c.userAgent = s
	}
}

// Timeout sets the default timeout for the http.Client.
func Timeout(d time.Duration) OptionFn {
	return func(c *Client) {
		c.httpClient.Timeout = d
	}
}

// WithHTTPClient updates the Client to use the provided HTTP client.
func WithHTTPClient(httpClient *http.Client) OptionFn {
	return func(c *Client) {
		c.httpClient = httpClient
	}
}

// WithCredentials sets username and password credentials for basic authentication.
func WithCredentials(username, password string) OptionFn {
	return func(c *Client) {
		c.username = username
		c.password = password
	}
}

type DebugOption int

const (
	None DebugOption = iota
	LogPath
	LogRequest
	LogAll
)

func (o DebugOption) apply(c *Client) {
	c.httpClient.Transport = &loggingTransport{
		RoundTripper: c.httpClient.Transport,
		level:        o,
	}
}

// DebugLogging enables printing the HTTP Request and HTTP Response
// of each interaction for debugging.
func DebugLogging() Option {
	return LogAll
}

// loggingTransport wraps an http.RoundTripper to log HTTP request and response
// values.
type loggingTransport struct {
	http.RoundTripper
	level DebugOption
}

func (t *loggingTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	// Dump the request data including the body (true)
	var logRequest bool
	var logResponse bool
	switch t.level {
	case None:
		return t.RoundTripper.RoundTrip(r)
	case LogPath:
		logRequest = false
		logResponse = false
	case LogRequest:
		logRequest = true
		logResponse = false
	case LogAll:
		logRequest = true
		logResponse = true
	}

	if logRequest {
		reqDump, err := httputil.DumpRequestOut(r, true)
		if err != nil {
			log.Printf("Error dumping request: %v", err)
		} else {
			fmt.Printf("REQUEST:\n%s\n", reqDump)
		}
	} else {
		fmt.Printf("REQUEST: %s\n", r.URL.Path)
	}

	// Execute the actual request
	resp, err := t.RoundTripper.RoundTrip(r)
	if err != nil {
		// Log error and return
		log.Printf("Error making request: %v", err)
		return resp, err
	}

	if logResponse {
		// Dump the response data including the body (true)
		respDump, err := httputil.DumpResponse(resp, false)
		if err != nil {
			log.Printf("Error dumping response: %v", err)
		} else {
			fmt.Printf("RESPONSE:\n%s\n", respDump)
		}
	}
	return resp, err
}
