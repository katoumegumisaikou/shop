package account

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"

	pkgjwt "shop/internal/pkg/jwt"
	"shop/internal/pkg/snowflake"
	"shop/internal/pkg/wxlogin"
)

func TestH5CallbackHTTPSetCookiesAndRedirect(t *testing.T) {
	gin.SetMode(gin.TestMode)
	snowflake.Init(1)
	pkgjwt.InitJwtConfig(pkgjwt.JwtConfig{
		JwtSecret:         "test-secret",
		Expiration:        time.Hour,
		RefreshExpiration: 24 * time.Hour,
	}, pkgjwt.JwtConfig{})

	openid := "h5-openid-001"
	repo := &mockUserRepo{
		h5Found: &User{
			ID:       2002,
			OpenidH5: &openid,
			Source:   "h5",
			Status:   "active",
		},
	}
	wx := mockWxLoginClient{
		oauthResp: &wxlogin.OAuthResp{
			OpenID:       openid,
			AccessToken:  "wx-access-token",
			RefreshToken: "wx-refresh-token",
			ExpiresIn:    7200,
		},
	}
	svc := NewService(repo, nil, nil, wx)
	h := NewHandler(svc, pkgjwt.JwtConfig{RefreshExpiration: 24 * time.Hour}, pkgjwt.JwtConfig{}, false, nil)

	r := gin.New()
	r.GET("/callback", h.H5Callback)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/callback?code=oauth-code&state=%2Forders%2F1", nil)
	r.ServeHTTP(w, req)

	resp := w.Result()
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusFound {
		t.Fatalf("expected status %d, got %d", http.StatusFound, resp.StatusCode)
	}
	if got := resp.Header.Get("Location"); got != "/orders/1" {
		t.Fatalf("expected redirect location %q, got %q", "/orders/1", got)
	}

	accessCookie := findCookie(t, resp.Cookies(), "access_token")
	if !accessCookie.HttpOnly {
		t.Fatal("expected access_token cookie to be HttpOnly")
	}
	if accessCookie.Secure {
		t.Fatal("expected access_token cookie to be insecure in non-prod handler")
	}
	if accessCookie.Path != "/" {
		t.Fatalf("expected access_token path %q, got %q", "/", accessCookie.Path)
	}
	if accessCookie.MaxAge != int(time.Hour.Seconds()) {
		t.Fatalf("expected access_token max age %d, got %d", int(time.Hour.Seconds()), accessCookie.MaxAge)
	}

	claims, err := pkgjwt.Parse("test-secret", accessCookie.Value)
	if err != nil {
		t.Fatalf("parse access token cookie: %v", err)
	}
	if claims.Sub != repo.h5Found.ID {
		t.Fatalf("expected access token subject %d, got %d", repo.h5Found.ID, claims.Sub)
	}
	if claims.Type != "user" {
		t.Fatalf("expected access token type %q, got %q", "user", claims.Type)
	}

	refreshCookie := findCookie(t, resp.Cookies(), "refresh_token")
	if !refreshCookie.HttpOnly {
		t.Fatal("expected refresh_token cookie to be HttpOnly")
	}
	if refreshCookie.Secure {
		t.Fatal("expected refresh_token cookie to be insecure in non-prod handler")
	}
	if refreshCookie.Path != refreshTokenCookiePath {
		t.Fatalf("expected refresh_token path %q, got %q", refreshTokenCookiePath, refreshCookie.Path)
	}
	if refreshCookie.MaxAge != int((24 * time.Hour).Seconds()) {
		t.Fatalf("expected refresh_token max age %d, got %d", int((24 * time.Hour).Seconds()), refreshCookie.MaxAge)
	}

	if repo.h5Upserted == nil {
		t.Fatal("expected H5 user to be upserted")
	}
	if repo.h5Upserted.OpenidH5 == nil || *repo.h5Upserted.OpenidH5 != openid {
		t.Fatalf("expected upsert openid_h5 %q, got %#v", openid, repo.h5Upserted.OpenidH5)
	}
	if repo.h5Upserted.OpenidMP != nil {
		t.Fatalf("expected openid_mp to stay nil for H5 login, got %#v", repo.h5Upserted.OpenidMP)
	}
}

func TestH5CallbackHTTPMissingCode(t *testing.T) {
	gin.SetMode(gin.TestMode)

	h := NewHandler(&Service{}, pkgjwt.JwtConfig{}, pkgjwt.JwtConfig{}, false, nil)
	r := gin.New()
	r.GET("/callback", h.H5Callback)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/callback?state=%2Forders%2F1", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected status %d, got %d", http.StatusUnauthorized, w.Code)
	}
}

func TestLogoutHTTPClearsCookiesAndBlacklistsTokens(t *testing.T) {
	gin.SetMode(gin.TestMode)
	pkgjwt.InitJwtConfig(pkgjwt.JwtConfig{
		JwtSecret:         "test-secret",
		Expiration:        time.Hour,
		RefreshExpiration: 24 * time.Hour,
	}, pkgjwt.JwtConfig{})

	redisServer := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: redisServer.Addr()})
	t.Cleanup(func() {
		_ = rdb.Close()
	})

	accessToken := signTestUserToken(t, "test-secret", 1001, "access-jti", time.Now().Add(time.Hour))
	refreshToken := signTestUserToken(t, "test-secret", 1001, "refresh-jti", time.Now().Add(24*time.Hour))
	h := NewHandler(NewService(nil, rdb, nil, nil), pkgjwt.GetUserConfig(), pkgjwt.JwtConfig{}, false, nil)

	r := gin.New()
	r.POST("/logout", h.Logout)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/logout", nil)
	req.AddCookie(&http.Cookie{Name: accessTokenCookieName, Value: accessToken, Path: accessTokenCookiePath})
	req.AddCookie(&http.Cookie{Name: refreshTokenCookieName, Value: refreshToken, Path: refreshTokenCookiePath})
	r.ServeHTTP(w, req)

	resp := w.Result()
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, resp.StatusCode)
	}

	accessCookie := findCookie(t, resp.Cookies(), accessTokenCookieName)
	if accessCookie.Path != accessTokenCookiePath {
		t.Fatalf("expected access_token delete path %q, got %q", accessTokenCookiePath, accessCookie.Path)
	}
	if accessCookie.MaxAge >= 0 {
		t.Fatalf("expected access_token delete max age < 0, got %d", accessCookie.MaxAge)
	}
	if accessCookie.Secure {
		t.Fatal("expected access_token delete cookie to be insecure in non-prod handler")
	}

	refreshCookie := findCookie(t, resp.Cookies(), refreshTokenCookieName)
	if refreshCookie.Path != refreshTokenCookiePath {
		t.Fatalf("expected refresh_token delete path %q, got %q", refreshTokenCookiePath, refreshCookie.Path)
	}
	if refreshCookie.MaxAge >= 0 {
		t.Fatalf("expected refresh_token delete max age < 0, got %d", refreshCookie.MaxAge)
	}
	if refreshCookie.Secure {
		t.Fatal("expected refresh_token delete cookie to be insecure in non-prod handler")
	}

	for _, key := range []string{"jwt:bl:access-jti", "jwt:bl:refresh-jti"} {
		exists, err := rdb.Exists(req.Context(), key).Result()
		if err != nil {
			t.Fatalf("check blacklist key %q: %v", key, err)
		}
		if exists != 1 {
			t.Fatalf("expected blacklist key %q to exist", key)
		}
	}
}

func findCookie(t *testing.T, cookies []*http.Cookie, name string) *http.Cookie {
	t.Helper()
	for _, cookie := range cookies {
		if cookie.Name == name {
			return cookie
		}
	}
	t.Fatalf("expected cookie %q", name)
	return nil
}
