package session

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestIsValidHTTPMethod(t *testing.T) {
	tests := []struct {
		name     string
		method   string
		expected bool
	}{
		{"GET method", http.MethodGet, true},
		{"POST method", http.MethodPost, true},
		{"PUT method", http.MethodPut, true},
		{"PATCH method", http.MethodPatch, true},
		{"DELETE method", http.MethodDelete, true},
		{"Invalid OPTIONS method", http.MethodOptions, false},
		{"Invalid HEAD method", http.MethodHead, false},
		{"Lowercase get", "get", false},
		{"Empty string", "", false},
		{"Random string", "INVALID", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isValidHTTPMethod(tt.method)
			if got != tt.expected {
				t.Errorf("isValidHTTPMethod(%q) = %v; want %v", tt.method, got, tt.expected)
			}
		})
	}
}

func TestCreateRequestBody(t *testing.T) {
	t.Run("nil payload returns nil", func(t *testing.T) {
		got, err := createRequestBody(nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != nil {
			t.Errorf("expected nil reader, got %v", got)
		}
	})

	t.Run("non-empty payload returns valid reader", func(t *testing.T) {
		payload := map[string]string{"name": "test"}
		got, err := createRequestBody(payload)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got == nil {
			t.Fatalf("expected reader, got nil")
		}
		data, err := io.ReadAll(got)
		if err != nil {
			t.Fatalf("unexpected read error: %v", err)
		}
		expected := `{"name":"test"}`
		if string(data) != expected {
			t.Errorf("got content %q, want %q", string(data), expected)
		}
	})
}

func TestHandleHTTPError(t *testing.T) {
	t.Run("nil response", func(t *testing.T) {
		err := handleHTTPError(nil)
		if err == nil || err.Error() != "nil HTTP response" {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("nil response body", func(t *testing.T) {
		resp := &http.Response{StatusCode: 404, Body: nil}
		err := handleHTTPError(resp)
		if err == nil || err.Error() != "nil HTTP response body" {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("valid error response with body", func(t *testing.T) {
		bodyText := "resource not found"
		resp := &http.Response{
			StatusCode: http.StatusNotFound,
			Body:       io.NopCloser(strings.NewReader(bodyText)),
		}
		err := handleHTTPError(resp)
		httpErr, ok := err.(*HTTPError)
		if !ok {
			t.Fatalf("expected *HTTPError, got %T", err)
		}
		if httpErr.Code != 404 || httpErr.Body != bodyText {
			t.Errorf("got code %d body %q; want 404 %q", httpErr.Code, httpErr.Body, bodyText)
		}
		expectedMsg := "HTTP Not Found: resource not found"
		if httpErr.Error() != expectedMsg {
			t.Errorf("got Error() %q; want %q", httpErr.Error(), expectedMsg)
		}
	})
}

type mockCloser struct {
	closed bool
	err    error
}

func (m *mockCloser) Read(p []byte) (n int, err error) { return 0, io.EOF }
func (m *mockCloser) Close() error {
	m.closed = true
	return m.err
}

func TestCloseResponseBody(t *testing.T) {
	t.Run("closes body successfully", func(t *testing.T) {
		mc := &mockCloser{}
		closeResponseBody(mc)
		if !mc.closed {
			t.Errorf("expected Close() to be called")
		}
	})

	t.Run("handles close error gracefully", func(t *testing.T) {
		mc := &mockCloser{err: errors.New("close failed")}
		closeResponseBody(mc)
		if !mc.closed {
			t.Errorf("expected Close() to be called")
		}
	})
}

func TestDoRequest(t *testing.T) {
	t.Run("invalid HTTP method", func(t *testing.T) {
		client, err := NewClient("http://127.0.0.1")
		if err != nil {
			t.Fatalf("failed to create client: %v", err)
		}
		req := &Request[any]{
			Command: "system/resource",
		}
		err = client.doRequest(context.Background(), "INVALID", req)
		if err == nil || !strings.Contains(err.Error(), "invalid HTTP method") {
			t.Errorf("expected invalid HTTP method error, got %v", err)
		}
	})

	t.Run("successful GET request with mock server", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			user, pass, ok := r.BasicAuth()
			if !ok || user != "admin" || pass != "secret" {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			if r.Header.Get("Content-Type") != "application/json" {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`[{"name":"ether1"}]`))
		}))
		defer server.Close()

		client, err := NewClient(server.URL, WithCredentials("admin", "secret"))
		if err != nil {
			t.Fatalf("failed to create client: %v", err)
		}
		client.httpClient = server.Client()

		type iface struct {
			Name string `json:"name"`
		}
		req := &Request[[]iface]{
			Command: "interface",
		}
		err = client.doRequest(context.Background(), http.MethodGet, req)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(req.Result) != 1 || req.Result[0].Name != "ether1" {
			t.Fatalf("expected [{ether1}], got %+v", req.Result)
		}
	})

	t.Run("HTTP non-2xx status code returns HTTPError", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte("not found"))
		}))
		defer server.Close()

		client, err := NewClient(server.URL)
		if err != nil {
			t.Fatalf("failed to create client: %v", err)
		}
		client.httpClient = server.Client()

		req := &Request[any]{
			Command: "unknown",
		}
		err = client.doRequest(context.Background(), http.MethodGet, req)
		if err == nil {
			t.Fatalf("expected error, got nil")
		}
		if _, ok := err.(*HTTPError); !ok {
			t.Errorf("expected *HTTPError, got %T: %v", err, err)
		}
	})
}
