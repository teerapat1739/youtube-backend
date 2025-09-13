package service

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"be-v2/internal/config"
	"be-v2/pkg/logger"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSupabaseClient_FetchAccumulateSlots(t *testing.T) {
	tests := []struct {
		name           string
		serverResponse interface{}
		serverStatus   int
		requestBody    map[string]interface{}
		expectedResult *SupabaseAccumulateResponse
		expectedError  bool
		errorContains  string
	}{
		{
			name: "successful fetch with increments",
			serverResponse: map[string]interface{}{
				"increments": map[string]int{
					"1": 100,
					"2": 200,
					"3": 300,
				},
				"total": 600,
			},
			serverStatus: http.StatusOK,
			requestBody:  nil,
			expectedResult: &SupabaseAccumulateResponse{
				Increments: map[string]int{
					"1": 100,
					"2": 200,
					"3": 300,
				},
				Total: 600,
			},
			expectedError: false,
		},
		{
			name: "successful fetch with request body",
			serverResponse: map[string]interface{}{
				"increments": map[string]int{
					"1": 150,
				},
				"total": 150,
			},
			serverStatus: http.StatusOK,
			requestBody: map[string]interface{}{
				"total_visits":  100,
				"unique_visits": 50,
			},
			expectedResult: &SupabaseAccumulateResponse{
				Increments: map[string]int{
					"1": 150,
				},
				Total: 150,
			},
			expectedError: false,
		},
		{
			name:           "server returns 500 error",
			serverResponse: "Internal Server Error",
			serverStatus:   http.StatusInternalServerError,
			requestBody:    nil,
			expectedResult: nil,
			expectedError:  true,
			errorContains:  "Supabase function returned status 500",
		},
		{
			name:           "server returns invalid JSON",
			serverResponse: "not a json",
			serverStatus:   http.StatusOK,
			requestBody:    nil,
			expectedResult: nil,
			expectedError:  true,
			errorContains:  "failed to parse Supabase response",
		},
		{
			name: "empty response",
			serverResponse: map[string]interface{}{
				"increments": map[string]int{},
				"total":      0,
			},
			serverStatus: http.StatusOK,
			requestBody:  nil,
			expectedResult: &SupabaseAccumulateResponse{
				Increments: map[string]int{},
				Total:      0,
			},
			expectedError: false,
		},
		{
			name:           "server timeout simulation",
			serverResponse: nil,
			serverStatus:   http.StatusRequestTimeout,
			requestBody:    nil,
			expectedResult: nil,
			expectedError:  true,
			errorContains:  "Supabase function returned status 408",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test server
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Verify request method
				assert.Equal(t, "POST", r.Method)
				assert.Equal(t, "/functions/v1/accumulate-slots", r.URL.Path)
				assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
				assert.Contains(t, r.Header.Get("Authorization"), "Bearer")

				// Send response
				w.WriteHeader(tt.serverStatus)
				if tt.serverResponse != nil {
					if str, ok := tt.serverResponse.(string); ok {
						w.Write([]byte(str))
					} else {
						json.NewEncoder(w).Encode(tt.serverResponse)
					}
				}
			}))
			defer server.Close()

			// Create config with test server URL
			cfg := &config.Config{
				SupabaseURL:     server.URL,
				SupabaseAnonKey: "test-key",
			}

			// Create logger
			log, _ := logger.New("info")

			// Create client
			client := NewSupabaseClient(cfg, log)

			// Execute test
			ctx := context.Background()
			result, err := client.FetchAccumulateSlots(ctx, tt.requestBody)

			// Assert results
			if tt.expectedError {
				require.Error(t, err)
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
				}
				assert.Nil(t, result)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expectedResult, result)
			}
		})
	}
}

func TestSupabaseClient_NetworkError(t *testing.T) {
	// Test network error scenario
	cfg := &config.Config{
		SupabaseURL:     "http://invalid-url-that-does-not-exist.local",
		SupabaseAnonKey: "test-key",
	}

	log, _ := logger.New("info")
	client := NewSupabaseClient(cfg, log)

	ctx := context.Background()
	result, err := client.FetchAccumulateSlots(ctx, nil)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to call Supabase function")
	assert.Nil(t, result)
}

func TestSupabaseClient_ContextCancellation(t *testing.T) {
	// Create a server that delays response
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// This handler will never complete
		select {}
	}))
	defer server.Close()

	cfg := &config.Config{
		SupabaseURL:     server.URL,
		SupabaseAnonKey: "test-key",
	}

	log, _ := logger.New("info")
	client := NewSupabaseClient(cfg, log)

	// Create context with immediate cancellation
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	result, err := client.FetchAccumulateSlots(ctx, nil)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to call Supabase function")
	assert.Nil(t, result)
}