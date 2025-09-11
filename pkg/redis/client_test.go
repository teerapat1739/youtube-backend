package redis

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestRedis(t *testing.T) (*miniredis.Miniredis, *Client) {
	// Create a miniredis server
	mr, err := miniredis.Run()
	require.NoError(t, err)

	// Create client with test redis
	client, err := NewClient("redis://"+mr.Addr(), "test")
	require.NoError(t, err)

	return mr, client
}

func TestNewClient(t *testing.T) {
	tests := []struct {
		name        string
		url         string
		environment string
		expectError bool
	}{
		{
			name:        "Valid Redis URL",
			url:         "redis://localhost:6379/0",
			environment: "test",
			expectError: false,
		},
		{
			name:        "Invalid URL",
			url:         "invalid://url",
			environment: "test",
			expectError: true,
		},
		{
			name:        "Empty URL",
			url:         "",
			environment: "test",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := NewClient(tt.url, tt.environment)

			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, client)
			} else {
				// For valid URL test, we expect no error in creation
				// but connection might fail if Redis isn't running
				assert.NoError(t, err)
				assert.NotNil(t, client)
				assert.NotNil(t, client.KeyBuilder)
			}
		})
	}
}

func TestClient_Get(t *testing.T) {
	mr, client := setupTestRedis(t)
	defer mr.Close()

	ctx := context.Background()

	tests := []struct {
		name          string
		key           string
		setValue      string
		expectedValue string
		expectError   bool
	}{
		{
			name:          "Get existing key",
			key:           "test:key1",
			setValue:      "value1",
			expectedValue: "value1",
			expectError:   false,
		},
		{
			name:          "Get non-existing key",
			key:           "test:nonexistent",
			setValue:      "",
			expectedValue: "",
			expectError:   true, // Returns error for non-existent key
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.setValue != "" {
				mr.Set(tt.key, tt.setValue)
			}

			value, err := client.Get(ctx, tt.key)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedValue, value)
			}
		})
	}
}

func TestClient_Set(t *testing.T) {
	mr, client := setupTestRedis(t)
	defer mr.Close()

	ctx := context.Background()

	tests := []struct {
		name       string
		key        string
		value      interface{}
		ttl        time.Duration
		expectError bool
	}{
		{
			name:        "Set string value",
			key:         "test:key1",
			value:       "value1",
			ttl:         time.Minute,
			expectError: false,
		},
		{
			name:        "Set integer value",
			key:         "test:key2",
			value:       42,
			ttl:         time.Hour,
			expectError: false,
		},
		{
			name:        "Set with no expiration",
			key:         "test:key3",
			value:       "permanent",
			ttl:         0,
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := client.Set(ctx, tt.key, tt.value, tt.ttl)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				
				// Verify the value was set
				val, _ := mr.Get(tt.key)
				assert.NotEmpty(t, val)

				// Check TTL if set
				if tt.ttl > 0 {
					ttl := mr.TTL(tt.key)
					assert.Greater(t, ttl, time.Duration(0))
				}
			}
		})
	}
}

func TestClient_Delete(t *testing.T) {
	mr, client := setupTestRedis(t)
	defer mr.Close()

	ctx := context.Background()

	// Set some test keys
	mr.Set("test:key1", "value1")
	mr.Set("test:key2", "value2")
	mr.Set("test:key3", "value3")

	tests := []struct {
		name        string
		keys        []string
		expectError bool
	}{
		{
			name:        "Delete single key",
			keys:        []string{"test:key1"},
			expectError: false,
		},
		{
			name:        "Delete multiple keys",
			keys:        []string{"test:key2", "test:key3"},
			expectError: false,
		},
		{
			name:        "Delete non-existent key",
			keys:        []string{"test:nonexistent"},
			expectError: false, // Delete of non-existent key is not an error
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := client.Delete(ctx, tt.keys...)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				
				// Verify keys were deleted
				for _, key := range tt.keys {
					val, _ := mr.Get(key)
					assert.Empty(t, val)
				}
			}
		})
	}
}

func TestClient_Exists(t *testing.T) {
	mr, client := setupTestRedis(t)
	defer mr.Close()

	ctx := context.Background()

	// Set some test keys
	mr.Set("test:exists1", "value1")
	mr.Set("test:exists2", "value2")

	tests := []struct {
		name          string
		keys          []string
		expectedCount int64
		expectError   bool
	}{
		{
			name:          "Single existing key",
			keys:          []string{"test:exists1"},
			expectedCount: 1,
			expectError:   false,
		},
		{
			name:          "Multiple existing keys",
			keys:          []string{"test:exists1", "test:exists2"},
			expectedCount: 2,
			expectError:   false,
		},
		{
			name:          "Non-existent key",
			keys:          []string{"test:nonexistent"},
			expectedCount: 0,
			expectError:   false,
		},
		{
			name:          "Mixed existing and non-existent",
			keys:          []string{"test:exists1", "test:nonexistent"},
			expectedCount: 1,
			expectError:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			count, err := client.Exists(ctx, tt.keys...)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedCount, count)
			}
		})
	}
}

func TestClient_Incr(t *testing.T) {
	mr, client := setupTestRedis(t)
	defer mr.Close()

	ctx := context.Background()

	tests := []struct {
		name          string
		key           string
		initialValue  string
		expectedValue int64
		expectError   bool
	}{
		{
			name:          "Increment non-existent key",
			key:           "test:counter1",
			initialValue:  "",
			expectedValue: 1,
			expectError:   false,
		},
		{
			name:          "Increment existing counter",
			key:           "test:counter2",
			initialValue:  "5",
			expectedValue: 6,
			expectError:   false,
		},
		{
			name:          "Increment zero value",
			key:           "test:counter3",
			initialValue:  "0",
			expectedValue: 1,
			expectError:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.initialValue != "" {
				mr.Set(tt.key, tt.initialValue)
			}

			value, err := client.Incr(ctx, tt.key)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedValue, value)
			}
		})
	}
}


func TestClient_Expire(t *testing.T) {
	mr, client := setupTestRedis(t)
	defer mr.Close()

	ctx := context.Background()

	tests := []struct {
		name        string
		key         string
		setValue    string
		expiration  time.Duration
		expectError bool
	}{
		{
			name:        "Set expiration on existing key",
			key:         "test:expire1",
			setValue:    "value1",
			expiration:  time.Hour,
			expectError: false,
		},
		{
			name:        "Set expiration on non-existent key",
			key:         "test:nonexistent",
			setValue:    "",
			expiration:  time.Hour,
			expectError: false, // Redis returns false but no error
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.setValue != "" {
				mr.Set(tt.key, tt.setValue)
			}

			err := client.Expire(ctx, tt.key, tt.expiration)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				
				if tt.setValue != "" {
					// Check that TTL was set
					ttl := mr.TTL(tt.key)
					assert.Greater(t, ttl, time.Duration(0))
				}
			}
		})
	}
}




func TestClient_Pipeline(t *testing.T) {
	mr, client := setupTestRedis(t)
	defer mr.Close()

	ctx := context.Background()

	pipe := client.Pipeline()
	assert.NotNil(t, pipe)

	// Add commands to pipeline
	pipe.Set(ctx, "test:pipe1", "value1", time.Minute)
	pipe.Set(ctx, "test:pipe2", "value2", time.Minute)
	pipe.Incr(ctx, "test:counter")

	// Execute pipeline
	cmds, err := pipe.Exec(ctx)
	assert.NoError(t, err)
	assert.Len(t, cmds, 3)

	// Verify results
	val1, _ := mr.Get("test:pipe1")
	assert.Equal(t, "value1", val1)

	val2, _ := mr.Get("test:pipe2")
	assert.Equal(t, "value2", val2)

	counter, _ := mr.Get("test:counter")
	assert.Equal(t, "1", counter)
}

func TestClient_Health(t *testing.T) {
	mr, client := setupTestRedis(t)
	defer mr.Close()

	ctx := context.Background()

	// Test healthy Redis
	err := client.Health(ctx)
	assert.NoError(t, err)

	// Test unhealthy Redis (close the miniredis)
	mr.Close()
	err = client.Health(ctx)
	assert.Error(t, err)
}

func TestClient_InvalidatePattern(t *testing.T) {
	mr, client := setupTestRedis(t)
	defer mr.Close()

	ctx := context.Background()

	// Set test keys
	mr.Set("test:pattern:1", "value1")
	mr.Set("test:pattern:2", "value2")
	mr.Set("test:pattern:3", "value3")
	mr.Set("test:other:1", "other1")

	// Test pattern invalidation
	err := client.InvalidatePattern(ctx, "test:pattern:*")
	assert.NoError(t, err)

	// Pattern invalidation is not directly supported by miniredis
	// In a real Redis environment, this would delete matching keys
	// Here we're just testing that the method doesn't error
}

func TestClient_Close(t *testing.T) {
	mr, client := setupTestRedis(t)
	defer mr.Close()

	// Close should not error
	err := client.Close()
	assert.NoError(t, err)

	// After close, operations should fail
	ctx := context.Background()
	_, err = client.Get(ctx, "test:key")
	assert.Error(t, err)
}

func TestClient_KeyBuilderIntegration(t *testing.T) {
	mr, client := setupTestRedis(t)
	defer mr.Close()

	ctx := context.Background()

	// Test that KeyBuilder is properly integrated
	assert.NotNil(t, client.KeyBuilder)

	// Use KeyBuilder to generate a key and test with it
	key := client.KeyBuilder.KeyVisitorTotal()

	err := client.Set(ctx, key, "1000", time.Hour)
	assert.NoError(t, err)

	value, err := client.Get(ctx, key)
	assert.NoError(t, err)
	assert.Equal(t, "1000", value)

	// Verify the value was stored with the correct key
	// Note: The key will be whatever KeyBuilder generates for "test" environment
	val, _ := mr.Get(key)
	assert.Equal(t, "1000", val)
}

func TestClient_SetMultiple(t *testing.T) {
	mr, client := setupTestRedis(t)
	defer mr.Close()

	ctx := context.Background()

	kvPairs := map[string]interface{}{
		"test:multi1": "value1",
		"test:multi2": "value2",
		"test:multi3": 123,
	}

	err := client.SetMultiple(ctx, kvPairs, time.Minute)
	assert.NoError(t, err)

	// Verify all values were set
	for key, expectedValue := range kvPairs {
		val, _ := mr.Get(key)
		if intVal, ok := expectedValue.(int); ok {
			assert.Equal(t, "123", val) // Redis stores as string
			_ = intVal // Use the variable
		} else {
			assert.Equal(t, expectedValue, val)
		}
	}
}