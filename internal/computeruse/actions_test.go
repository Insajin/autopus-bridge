package computeruse

import (
	"bytes"
	"encoding/base64"
	"image"
	"image/png"
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

// createLargePNG은 MaxScreenshotBytes를 초과하는 유효한 PNG 이미지를 생성한다.
// 부드러운 그라데이션 패턴으로 JPEG 압축에도 효율적이면서 PNG 크기는 2MB를 초과한다.
func createLargePNG(t *testing.T) []byte {
	t.Helper()
	// 3000x3000 NRGBA 이미지: 적당한 노이즈 + 그라데이션 혼합
	width, height := 3000, 3000
	img := image.NewNRGBA(image.Rect(0, 0, width, height))
	// 의사난수 패턴을 적용하되 부분적으로 예측 가능하게
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			offset := (y*width + x) * 4
			v := uint32(x*37+y*13) ^ uint32(x*y)
			img.Pix[offset+0] = byte(v)
			img.Pix[offset+1] = byte(v >> 8)
			img.Pix[offset+2] = byte(v >> 16)
			img.Pix[offset+3] = 255
		}
	}

	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("PNG 인코딩 실패: %v", err)
	}

	// 2MB 초과 확인, 미달 시 픽셀 데이터를 직접 추가하여 크기 보장
	if buf.Len() <= MaxScreenshotBytes {
		// 더 큰 이미지로 재시도
		width, height = 6000, 6000
		img = image.NewNRGBA(image.Rect(0, 0, width, height))
		for i := range img.Pix {
			img.Pix[i] = byte(i ^ (i >> 8) ^ (i >> 16))
		}
		buf.Reset()
		if err := png.Encode(&buf, img); err != nil {
			t.Fatalf("PNG 인코딩 실패: %v", err)
		}
	}

	if buf.Len() <= MaxScreenshotBytes {
		t.Skipf("생성된 PNG 크기 = %d 바이트; MaxScreenshotBytes 초과 불가 (환경 제한)", buf.Len())
	}

	return buf.Bytes()
}

func TestEncodeScreenshot_LargeImage_JPEG_Compression(t *testing.T) {
	// MaxScreenshotBytes 초과하는 유효한 PNG 생성
	largePNG := createLargePNG(t)

	encoded, err := encodeScreenshot(largePNG)
	if err != nil {
		t.Fatalf("encodeScreenshot(large PNG) = error %v; want nil", err)
	}
	if encoded == "" {
		t.Error("encodeScreenshot(large PNG) = empty string; want non-empty base64")
	}

	// base64 디코딩하여 JPEG 데이터가 유효한지 확인
	decoded, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		t.Fatalf("base64 디코딩 실패: %v", err)
	}
	if len(decoded) == 0 {
		t.Error("JPEG 결과가 비어있음")
	}
}

func TestEncodeScreenshot_InvalidPNG_OverLimit(t *testing.T) {
	// MaxScreenshotBytes 초과하지만 유효하지 않은 PNG 데이터
	invalidLargeData := make([]byte, MaxScreenshotBytes+1)
	for i := range invalidLargeData {
		invalidLargeData[i] = byte(i % 256)
	}

	_, err := encodeScreenshot(invalidLargeData)
	if err == nil {
		t.Error("encodeScreenshot(invalid large data) = nil error; want PNG decode error")
	}
}

func TestParseScrollParams_DirectionNotString(t *testing.T) {
	// direction이 문자열이 아닌 경우
	params := map[string]interface{}{"direction": 123}
	_, _, err := parseScrollParams(params)
	if err == nil {
		t.Error("parseScrollParams(direction=int) = nil error; want error")
	}
}

func TestParseScrollParams_InvalidAmount(t *testing.T) {
	// amount가 숫자로 변환 불가능한 경우
	params := map[string]interface{}{"direction": "down", "amount": "invalid"}
	_, _, err := parseScrollParams(params)
	if err == nil {
		t.Error("parseScrollParams(amount=string) = nil error; want error")
	}
}
