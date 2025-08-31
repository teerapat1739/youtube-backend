package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
)

func main() {
	// Get the Google token from environment or use a test token
	token := os.Getenv("GOOGLE_ACCESS_TOKEN")
	if token == "" {
		fmt.Println("Please set GOOGLE_ACCESS_TOKEN environment variable with a valid Google access token")
		fmt.Println("You can get one from the browser DevTools when logged in to the frontend")
		return
	}

	// Prepare the vote request
	voteData := map[string]interface{}{
		"team_id": 1,
		"personal_info": map[string]string{
			"first_name": "Test",
			"last_name":  "User",
			"email":      "test@example.com",
			"phone":      "081-234-5678",
		},
		"consent": map[string]interface{}{
			"pdpa_consent":            true,
			"marketing_consent":       false,
			"privacy_policy_version": "1.0",
		},
	}

	jsonData, _ := json.Marshal(voteData)

	// Create the request
	req, err := http.NewRequest("POST", "http://localhost:8080/api/v1/voting/vote", bytes.NewBuffer(jsonData))
	if err != nil {
		fmt.Printf("Error creating request: %v\n", err)
		return
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	// Send the request
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		fmt.Printf("Error sending request: %v\n", err)
		return
	}
	defer resp.Body.Close()

	// Read response
	body, _ := io.ReadAll(resp.Body)

	// Parse and display response
	var result map[string]interface{}
	json.Unmarshal(body, &result)

	fmt.Printf("Status: %d\n", resp.StatusCode)
	fmt.Printf("Response: %s\n", string(body))

	if resp.StatusCode == 200 || resp.StatusCode == 201 {
		fmt.Println("\n✅ Authentication successful! Token validation is working correctly.")
	} else if resp.StatusCode == 401 {
		fmt.Println("\n❌ Authentication failed. Token might be invalid or expired.")
		if errorData, ok := result["error"].(map[string]interface{}); ok {
			fmt.Printf("Error details: %v\n", errorData["message"])
		}
	}
}