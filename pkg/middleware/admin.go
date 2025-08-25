package middleware

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"sync"
)

// RateLimiter implements a simple rate limiter
type RateLimiter struct {
	mu       sync.RWMutex
	requests map[string][]time.Time
	limit    int
	window   time.Duration
}

// NewRateLimiter creates a new rate limiter
func NewRateLimiter(limit int, window time.Duration) *RateLimiter {
	return &RateLimiter{
		requests: make(map[string][]time.Time),
		limit:    limit,
		window:   window,
	}
}

// Allow checks if a request is allowed for the given key
func (rl *RateLimiter) Allow(key string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	
	now := time.Now()
	cutoff := now.Add(-rl.window)
	
	// Clean up old requests
	requests := rl.requests[key]
	validRequests := requests[:0]
	for _, reqTime := range requests {
		if reqTime.After(cutoff) {
			validRequests = append(validRequests, reqTime)
		}
	}
	
	// Check if limit exceeded
	if len(validRequests) >= rl.limit {
		rl.requests[key] = validRequests
		return false
	}
	
	// Add current request
	validRequests = append(validRequests, now)
	rl.requests[key] = validRequests
	return true
}

// AuditLogger handles audit logging for admin operations
type AuditLogger struct {
	mu sync.Mutex
}

// NewAuditLogger creates a new audit logger
func NewAuditLogger() *AuditLogger {
	return &AuditLogger{}
}

// LogAccess logs an access attempt
func (al *AuditLogger) LogAccess(r *http.Request, status, details string) {
	al.mu.Lock()
	defer al.mu.Unlock()
	
	log.Printf("🔍 [ADMIN-AUDIT] %s %s from %s - Status: %s - Details: %s - UA: %s", 
		r.Method, r.URL.Path, getClientIP(r), status, details, r.Header.Get("User-Agent"))
}

// getClientIP extracts client IP for audit logging
func getClientIP(r *http.Request) string {
	// Check X-Forwarded-For header
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		ips := strings.Split(xff, ",")
		return strings.TrimSpace(ips[0])
	}
	
	// Check X-Real-IP header
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return strings.TrimSpace(xri)
	}
	
	// Fall back to remote address
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return ip
}

// AdminClaims represents JWT claims for admin users
type AdminClaims struct {
	UserID   string   `json:"user_id"`
	Email    string   `json:"email"`
	Roles    []string `json:"roles"`
	IsAdmin  bool     `json:"is_admin"`
	GoogleID string   `json:"google_id"`
	jwt.RegisteredClaims
}

// AdminMiddleware provides authentication and authorization for admin endpoints
type AdminMiddleware struct {
	jwtSecret      string
	allowedIPs     []string
	rateLimiter    *RateLimiter
	auditLogger    *AuditLogger
}

// NewAdminMiddleware creates a new admin middleware instance
func NewAdminMiddleware() *AdminMiddleware {
	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		jwtSecret = "default-development-secret-change-in-production"
		log.Println("⚠️ Using default JWT secret for admin middleware. Set JWT_SECRET in production!")
	}

	// Parse allowed IPs from environment variable
	allowedIPsEnv := os.Getenv("ADMIN_ALLOWED_IPS")
	var allowedIPs []string
	if allowedIPsEnv != "" {
		allowedIPs = strings.Split(allowedIPsEnv, ",")
		for i, ip := range allowedIPs {
			allowedIPs[i] = strings.TrimSpace(ip)
		}
	} else {
		// Default to localhost for development
		allowedIPs = []string{"127.0.0.1", "::1", "localhost"}
		log.Println("⚠️ No ADMIN_ALLOWED_IPS set, defaulting to localhost only")
	}

	return &AdminMiddleware{
		jwtSecret:   jwtSecret,
		allowedIPs:  allowedIPs,
		rateLimiter: NewRateLimiter(10, time.Minute), // 10 requests per minute per IP
		auditLogger: NewAuditLogger(),
	}
}

// RequireAdmin is the main middleware function for admin routes
func (m *AdminMiddleware) RequireAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Step 1: IP allowlist check
		if !m.isIPAllowed(r) {
			m.auditLogger.LogAccess(r, "BLOCKED", "IP not in allowlist")
			m.sendErrorResponse(w, "Access denied", http.StatusForbidden)
			return
		}

		// Step 2: Rate limiting
		clientIP := m.getClientIP(r)
		if !m.rateLimiter.Allow(clientIP) {
			m.auditLogger.LogAccess(r, "RATE_LIMITED", "Too many requests")
			m.sendErrorResponse(w, "Rate limit exceeded", http.StatusTooManyRequests)
			return
		}

		// Step 3: JWT authentication and authorization
		claims, err := m.validateAdminToken(r)
		if err != nil {
			m.auditLogger.LogAccess(r, "AUTH_FAILED", err.Error())
			m.sendErrorResponse(w, "Authentication failed", http.StatusUnauthorized)
			return
		}

		// Step 4: Admin role check
		if !claims.IsAdmin && !m.hasAdminRole(claims.Roles) {
			m.auditLogger.LogAccess(r, "AUTHORIZATION_FAILED", "Insufficient privileges")
			m.sendErrorResponse(w, "Insufficient privileges", http.StatusForbidden)
			return
		}

		// Step 5: Add admin context and continue
		ctx := context.WithValue(r.Context(), "admin_claims", claims)
		m.auditLogger.LogAccess(r, "AUTHORIZED", fmt.Sprintf("Admin: %s", claims.Email))
		
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// isIPAllowed checks if the request IP is in the allowlist
func (m *AdminMiddleware) isIPAllowed(r *http.Request) bool {
	clientIP := m.getClientIP(r)
	
	// Check if it's a local development environment
	if clientIP == "127.0.0.1" || clientIP == "::1" || clientIP == "localhost" {
		return m.containsIP(m.allowedIPs, clientIP)
	}

	// Parse IP for proper comparison
	ip := net.ParseIP(clientIP)
	if ip == nil {
		log.Printf("❌ [ADMIN-MIDDLEWARE] Invalid IP format: %s", clientIP)
		return false
	}

	for _, allowedIP := range m.allowedIPs {
		// Check for exact match
		if allowedIP == clientIP {
			return true
		}
		
		// Check for CIDR match
		if strings.Contains(allowedIP, "/") {
			_, network, err := net.ParseCIDR(allowedIP)
			if err == nil && network.Contains(ip) {
				return true
			}
		}
	}

	log.Printf("❌ [ADMIN-MIDDLEWARE] IP %s not in allowlist: %v", clientIP, m.allowedIPs)
	return false
}

// getClientIP extracts the client IP from request headers
func (m *AdminMiddleware) getClientIP(r *http.Request) string {
	return getClientIP(r)
}

// containsIP checks if an IP is in the allowed list
func (m *AdminMiddleware) containsIP(allowedIPs []string, ip string) bool {
	for _, allowedIP := range allowedIPs {
		if allowedIP == ip {
			return true
		}
	}
	return false
}

// validateAdminToken validates the JWT token and extracts admin claims
func (m *AdminMiddleware) validateAdminToken(r *http.Request) (*AdminClaims, error) {
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		return nil, fmt.Errorf("no authorization header")
	}

	// Extract token from "Bearer <token>"
	parts := strings.Split(authHeader, " ")
	if len(parts) != 2 || parts[0] != "Bearer" {
		return nil, fmt.Errorf("invalid authorization header format")
	}

	tokenString := parts[1]

	// Parse and validate JWT token
	token, err := jwt.ParseWithClaims(tokenString, &AdminClaims{}, func(token *jwt.Token) (interface{}, error) {
		// Validate signing method
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(m.jwtSecret), nil
	})

	if err != nil {
		return nil, fmt.Errorf("invalid token: %w", err)
	}

	if claims, ok := token.Claims.(*AdminClaims); ok && token.Valid {
		// Validate required claims
		if claims.UserID == "" {
			return nil, fmt.Errorf("invalid token: missing user_id claim")
		}

		// Validate issuer if present
		if claims.Issuer != "" && claims.Issuer != "youtube-activity-platform" {
			return nil, fmt.Errorf("invalid token: invalid issuer")
		}

		return claims, nil
	}

	return nil, fmt.Errorf("invalid token claims")
}

// hasAdminRole checks if the user has admin role
func (m *AdminMiddleware) hasAdminRole(roles []string) bool {
	for _, role := range roles {
		if role == "admin" || role == "super_admin" {
			return true
		}
	}
	return false
}

// sendErrorResponse sends a standardized error response
func (m *AdminMiddleware) sendErrorResponse(w http.ResponseWriter, message string, statusCode int) {
	response := map[string]interface{}{
		"success": false,
		"message": message,
		"error":   true,
	}
	
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(response)
}

// GetAdminClaims retrieves admin claims from request context
func GetAdminClaims(r *http.Request) (*AdminClaims, bool) {
	claims, ok := r.Context().Value("admin_claims").(*AdminClaims)
	return claims, ok
}