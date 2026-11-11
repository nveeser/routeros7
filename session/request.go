package session

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
)

type Request[T any] struct {
	Command string
	Payload any
	Result  T
}

type Requester interface {
	command() string
	payload() any
	decode(r io.Reader) error
}

func (r *Request[T]) command() string { return r.Command }
func (r *Request[T]) payload() any    { return r.Payload }
func (r *Request[T]) decode(body io.Reader) error {
	return json.NewDecoder(body).Decode(&r.Result)
}

func (c *Client) doRequest(ctx context.Context, method string, r Requester) error {
	// Check if the HTTP method is valid
	if !isValidHTTPMethod(method) {
		return fmt.Errorf("mustRequest: invalid HTTP method: %s", method) // Return an error
	}

	body, err := createRequestBody(r.payload())
	if err != nil {
		return err // Return error
	}
	request, err := http.NewRequestWithContext(ctx, method, c.url(r.command()), body)
	if err != nil {
		return err // Return error
	}

	request.SetBasicAuth(c.username, c.password) // Set BasicAuth on the request
	request.Header.Set("Content-Type", "application/json")

	response, err := c.httpClient.Do(request)
	if err != nil {
		return err // Return error
	}

	defer closeResponseBody(response.Body)
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return handleHTTPError(response)
	}
	if err := r.decode(response.Body); err != nil {
		return fmt.Errorf("error decoding body: %w", err)
	}
	return nil
}

// url constructs the REST URL for a given command.
func (c *Client) url(command string) string {
	x := *c.baseURL
	x.Path = fmt.Sprintf("/rest/%s", command)
	// Set RawPath so that url.EscapedPath() does not encode *1
	x.RawPath = x.Path
	return x.String()
}

func isValidHTTPMethod(method string) bool {
	return method == http.MethodGet || method == http.MethodPost ||
		method == http.MethodPut || method == http.MethodPatch ||
		method == http.MethodDelete
}

// createRequestBody function is used to create the request body from the payload byte array.
// If the payload is not empty, it returns a bytes.Buffer containing the payload.
// If the payload is empty, it returns nil.
func createRequestBody(payload any) (io.Reader, error) {
	if payload == nil {
		return nil, nil
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("createRequestBody: %w", err)
	}
	// Check if the payload is not empty
	if len(data) > 0 {
		return bytes.NewBuffer(data), nil // Return a bytes.Buffer containing the payload
	}
	// Return nil
	return nil, nil
}

// closeResponseBody function is used to close the response body and log the error if any error occurs.
func closeResponseBody(body io.ReadCloser) {
	err := body.Close() // Close the response body

	// Check if there is an error while closing the response body
	if err != nil {
		log.Println(err)
	}
}

// handleHTTPError function is used to handle the HTTP error and return the error message and response body.
// If any error occurs while reading the response body, it logs the error and returns the HTTP error message.
func handleHTTPError(response *http.Response) error {
	// Check if the response is nil
	if response == nil {
		return fmt.Errorf("nil HTTP response") // Return an error
	}

	// Check if the response body is nil
	if response.Body == nil {
		return fmt.Errorf("nil HTTP response body") // Return an error
	}

	// Read the response body
	body, err := io.ReadAll(response.Body)
	// Check if there is an error while reading the response body
	if err != nil {
		log.Println(err) // Log the error
	}

	return &HTTPError{Code: response.StatusCode, Body: string(body)}
}

type HTTPError struct {
	Code int
	Body string
}

func (h *HTTPError) Error() string {
	return fmt.Sprintf("HTTP %s: %s", http.StatusText(h.Code), h.Body)
}
