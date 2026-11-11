package session

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
		err := client.Auth(ctx)
		require.NoError(t, err)
		assert.Equal(t, http.MethodGet, capturedMethod)
		assert.Equal(t, "/rest/system/resource", capturedPath)
	})

	t.Run("Print", func(t *testing.T) {
		req := &Request[map[string]string]{Command: "interface"}
		err := client.Print(ctx, req)
		require.NoError(t, err)
		assert.Equal(t, "success", req.Result["status"])
		assert.Equal(t, http.MethodGet, capturedMethod)
		assert.Equal(t, "/rest/interface", capturedPath)
	})

	t.Run("Add", func(t *testing.T) {
		req := &Request[map[string]string]{
			Command: "interface",
			Payload: map[string]string{"name": "ether2"},
		}
		err := client.Add(ctx, req)
		require.NoError(t, err)
		assert.Equal(t, "success", req.Result["status"])
		assert.Equal(t, http.MethodPut, capturedMethod)
		assert.Equal(t, "/rest/interface", capturedPath)
		assert.Equal(t, `{"name":"ether2"}`, capturedBody)
	})

	t.Run("Set", func(t *testing.T) {
		req := &Request[map[string]string]{
			Command: "interface/ether1",
			Payload: map[string]string{"comment": "disabled"},
		}
		err := client.Set(ctx, req)
		require.NoError(t, err)
		assert.Equal(t, "success", req.Result["status"])
		assert.Equal(t, http.MethodPatch, capturedMethod)
		assert.Equal(t, "/rest/interface/ether1", capturedPath)
		assert.Equal(t, `{"comment":"disabled"}`, capturedBody)
	})

	t.Run("Remove", func(t *testing.T) {
		req := &Request[map[string]string]{Command: "interface/*1"}
		err := client.Remove(ctx, req)
		require.NoError(t, err)
		assert.Equal(t, "success", req.Result["status"])
		assert.Equal(t, http.MethodDelete, capturedMethod)
		assert.Equal(t, "/rest/interface/*1", capturedPath)
	})

	t.Run("Run", func(t *testing.T) {
		req := &Request[map[string]string]{
			Command: "ping",
			Payload: map[string]string{"target": "8.8.8.8"},
		}
		err := client.Run(ctx, req)
		require.NoError(t, err)
		assert.Equal(t, "success", req.Result["status"])
		assert.Equal(t, http.MethodPost, capturedMethod)
		assert.Equal(t, "/rest/ping", capturedPath)
		assert.Equal(t, `{"target":"8.8.8.8"}`, capturedBody)
	})
}
