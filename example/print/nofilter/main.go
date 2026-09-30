package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/nveeser/routeros7"
)

// Create constants for the default values for this example application
const (
	routerIP = "10.90.0.251" // Change this to your router's IP address
	username = "userapi"     // Change this to your router's username
	password = "password"    // Change this to your router's password
	command  = "ip/address"  // Change this to the command you want to execute
)

// AppConfig struct for this example application configuration values
type AppConfig struct {
	Client  *routerosv7.Client // RouterOS client
	Command string             // Command to execute
}

// NewAppConfig function to create new AppConfig instance with default values for this example application
func NewAppConfig(client *routerosv7.Client, command string) *AppConfig {
	return &AppConfig{
		Client:  client,
		Command: command,
	}
}

// DataRetriever interface for retrieving data from RouterOS device
type DataRetriever interface {
	GetData(ctx context.Context) (interface{}, error)
}

// RouterOSDataRetriever struct for retrieving data from RouterOS device
type RouterOSDataRetriever struct {
	config *AppConfig
}

// NewRouterOSDataRetriever function to create new RouterOSDataRetriever instance
func NewRouterOSDataRetriever(config *AppConfig) DataRetriever {
	return &RouterOSDataRetriever{config: config}
}

// GetData function to retrieve data from RouterOS device using the config values from the RouterOSDataRetriever
// instance that called this function and return the data as interface{} and error
func (r *RouterOSDataRetriever) GetData(ctx context.Context) (interface{}, error) {

	// Calling Print method on the client
	data, err := r.config.Client.Print(ctx, command)

	// Checking for errors
	if err != nil {
		log.Fatalf("Error: %v", err)
	}

	// Return the data as interface{} and nil error
	return data, nil
}

// PrintJSON function to nofilter the data as JSON to the console
func PrintJSON(data interface{}) {

	// Check if the data is not []interface{} type
	response, ok := data.([]interface{})

	// Print error message if the data is not []interface{} type and return from this function
	if !ok {
		fmt.Println("Failed to process data")
		return
	}

	// Process and nofilter the data as JSON
	var resultJSON []map[string]interface{}

	// Iterate over the response and append the data to ipAddresses
	for _, item := range response {
		if dataItem, ok := item.(map[string]interface{}); ok {
			resultJSON = append(resultJSON, dataItem)
		}
	}

	// Marshal the resultJSON to JSON string and nofilter it to the console
	jsonData, err := json.Marshal(map[string]interface{}{
		"code":   200,
		"status": "OK",
		"data":   resultJSON,
	})

	// Print error message if there is an error and return from this function
	if err != nil {
		fmt.Println("Failed to marshal JSON:", err)
		return
	}

	// Print the JSON string to the console
	fmt.Println(string(jsonData))
}

// Main function for this example application
func main() {

	// Create a client for the router
	client, err := routerosv7.NewClient(routerIP, routerosv7.WithCredentials(username, password))
	if err != nil {
		fmt.Println("Failed to create client:", err)
		return
	}

	// Authenticate to the router
	if _, err := client.Auth(context.Background()); err != nil {
		fmt.Println("Authentication failed:", err)
		return
	}

	// Create new AppConfig instance with the client and command
	config := NewAppConfig(client, command)

	// Create new RouterOSDataRetriever instance with the config values from the AppConfig instance
	dataRetriever := NewRouterOSDataRetriever(config)

	// Retrieve data from RouterOS device using the config values from the RouterOSDataRetriever instance
	data, err := dataRetriever.GetData(context.Background())
	if err != nil {
		fmt.Println("Failed to retrieve data:", err)
		return
	}

	// Print the data as JSON to the console
	PrintJSON(data)
}
