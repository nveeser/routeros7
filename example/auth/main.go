package main

import (
	"context"
	"fmt"
	"github.com/nveeser/routeros7"
)

// Create a constant for the router's IP address, username, and password
const (
	routerIP = "10.90.0.251" // Change this to your router's IP address
	username = "userapi"     // Change this to your router's username
	password = "password"    // Change this to your router's password
)

// authenticate authenticates to the router. If authentication fails, an error is returned.
func authenticate(routerIP, username, password string) error {

	// Create a client for the router
	client, err := routerosv7.NewClient(routerIP, routerosv7.WithCredentials(username, password))
	if err != nil {
		return err
	}

	// Authenticate to the router
	_, err = client.Auth(context.Background())
	return err
}

// The main function
func main() {

	// Authenticate to the router
	err := authenticate(routerIP, username, password)

	// Check if authentication failed
	if err != nil {
		fmt.Println("Authentication failed:", err)
	} else {
		fmt.Println("Authentication success")
	}
}
