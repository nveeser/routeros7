package routerosv7

import (
	"net"
	"testing"
)

// TestIsHostAvailableOnPort_NotAvailable tests the isHostAvailableOnPort function.
// It is not possible to test the actual connection to the host, if the host is not available.
func TestIsHostAvailableOnPort_NotAvailable(t *testing.T) {
	// Set the dialerInstance to use the mockDialer for testing

	defer func() {}()

	// Use a non-existent port for testing
	available := isHostAvailableOnPort("localhost", "9999")
	if available {
		t.Error("Expected host to be not available, got true")
	}
}

// TestIsHostAvailableOnPort_Available tests the isHostAvailableOnPort function.
// It is not possible to test the actual connection to the host, if the host is not available.
func TestIsHostAvailableOnPort_Available(t *testing.T) {
	// Create a listener to simulate a server on localhost:8080
	listener, err := net.Listen("tcp", "localhost:8080")
	if err != nil {
		t.Error("Failed to create a listener for coverage")
		return
	}

	// Close the listener when the test is done
	defer func(listener net.Listener) {
		err := listener.Close()
		if err != nil {
			t.Error("Failed to close the listener for coverage")
		}
	}(listener)

	// Set the dialerInstance to use the defaultDialer for testing
	defer func() {}()

	// Use an existing port for testing
	available := isHostAvailableOnPort("localhost", "8080")
	if !available {
		t.Error("Expected host to be available, got false")
	}
}
