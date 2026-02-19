package computeruse

import (
	"testing"
)

func TestParseClickParams(t *testing.T) {
	tests := []struct {
		name    string
		params  map[string]interface{}
		wantX   float64
		wantY   float64
		wantErr bool
	}{
		{
			name:   "valid float64 params",
			params: map[string]interface{}{"x": 100.0, "y": 200.0},
			wantX:  100.0, wantY: 200.0,
		},
		{
			name:    "missing x",
			params:  map[string]interface{}{"y": 200.0},
			wantErr: true,
		},
		{
			name:    "missing y",
			params:  map[string]interface{}{"x": 100.0},
			wantErr: true,
		},
		{
			name:    "invalid type",
			params:  map[string]interface{}{"x": "abc", "y": 200.0},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			x, y, err := parseClickParams(tt.params)
			if tt.wantErr {
				if err == nil {
					t.Error("parseClickParams() = nil error; want error")
				}
				return
			}
			if err != nil {
				t.Fatalf("parseClickParams() = error %v; want nil", err)
			}
			if x != tt.wantX || y != tt.wantY {
				t.Errorf("parseClickParams() = (%v, %v); want (%v, %v)", x, y, tt.wantX, tt.wantY)
			}
		})
	}
}

func TestParseTypeParams(t *testing.T) {
	tests := []struct {
		name    string
		params  map[string]interface{}
		want    string
		wantErr bool
	}{
		{
			name:   "valid text",
			params: map[string]interface{}{"text": "hello world"},
			want:   "hello world",
		},
		{
			name:    "missing text",
			params:  map[string]interface{}{},
			wantErr: true,
		},
		{
			name:    "wrong type",
			params:  map[string]interface{}{"text": 123},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			text, err := parseTypeParams(tt.params)
			if tt.wantErr {
				if err == nil {
					t.Error("parseTypeParams() = nil error; want error")
				}
				return
			}
			if err != nil {
				t.Fatalf("parseTypeParams() = error %v; want nil", err)
			}
			if text != tt.want {
				t.Errorf("parseTypeParams() = %q; want %q", text, tt.want)
			}
		})
	}
}

func TestParseScrollParams(t *testing.T) {
	tests := []struct {
		name    string
		params  map[string]interface{}
		wantDir string
		wantAmt int
		wantErr bool
	}{
		{
			name:    "scroll down with amount",
			params:  map[string]interface{}{"direction": "down", "amount": 500.0},
			wantDir: "down", wantAmt: 500,
		},
		{
			name:    "scroll up with default amount",
			params:  map[string]interface{}{"direction": "up"},
			wantDir: "up", wantAmt: 300,
		},
		{
			name:    "missing direction",
			params:  map[string]interface{}{},
			wantErr: true,
		},
		{
			name:    "invalid direction",
			params:  map[string]interface{}{"direction": "left"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir, amt, err := parseScrollParams(tt.params)
			if tt.wantErr {
				if err == nil {
					t.Error("parseScrollParams() = nil error; want error")
				}
				return
			}
			if err != nil {
				t.Fatalf("parseScrollParams() = error %v; want nil", err)
			}
			if dir != tt.wantDir || amt != tt.wantAmt {
				t.Errorf("parseScrollParams() = (%q, %d); want (%q, %d)", dir, amt, tt.wantDir, tt.wantAmt)
			}
		})
	}
}

func TestParseNavigateParams(t *testing.T) {
	tests := []struct {
		name    string
		params  map[string]interface{}
		want    string
		wantErr bool
	}{
		{
			name:   "valid URL",
			params: map[string]interface{}{"url": "http://example.com"},
			want:   "http://example.com",
		},
		{
			name:    "missing url",
			params:  map[string]interface{}{},
			wantErr: true,
		},
		{
			name:    "wrong type",
			params:  map[string]interface{}{"url": 123},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			url, err := parseNavigateParams(tt.params)
			if tt.wantErr {
				if err == nil {
					t.Error("parseNavigateParams() = nil error; want error")
				}
				return
			}
			if err != nil {
				t.Fatalf("parseNavigateParams() = error %v; want nil", err)
			}
			if url != tt.want {
				t.Errorf("parseNavigateParams() = %q; want %q", url, tt.want)
			}
		})
	}
}

func TestToFloat64Value(t *testing.T) {
	tests := []struct {
		name    string
		val     interface{}
		want    float64
		wantErr bool
	}{
		{name: "float64", val: 42.5, want: 42.5},
		{name: "int", val: 42, want: 42.0},
		{name: "int64", val: int64(42), want: 42.0},
		{name: "string", val: "42", wantErr: true},
		{name: "nil", val: nil, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := toFloat64Value(tt.val)
			if tt.wantErr {
				if err == nil {
					t.Error("toFloat64Value() = nil error; want error")
				}
				return
			}
			if err != nil {
				t.Fatalf("toFloat64Value() = error %v; want nil", err)
			}
			if got != tt.want {
				t.Errorf("toFloat64Value() = %v; want %v", got, tt.want)
			}
		})
	}
}

func TestEncodeScreenshot_SmallImage(t *testing.T) {
	// Small PNG-like data (under MaxScreenshotBytes).
	smallData := make([]byte, 1024)
	for i := range smallData {
		smallData[i] = byte(i % 256)
	}

	// This won't be valid PNG but encodeScreenshot should still base64 it
	// since it's under the limit.
	encoded, err := encodeScreenshot(smallData)
	if err != nil {
		t.Fatalf("encodeScreenshot() = error %v; want nil", err)
	}
	if encoded == "" {
		t.Error("encodeScreenshot() = empty string; want non-empty base64")
	}
}
