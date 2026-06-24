package utils

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
		cookieName  string
		cookieToken string
		wantToken   string
		wantOK      bool
	}{
		{
			name:       "bearer header",
			authHeader: "Bearer header-token",
			cookieName: "access_token",
			wantToken:  "header-token",
			wantOK:     true,
		},
		{
			name:        "cookie fallback",
			cookieName:  "refresh_token",
			cookieToken: "cookie-token",
			wantToken:   "cookie-token",
			wantOK:      true,
		},
		{
			name:        "malformed header does not fallback to cookie",
			authHeader:  "Basic bad-token",
			cookieName:  "access_token",
			cookieToken: "cookie-token",
			wantOK:      false,
		},
		{
			name:        "missing cookie name",
			cookieToken: "cookie-token",
			wantOK:      false,
		},
		{
			name:       "missing token",
			cookieName: "access_token",
			wantOK:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := newTestContextWithJWT(tt.authHeader, tt.cookieName, tt.cookieToken)

			gotToken, gotOK := GetJWTTokenFromCtx(c, tt.cookieName)
			if gotOK != tt.wantOK {
				t.Fatalf("expected ok %v, got %v", tt.wantOK, gotOK)
			}
			if gotToken != tt.wantToken {
				t.Fatalf("expected token %q, got %q", tt.wantToken, gotToken)
			}
		})
	}
}

func newTestContextWithJWT(authHeader, cookieName, cookieToken string) *gin.Context {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	if authHeader != "" {
		req.Header.Set("Authorization", authHeader)
	}
	if cookieName != "" && cookieToken != "" {
		req.AddCookie(&http.Cookie{Name: cookieName, Value: cookieToken})
	}
	c.Request = req
	return c
}
