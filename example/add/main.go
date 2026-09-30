package main

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/nveeser/routeros7"
)

// Create constants for the default values for this example application
const (
	routerIP         = "10.90.0.251"     // Change this to your router's IP address
	username         = "userapi"         // Change this to your router's username
	password         = "password"        // Change this to your router's password
	payloadIpAddr    = "192.168.99.1/24" // Change this to the payload name for the IP address
	payloadInterface = "ether1"          // Change this to the payload name for the interface
)

// webResponse struct for web response data
type webResponse struct {
	Code   int         `json:"code"`   // HTTP status code
	Status string      `json:"status"` // HTTP status text
	Data   interface{} `json:"data"`   // Data to be sent
}

// checkIfAddressExists checks if the address already exists
func checkIfAddressExists(ctx context.Context, client *routerosv7.Client, address string) (bool, error) {

	cmd := "ip/address"

	// Calling Print method
	data, err := client.Print(ctx, cmd)

	if err != nil {
		return false, err
	}

	// Check if the data is an array of map[string]interface{}
	response, ok := data.([]interface{})
	if !ok {
		return false, fmt.Errorf("unexpected data format: %v", data)
	}

	for _, item := range response {
		if dataItem, ok := item.(map[string]interface{}); ok {
			if fieldValue, ok := dataItem["address"].(string); ok {
				if fieldValue == address {
					return true, nil // Address exists
				}
			}
		}
	}

	return false, nil // Address does not exist
}

// putAddress updates an address on the router and returns the response
func putAddress(
	ctx context.Context, client *routerosv7.Client, command string,
	payload []byte,
) (map[string]interface{}, error) {

	data, err := client.Add(ctx, command, payload)

	if err != nil {
		return nil, err
	}

	// Type assert the response to map[string]interface{} since that's what's expected
	response, ok := data.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("unexpected response format: %v", data)
	}

	return response, nil
}

// main function for this example application
func main() {

	// Create a client for the router
	client, err := routerosv7.NewClient(routerIP, routerosv7.WithCredentials(username, password))
	if err != nil {
		fmt.Println("Failed to create client:", err)
		return
	}

	// Check if the address already exists
	if exists, err := checkIfAddressExists(context.Background(), client, payloadIpAddr); err != nil {
		fmt.Println("Failed to check if address exists:", err)
		return
	} else if exists {
		// Create jsonError variable
		jsonError := webResponse{
			Code:   409,
			Status: "Conflict",
			Data:   fmt.Sprintf("Address %s already exists", payloadIpAddr),
		}

		// Marshal the jsonError to JSON
		jsonData, err := json.Marshal(jsonError)

		// Print error message if there is an error and return from this function
		if err != nil {
			fmt.Println("Failed to marshal JSON:", err)
			return
		}

		// Print the JSON string to the console
		fmt.Println(string(jsonData))
		return
	}

	// Create payload variable as []byte with the desired payload data
	payload := fmt.Sprintf(`{"address": "%s","interface": "%s"}`, payloadIpAddr, payloadInterface)

	// Add address with putAddress function and get the response data as map[string]interface{}
	// and print the response data to the console as JSON string
	response, err := putAddress(context.Background(), client, "ip/address", []byte(payload))
	if err != nil {
		fmt.Println("Failed to add address:", err)
		return
	}

	jsonSuccess := webResponse{
		Code:   200,
		Status: "OK",
		Data:   response,
	}

	// Marshal the jsonSuccess to JSON
	jsonData, err := json.Marshal(jsonSuccess)

	// Print error message if there is an error and return from this function
	if err != nil {
		fmt.Println("Failed to marshal JSON:", err)
		return
	}

	// Print the JSON string to the console
	fmt.Println(string(jsonData))

}
