package computeruse

import (
	"testing"
)

func TestSecurityValidator_ValidateURL(t *testing.T) {
	v := NewSecurityValidator()

	tests := []struct {
		name    string
		url     string
		wantErr bool
		errMsg  string
	}{
		// Allowed URLs.
		{name: "http URL", url: "http://localhost:3000", wantErr: false},
		{name: "https URL", url: "https://example.com", wantErr: false},
		{name: "http with path", url: "http://127.0.0.1:8080/api/v1", wantErr: false},
		{name: "https default port", url: "https://example.com/page", wantErr: false},
		{name: "http default port", url: "http://example.com/page", wantErr: false},
		{name: "high port number", url: "http://localhost:49152", wantErr: false},

		// Blocked protocols (REQ-M2-08).
		{name: "file protocol", url: "file:///etc/passwd", wantErr: true, errMsg: "blocked protocol"},
		{name: "ftp protocol", url: "ftp://files.example.com/data", wantErr: true, errMsg: "blocked protocol"},
		{name: "javascript protocol", url: "javascript:alert(1)", wantErr: true, errMsg: "blocked protocol"},
		{name: "data protocol", url: "data:text/html,<h1>test</h1>", wantErr: true, errMsg: "blocked protocol"},

		// Blocked ports (REQ-M2-08).
		{name: "SSH port 22", url: "http://localhost:22", wantErr: true, errMsg: "blocked port 22"},
		{name: "SMTP port 25", url: "http://localhost:25", wantErr: true, errMsg: "blocked port 25"},
		{name: "SMTP submission 587", url: "http://localhost:587", wantErr: true, errMsg: "blocked port 587"},
		{name: "PostgreSQL port 5432", url: "http://localhost:5432", wantErr: true, errMsg: "blocked port 5432"},
		{name: "MySQL port 3306", url: "http://localhost:3306", wantErr: true, errMsg: "blocked port 3306"},
		{name: "Redis port 6379", url: "http://localhost:6379", wantErr: true, errMsg: "blocked port 6379"},
		{name: "MongoDB port 27017", url: "http://localhost:27017", wantErr: true, errMsg: "blocked port 27017"},

		// Invalid URLs.
		{name: "empty URL", url: "", wantErr: true, errMsg: "empty"},
		{name: "no scheme", url: "example.com", wantErr: true, errMsg: "no scheme"},
		{name: "invalid port in URL", url: "http://localhost:abc/page", wantErr: true, errMsg: "failed to extract port"},
		{name: "port out of range", url: "http://localhost:0/page", wantErr: true, errMsg: "failed to extract port"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := v.ValidateURL(tt.url)
			if tt.wantErr {
				if err == nil {
					t.Errorf("ValidateURL(%q) = nil, want error containing %q", tt.url, tt.errMsg)
				}
			} else {
				if err != nil {
					t.Errorf("ValidateURL(%q) = %v, want nil", tt.url, err)
				}
			}
		})
	}
}

func TestSecurityValidator_extractPort(t *testing.T) {
	v := NewSecurityValidator()

	tests := []struct {
		name     string
		host     string
		scheme   string
		wantPort int
		wantErr  bool
	}{
		{name: "explicit port", host: "localhost:8080", scheme: "http", wantPort: 8080},
		{name: "http default", host: "example.com", scheme: "http", wantPort: 80},
		{name: "https default", host: "example.com", scheme: "https", wantPort: 443},
		{name: "explicit port 443", host: "example.com:443", scheme: "https", wantPort: 443},
		{name: "unknown scheme no port", host: "example.com", scheme: "ws", wantErr: true},
		{name: "invalid port string", host: "example.com:abc", scheme: "http", wantErr: true},
		{name: "port too low", host: "example.com:0", scheme: "http", wantErr: true},
		{name: "port too high", host: "example.com:99999", scheme: "http", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			port, err := v.extractPort(tt.host, tt.scheme)
			if tt.wantErr {
				if err == nil {
					t.Errorf("extractPort(%q, %q) = %d, nil; want error", tt.host, tt.scheme, port)
				}
				return
			}
			if err != nil {
				t.Errorf("extractPort(%q, %q) = error %v; want %d", tt.host, tt.scheme, err, tt.wantPort)
				return
			}
			if port != tt.wantPort {
				t.Errorf("extractPort(%q, %q) = %d; want %d", tt.host, tt.scheme, port, tt.wantPort)
			}
		})
	}
}
