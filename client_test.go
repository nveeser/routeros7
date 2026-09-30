package routerosv7

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestURL(t *testing.T) {
	client, err := NewClient("http://example.com", WithCredentials("user", "pass"))
	require.NoError(t, err)

	expectedURL := "http://example.com/rest/example"
	actualURL := client.url("example")

	assert.Equal(t, expectedURL, actualURL, "URL does not match expected")
}

func TestNewClientOptions(t *testing.T) {
	customHTTP := &http.Client{Timeout: 10 * time.Second}

	client, err := NewClient(
		"192.168.88.1",
		WithCredentials("admin", "password123"),
		UserAgent("custom-agent/1.0"),
		Timeout(2*time.Second),
		Insecure(),
		WithHTTPClient(customHTTP),
	)
	require.NoError(t, err)

	assert.Equal(t, "https://192.168.88.1", client.baseURL.String())
	assert.Equal(t, "admin", client.username)
	assert.Equal(t, "password123", client.password)
	assert.Equal(t, "custom-agent/1.0", client.userAgent)
	assert.Equal(t, customHTTP, client.httpClient)

	// Test Insecure on default http.Transport
	c2, err := NewClient("192.168.88.1", Insecure())
	require.NoError(t, err)
	transport, ok := c2.httpClient.Transport.(*http.Transport)
	require.True(t, ok)
	assert.True(t, transport.TLSClientConfig.InsecureSkipVerify)
}

func TestBuildURL(t *testing.T) {
	tests := []struct {
		input          string
		expectedScheme string
		expectedHost   string
	}{
		{"http://10.0.0.1:8080", "http", "10.0.0.1:8080"},
		{"https://router.local", "https", "router.local"},
		{"192.168.1.1", "https", "192.168.1.1"},
		{"192.168.1.1:8728", "https", "192.168.1.1:8728"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			u, ok := buildURL(tt.input)
			assert.True(t, ok)
			assert.Equal(t, tt.expectedScheme, u.Scheme)
			assert.Equal(t, tt.expectedHost, u.Host)
		})
	}
}

func TestClientAPIMethods(t *testing.T) {
	var capturedMethod string
	var capturedPath string
	var capturedBody string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedMethod = r.Method
		capturedPath = r.URL.Path
		bodyBytes, _ := io.ReadAll(r.Body)
		capturedBody = string(bodyBytes)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"success"}`))
	}))
	defer server.Close()

	client, err := NewClient(server.URL, WithCredentials("admin", "pass"))
	require.NoError(t, err)
	client.httpClient = server.Client()

	ctx := context.Background()

	t.Run("Auth", func(t *testing.T) {
		resp, err := client.Auth(ctx)
		require.NoError(t, err)
		assert.NotNil(t, resp)
		assert.Equal(t, http.MethodGet, capturedMethod)
		assert.Equal(t, "/rest/system/resource", capturedPath)
	})

	t.Run("Print", func(t *testing.T) {
		resp, err := client.Print(ctx, "interface")
		require.NoError(t, err)
		assert.NotNil(t, resp)
		assert.Equal(t, http.MethodGet, capturedMethod)
		assert.Equal(t, "/rest/interface", capturedPath)
	})

	t.Run("Add", func(t *testing.T) {
		payload := []byte(`{"name":"ether2"}`)
		resp, err := client.Add(ctx, "interface", payload)
		require.NoError(t, err)
		assert.NotNil(t, resp)
		assert.Equal(t, http.MethodPut, capturedMethod)
		assert.Equal(t, "/rest/interface", capturedPath)
		assert.Equal(t, string(payload), capturedBody)
	})

	t.Run("Set", func(t *testing.T) {
		payload := []byte(`{"comment":"disabled"}`)
		resp, err := client.Set(ctx, "interface/ether1", payload)
		require.NoError(t, err)
		assert.NotNil(t, resp)
		assert.Equal(t, http.MethodPatch, capturedMethod)
		assert.Equal(t, "/rest/interface/ether1", capturedPath)
		assert.Equal(t, string(payload), capturedBody)
	})

	t.Run("Remove", func(t *testing.T) {
		resp, err := client.Remove(ctx, "interface/*1")
		require.NoError(t, err)
		assert.NotNil(t, resp)
		assert.Equal(t, http.MethodDelete, capturedMethod)
		assert.Equal(t, "/rest/interface/*1", capturedPath)
	})

	t.Run("Run", func(t *testing.T) {
		payload := []byte(`{"target":"8.8.8.8"}`)
		resp, err := client.Run(ctx, "ping", payload)
		require.NoError(t, err)
		assert.NotNil(t, resp)
		assert.Equal(t, http.MethodPost, capturedMethod)
		assert.Equal(t, "/rest/ping", capturedPath)
		assert.Equal(t, string(payload), capturedBody)
	})
}
