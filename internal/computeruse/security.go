// Package computeruse provides browser automation for computer use actions
// in the Local Agent Bridge (SPEC-COMPUTER-USE-001).
// REQ-M2-08: Block file:// and sensitive ports.
package computeruse

import (
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"
)

// SecurityValidator validates URLs and actions for safety.
type SecurityValidator struct{}

// blockedPorts are ports that should never be accessed.
var blockedPorts = map[int]bool{
	22:    true, // SSH
	25:    true, // SMTP
	587:   true, // SMTP submission
	5432:  true, // PostgreSQL
	3306:  true, // MySQL
	6379:  true, // Redis
	27017: true, // MongoDB
}

// allowedProtocols are the only protocols allowed for navigation.
var allowedProtocols = map[string]bool{
	"http":  true,
	"https": true,
}

// defaultPorts maps protocol to its default port.
var defaultPorts = map[string]int{
	"http":  80,
	"https": 443,
}

// NewSecurityValidator creates a new SecurityValidator.
func NewSecurityValidator() *SecurityValidator {
	return &SecurityValidator{}
}

// ValidateURL checks if a URL is safe to navigate to.
// Returns error if URL uses a blocked protocol or port.
func (v *SecurityValidator) ValidateURL(rawURL string) error {
	if rawURL == "" {
		return fmt.Errorf("URL is empty")
	}

	parsed, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("invalid URL: %w", err)
	}

	// Check protocol (scheme).
	scheme := strings.ToLower(parsed.Scheme)
	if scheme == "" {
		return fmt.Errorf("URL has no scheme: %s", rawURL)
	}

	if !allowedProtocols[scheme] {
		return fmt.Errorf("blocked protocol %q: only http and https are allowed", scheme)
	}

	// Extract port from host.
	port, err := v.extractPort(parsed.Host, scheme)
	if err != nil {
		return fmt.Errorf("failed to extract port from URL: %w", err)
	}

	// Check against blocked ports.
	if blockedPorts[port] {
		return fmt.Errorf("blocked port %d: access to this port is not allowed", port)
	}

	return nil
}

// extractPort extracts the port number from a host string.
// If no port is explicitly specified, the default port for the scheme is returned.
func (v *SecurityValidator) extractPort(host, scheme string) (int, error) {
	// net.SplitHostPort expects host:port format.
	_, portStr, err := net.SplitHostPort(host)
	if err != nil {
		// No port specified, use default for scheme.
		if defaultPort, ok := defaultPorts[scheme]; ok {
			return defaultPort, nil
		}
		return 0, fmt.Errorf("unknown scheme %q and no port specified", scheme)
	}

	port, err := strconv.Atoi(portStr)
	if err != nil {
		return 0, fmt.Errorf("invalid port %q: %w", portStr, err)
	}

	if port < 1 || port > 65535 {
		return 0, fmt.Errorf("port %d out of valid range (1-65535)", port)
	}

	return port, nil
}
