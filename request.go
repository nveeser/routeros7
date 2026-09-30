package routerosv7

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
)

// requestConfig represents a request to the API.
type requestConfig struct {
	path    string
	method  string // Method for the request to Mikrotik Router
	payload []byte // Payload for the request to Mikrotik Router
}

func (c *Client) doRequest(ctx context.Context, config requestConfig) (interface{}, error) {
	// Check if the HTTP method is valid
	if !isValidHTTPMethod(config.method) {
		return nil, fmt.Errorf("makeRequest: invalid HTTP method: %s", config.method) // Return an error
	}

	body := createRequestBody(config.payload)
	request, err := http.NewRequestWithContext(ctx, config.method, c.url(config.path), body)
	if err != nil {
		return nil, err // Return nil and error
	}

	request.SetBasicAuth(c.username, c.password) // Set BasicAuth on the request
	request.Header.Set("Content-Type", "application/json")

	response, err := c.httpClient.Do(request)
	if err != nil {
		return nil, err // Return nil and error
	}

	defer closeResponseBody(response.Body)
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, handleHTTPError(response) // Return nil and error
	}
	var responseData interface{}
	if err := json.NewDecoder(response.Body).Decode(&responseData); err != nil {
		return nil, err // Return nil and error
	}
	return responseData, nil
}

// url constructs the REST URL for a given command.
func (c *Client) url(command string) string {
	x := *c.baseURL
	x.Path = fmt.Sprintf("rest/%s", command)
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
func createRequestBody(payload []byte) io.Reader {
	// Check if the payload is not empty
	if len(payload) > 0 {
		return bytes.NewBuffer(payload) // Return a bytes.Buffer containing the payload
	}
	// Return nil
	return nil
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

//// retryTlsErrorRequest retries the request after modifying the URL to use HTTP instead of HTTPS.
//func retryTlsErrorRequest(httpClient *http.Client, request *http.Request, config requestConfig) (
//	*http.Response, error,
//) {
//	// Replace the URL protocol from https to http
//	config.URL = replaceProtocol(config.bas, httpsProtocol, httpProtocol)
//
//	// Create a new HTTP request with the provided context, method, URL, body, username and password
//	request.URL, _ = parseURL(config.URL)
//
//	// Send the HTTP request and return the response and error
//	return httpClient.Do(request)
//}
