package provider

import "testing"

// TestParseChatGPTAuthEnv는 chatgpt_auth_env JSON 파싱을 검증합니다.
func TestParseChatGPTAuthEnv(t *testing.T) {
	tests := []struct {
		name          string
		raw           string
		wantToken     string
		wantAccountID string
		wantErr       bool
	}{
		{
			name:          "camelCase 키",
			raw:           `{"accessToken":"acc-token-1","chatgptAccountId":"acct-1"}`,
			wantToken:     "acc-token-1",
			wantAccountID: "acct-1",
			wantErr:       false,
		},
		{
			name:          "snake_case 키",
			raw:           `{"access_token":"acc-token-2","chatgpt_account_id":"acct-2"}`,
			wantToken:     "acc-token-2",
			wantAccountID: "acct-2",
			wantErr:       false,
		},
		{
			name:    "access token 누락",
			raw:     `{"chatgptAccountId":"acct-3"}`,
			wantErr: true,
		},
		{
			name:    "account id 누락",
			raw:     `{"accessToken":"acc-token-4"}`,
			wantErr: true,
		},
		{
			name:    "유효하지 않은 JSON",
			raw:     `not-json`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotToken, gotAccountID, err := parseChatGPTAuthEnv(tt.raw)
			if (err != nil) != tt.wantErr {
				t.Fatalf("parseChatGPTAuthEnv() err = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}
			if gotToken != tt.wantToken {
				t.Fatalf("access token = %q, want %q", gotToken, tt.wantToken)
			}
			if gotAccountID != tt.wantAccountID {
				t.Fatalf("account ID = %q, want %q", gotAccountID, tt.wantAccountID)
			}
		})
	}
}
