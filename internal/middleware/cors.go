package middleware

import (
	"be-v2/pkg/logger"
	"net/http"
	"strings"
)

// CORSConfig holds CORS configuration
type CORSConfig struct {
	AllowedOrigins   []string
	AllowedMethods   []string
	AllowedHeaders   []string
	ExposedHeaders   []string
	AllowCredentials bool
	MaxAge           int
}

// DefaultCORSConfig returns a default CORS configuration
func DefaultCORSConfig() *CORSConfig {
	return &CORSConfig{
		AllowedMethods: []string{
			http.MethodGet,
			http.MethodPost,
			http.MethodPut,
			http.MethodPatch,
			http.MethodDelete,
			http.MethodOptions,
		},
		AllowedHeaders: []string{
			"Accept",
			"Content-Type",
			"Content-Length",
			"Accept-Encoding",
			"X-CSRF-Token",
			"Authorization",
			"X-Requested-With",
		},
		ExposedHeaders: []string{
			"Content-Length",
			"Access-Control-Allow-Origin",
			"Access-Control-Allow-Headers",
			"Content-Type",
		},
		AllowCredentials: true,
		MaxAge:           86400, // 24 hours
	}
}

// CORS creates a CORS middleware
func CORS(config *CORSConfig, logger *logger.Logger) func(http.Handler) http.Handler {
	if config == nil {
		config = DefaultCORSConfig()
	}

	allowedOrigins := make(map[string]bool)
	for _, origin := range config.AllowedOrigins {
		allowedOrigins[origin] = true
	}

	allowedMethods := strings.Join(config.AllowedMethods, ", ")
	allowedHeaders := strings.Join(config.AllowedHeaders, ", ")
	exposedHeaders := strings.Join(config.ExposedHeaders, ", ")

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")

			// Log CORS request
			logger.WithFields(map[string]interface{}{
				"origin": origin,
				"method": r.Method,
				"path":   r.URL.Path,
			}).Debug("CORS request")

			// Check if origin is allowed
			if origin != "" && (len(config.AllowedOrigins) == 0 || allowedOrigins["*"] || allowedOrigins[origin]) {
				w.Header().Set("Access-Control-Allow-Origin", origin)
			} else if len(config.AllowedOrigins) > 0 && allowedOrigins["*"] {
				w.Header().Set("Access-Control-Allow-Origin", "*")
			}

			// Set CORS headers
			if config.AllowCredentials {
				w.Header().Set("Access-Control-Allow-Credentials", "true")
			}

			if len(config.AllowedMethods) > 0 {
				w.Header().Set("Access-Control-Allow-Methods", allowedMethods)
			}

			if len(config.AllowedHeaders) > 0 {
				w.Header().Set("Access-Control-Allow-Headers", allowedHeaders)
			}

			if len(config.ExposedHeaders) > 0 {
				w.Header().Set("Access-Control-Expose-Headers", exposedHeaders)
			}

			if config.MaxAge > 0 {
				w.Header().Set("Access-Control-Max-Age", string(rune(config.MaxAge)))
			}

			// Handle preflight requests
			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
