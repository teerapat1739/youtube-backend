package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"be-v2/internal/config"
	"be-v2/pkg/logger"
)

// SupabaseAccumulateResponse represents the response from Supabase accumulate-slots function
type SupabaseAccumulateResponse struct {
	Increments map[string]int `json:"increments"`
	Total      int            `json:"total"`
}

// SupabaseClient handles all interactions with Supabase functions
type SupabaseClient struct {
	config     *config.Config
	httpClient *http.Client
	logger     *logger.Logger
}

// NewSupabaseClient creates a new Supabase client
func NewSupabaseClient(cfg *config.Config, logger *logger.Logger) *SupabaseClient {
	return &SupabaseClient{
		config: cfg,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
		logger: logger,
	}
}

// FetchAccumulateSlots calls the Supabase accumulate-slots function
// requestBody can be nil for just fetching, or contain data to update
func (s *SupabaseClient) FetchAccumulateSlots(ctx context.Context, requestBody map[string]interface{}) (*SupabaseAccumulateResponse, error) {
	// Use empty body if nil
	if requestBody == nil {
		requestBody = map[string]interface{}{}
	}

	jsonBody, err := json.Marshal(requestBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request body: %w", err)
	}

	// Create the request to Supabase function
	url := fmt.Sprintf("%s/functions/v1/accumulate-slots", s.config.SupabaseURL)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", s.config.SupabaseAnonKey))
	req.Header.Set("Content-Type", "application/json")

	// Make the request
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to call Supabase function: %w", err)
	}
	defer resp.Body.Close()

	// Read the response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	// Check for non-200 status codes
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Supabase function returned status %d: %s", resp.StatusCode, string(body))
	}

	// Parse the response
	var supabaseResp SupabaseAccumulateResponse
	if err := json.Unmarshal(body, &supabaseResp); err != nil {
		s.logger.WithFields(map[string]interface{}{
			"response_body": string(body),
			"status_code":   resp.StatusCode,
		}).Error("Failed to parse Supabase response")
		return nil, fmt.Errorf("failed to parse Supabase response: %w", err)
	}

	s.logger.WithFields(map[string]interface{}{
		"total":      supabaseResp.Total,
		"increments": supabaseResp.Increments,
	}).Debug("Successfully fetched Supabase accumulate slots")

	return &supabaseResp, nil
}