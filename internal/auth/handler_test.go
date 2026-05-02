package auth

import "testing"

func TestResolveOAuthRedirectURL(t *testing.T) {
	frontendURL := "http://localhost:3000"

	tests := []struct {
		name        string
		redirectURL string
		want        string
	}{
		{
			name:        "default redirects to frontend root",
			redirectURL: "",
			want:        "http://localhost:3000/",
		},
		{
			name:        "relative path resolves against frontend",
			redirectURL: "/user/dashboard",
			want:        "http://localhost:3000/user/dashboard",
		},
		{
			name:        "login path is normalized to home",
			redirectURL: "/login",
			want:        "http://localhost:3000/",
		},
		{
			name:        "external absolute url falls back to home",
			redirectURL: "https://example.com/welcome",
			want:        "http://localhost:3000/",
		},
		{
			name:        "frontend absolute url is preserved",
			redirectURL: "http://localhost:3000/user/dashboard",
			want:        "http://localhost:3000/user/dashboard",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := resolveOAuthRedirectURL(frontendURL, tt.redirectURL)
			if got != tt.want {
				t.Fatalf("resolveOAuthRedirectURL(%q) = %q, want %q", tt.redirectURL, got, tt.want)
			}
		})
	}
}
