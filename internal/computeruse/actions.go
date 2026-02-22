package computeruse

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"image/jpeg"
	"image/png"
	"log"
)

// MaxScreenshotBytes is the maximum screenshot size before JPEG compression (2MB).
const MaxScreenshotBytes = 2 * 1024 * 1024

// ActionExecutor executes individual browser actions and captures screenshots.
type ActionExecutor struct {
	backend  BrowserBackend
	security *SecurityValidator
}

// NewActionExecutor creates a new ActionExecutor.
func NewActionExecutor(backend BrowserBackend, security *SecurityValidator) *ActionExecutor {
	return &ActionExecutor{
		backend:  backend,
		security: security,
	}
}

// Execute runs a single computer use action and returns a base64-encoded screenshot.
// REQ-M2-01: Route computer_action messages to appropriate actions.
// REQ-M2-02: Check browser instance state before actions.
func (ae *ActionExecutor) Execute(ctx context.Context, action string, params map[string]interface{}) (screenshot string, err error) {
	// 액션 실행 전 브라우저 상태 검증
	if !ae.backend.IsActive() {
		return "", fmt.Errorf("browser is not active")
	}

	// Dispatch to the appropriate action handler.
	switch action {
	case "screenshot":
		// Screenshot-only action, no additional operation needed.

	case "click":
		x, y, parseErr := parseClickParams(params)
		if parseErr != nil {
			return "", fmt.Errorf("invalid click params: %w", parseErr)
		}
		if err := ae.backend.Click(ctx, x, y); err != nil {
			return "", err
		}

	case "type":
		text, parseErr := parseTypeParams(params)
		if parseErr != nil {
			return "", fmt.Errorf("invalid type params: %w", parseErr)
		}
		if err := ae.backend.Type(ctx, text); err != nil {
			return "", err
		}

	case "scroll":
		direction, amount, parseErr := parseScrollParams(params)
		if parseErr != nil {
			return "", fmt.Errorf("invalid scroll params: %w", parseErr)
		}
		if err := ae.backend.Scroll(ctx, direction, amount); err != nil {
			return "", err
		}

	case "navigate":
		url, parseErr := parseNavigateParams(params)
		if parseErr != nil {
			return "", fmt.Errorf("invalid navigate params: %w", parseErr)
		}
		// Validate URL before navigating.
		if err := ae.security.ValidateURL(url); err != nil {
			return "", fmt.Errorf("URL blocked: %w", err)
		}
		if err := ae.backend.Navigate(ctx, url); err != nil {
			return "", err
		}

	default:
		return "", fmt.Errorf("unknown action: %s", action)
	}

	// Capture screenshot after every action.
	pngBytes, err := ae.backend.Screenshot(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to capture screenshot: %w", err)
	}

	// Compress to JPEG if screenshot exceeds size limit.
	encoded, err := encodeScreenshot(pngBytes)
	if err != nil {
		return "", fmt.Errorf("failed to encode screenshot: %w", err)
	}

	return encoded, nil
}

// encodeScreenshot returns a base64-encoded screenshot.
// If the PNG exceeds MaxScreenshotBytes, it compresses to JPEG at 80% quality.
func encodeScreenshot(pngBytes []byte) (string, error) {
	if len(pngBytes) <= MaxScreenshotBytes {
		return base64.StdEncoding.EncodeToString(pngBytes), nil
	}

	// Decode PNG to re-encode as JPEG.
	img, err := png.Decode(bytes.NewReader(pngBytes))
	if err != nil {
		return "", fmt.Errorf("failed to decode PNG for compression: %w", err)
	}

	var jpegBuf bytes.Buffer
	if err := jpeg.Encode(&jpegBuf, img, &jpeg.Options{Quality: 80}); err != nil {
		return "", fmt.Errorf("failed to encode JPEG: %w", err)
	}

	log.Printf("[computer-use] screenshot compressed: PNG %d bytes -> JPEG %d bytes", len(pngBytes), jpegBuf.Len())
	return base64.StdEncoding.EncodeToString(jpegBuf.Bytes()), nil
}

// parseClickParams extracts x and y coordinates from action parameters.
func parseClickParams(params map[string]interface{}) (float64, float64, error) {
	x, err := toFloat64(params, "x")
	if err != nil {
		return 0, 0, fmt.Errorf("missing or invalid 'x': %w", err)
	}
	y, err := toFloat64(params, "y")
	if err != nil {
		return 0, 0, fmt.Errorf("missing or invalid 'y': %w", err)
	}
	return x, y, nil
}

// parseTypeParams extracts the text string from action parameters.
func parseTypeParams(params map[string]interface{}) (string, error) {
	text, ok := params["text"]
	if !ok {
		return "", fmt.Errorf("missing 'text' parameter")
	}
	s, ok := text.(string)
	if !ok {
		return "", fmt.Errorf("'text' parameter is not a string")
	}
	return s, nil
}

// parseScrollParams extracts direction and amount from action parameters.
func parseScrollParams(params map[string]interface{}) (string, int, error) {
	direction, ok := params["direction"]
	if !ok {
		return "", 0, fmt.Errorf("missing 'direction' parameter")
	}
	dirStr, ok := direction.(string)
	if !ok {
		return "", 0, fmt.Errorf("'direction' parameter is not a string")
	}
	if dirStr != "up" && dirStr != "down" {
		return "", 0, fmt.Errorf("'direction' must be 'up' or 'down', got %q", dirStr)
	}

	amount := 300 // Default scroll amount in pixels.
	if amountVal, ok := params["amount"]; ok {
		amountFloat, err := toFloat64Value(amountVal)
		if err != nil {
			return "", 0, fmt.Errorf("invalid 'amount': %w", err)
		}
		amount = int(amountFloat)
	}

	return dirStr, amount, nil
}

// parseNavigateParams extracts the URL from action parameters.
func parseNavigateParams(params map[string]interface{}) (string, error) {
	urlVal, ok := params["url"]
	if !ok {
		return "", fmt.Errorf("missing 'url' parameter")
	}
	urlStr, ok := urlVal.(string)
	if !ok {
		return "", fmt.Errorf("'url' parameter is not a string")
	}
	return urlStr, nil
}

// toFloat64 extracts a float64 value from a map by key.
func toFloat64(m map[string]interface{}, key string) (float64, error) {
	val, ok := m[key]
	if !ok {
		return 0, fmt.Errorf("key %q not found", key)
	}
	return toFloat64Value(val)
}

// toFloat64Value converts an interface{} to float64.
// Handles both float64 (JSON default) and int types.
func toFloat64Value(val interface{}) (float64, error) {
	switch v := val.(type) {
	case float64:
		return v, nil
	case int:
		return float64(v), nil
	case int64:
		return float64(v), nil
	default:
		return 0, fmt.Errorf("cannot convert %T to float64", val)
	}
}
