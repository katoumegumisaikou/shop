package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestGetJWTTokenFromCtx(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name        string
		authHeader  string
		cookieToken string
		wantToken   string
		wantOK      bool
	}{
		{
			name:       "bearer header",
			authHeader: "Bearer header-token",
			wantToken:  "header-token",
			wantOK:     true,
		},
		{
			name:       "lowercase bearer header",
			authHeader: "bearer header-token",
			wantToken:  "header-token",
			wantOK:     true,
		},
		{
			name:        "cookie fallback",
			cookieToken: "cookie-token",
			wantToken:   "cookie-token",
			wantOK:      true,
		},
		{
			name:        "malformed header does not fallback to cookie",
			authHeader:  "Basic bad-token",
			cookieToken: "cookie-token",
			wantOK:      false,
		},
		{
			name:   "missing token",
			wantOK: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := newTestContextWithToken(tt.authHeader, tt.cookieToken)

			gotToken, gotOK := getJWTTokenFromCtx(c)
			if gotOK != tt.wantOK {
				t.Fatalf("expected ok %v, got %v", tt.wantOK, gotOK)
			}
			if gotToken != tt.wantToken {
				t.Fatalf("expected token %q, got %q", tt.wantToken, gotToken)
			}
		})
	}
}

func newTestContextWithToken(authHeader, cookieToken string) *gin.Context {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	if authHeader != "" {
		req.Header.Set("Authorization", authHeader)
	}
	if cookieToken != "" {
		req.AddCookie(&http.Cookie{Name: "access_token", Value: cookieToken})
	}
	c.Request = req
	return c
}
