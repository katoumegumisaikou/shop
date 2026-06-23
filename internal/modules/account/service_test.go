package account

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"

	"shop/internal/pkg/errs"
	pkgjwt "shop/internal/pkg/jwt"
	"shop/internal/pkg/snowflake"
	"shop/internal/pkg/wxlogin"
)

type mockUserRepo struct {
	upserted                     *User
	found                        *User
	h5Upserted                   *User
	h5Found                      *User
	countActiveByPhoneExclude    int64
	countActiveByPhoneExcludeErr error
	updatedID                    int64
	updates                      map[string]any
	updateErr                    error
}

func (m *mockUserRepo) FindByOpenidMP(_ context.Context, openid string) (*User, error) {
	if m.found != nil && m.found.OpenidMP != nil && *m.found.OpenidMP == openid {
		return m.found, nil
	}
	return nil, nil
}

func (m *mockUserRepo) UpsertByOpenidMP(_ context.Context, user *User) error {
	m.upserted = user
	return nil
}

func (m *mockUserRepo) FindByOpenidH5(_ context.Context, openid string) (*User, error) {
	if m.h5Found != nil && m.h5Found.OpenidH5 != nil && *m.h5Found.OpenidH5 == openid {
		return m.h5Found, nil
	}
	return nil, nil
}

func (m *mockUserRepo) UpsertByOpenidH5(_ context.Context, user *User) error {
	m.h5Upserted = user
	return nil
}

func (m *mockUserRepo) CountActiveByPhoneExclude(_ context.Context, _ string, _ int64) (int64, error) {
	return m.countActiveByPhoneExclude, m.countActiveByPhoneExcludeErr
}

func (m *mockUserRepo) Update(_ context.Context, id int64, updates map[string]any) error {
	m.updatedID = id
	m.updates = updates
	return m.updateErr
}

type mockWxLoginClient struct {
	resp      *wxlogin.Code2SessionResp
	err       error
	oauthResp *wxlogin.OAuthResp
	oauthErr  error
}

func (m mockWxLoginClient) Code2Session(context.Context, string) (*wxlogin.Code2SessionResp, error) {
	return m.resp, m.err
}

func (m mockWxLoginClient) DecryptUserData(string, string, string) (map[string]any, error) {
	return nil, nil
}

func (m mockWxLoginClient) GetOAuthURL(string, string) string {
	return ""
}

func (m mockWxLoginClient) OAuthCode2Token(context.Context, string) (*wxlogin.OAuthResp, error) {
	return m.oauthResp, m.oauthErr
}

func TestMpLoginSuccess(t *testing.T) {
	ctx := context.Background()
	snowflake.Init(1)
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

	openid := "mp-openid-001"
	unionid := "unionid-001"
	repo := &mockUserRepo{
		found: &User{
			ID:       1001,
			OpenidMP: &openid,
			Unionid:  &unionid,
			Status:   "active",
		},
	}
	wx := mockWxLoginClient{
		resp: &wxlogin.Code2SessionResp{
			OpenID:     openid,
			UnionID:    unionid,
			SessionKey: "session-key-001",
		},
	}

	svc := NewService(repo, rdb, wx, nil)
	result, err := svc.MpLogin(ctx, "login-code")
	if err != nil {
		t.Fatalf("MpLogin returned error: %v", err)
	}
	if result == nil {
		t.Fatal("expected login result, got nil")
	}
	if result.UserID != repo.found.ID {
		t.Fatalf("expected result user id %d, got %d", repo.found.ID, result.UserID)
	}
	if result.AccessToken == "" {
		t.Fatal("expected access token")
	}
	if result.RefreshToken == "" {
		t.Fatal("expected refresh token")
	}

	if repo.upserted == nil {
		t.Fatal("expected user to be upserted")
	}
	if repo.upserted.OpenidMP == nil || *repo.upserted.OpenidMP != openid {
		t.Fatalf("expected upsert openid %q, got %#v", openid, repo.upserted.OpenidMP)
	}
	if repo.upserted.Unionid == nil || *repo.upserted.Unionid != unionid {
		t.Fatalf("expected upsert unionid %q, got %#v", unionid, repo.upserted.Unionid)
	}

	sessionKey, err := rdb.Get(ctx, "mp:1001").Result()
	if err != nil {
		t.Fatalf("expected cached session key: %v", err)
	}
	if sessionKey != "session-key-001" {
		t.Fatalf("expected cached session key %q, got %q", "session-key-001", sessionKey)
	}

	claims, err := pkgjwt.Parse("test-secret", result.AccessToken)
	if err != nil {
		t.Fatalf("parse access token: %v", err)
	}
	if claims.Sub != repo.found.ID {
		t.Fatalf("expected token subject %d, got %d", repo.found.ID, claims.Sub)
	}
}

func TestMpLoginMissingDependency(t *testing.T) {
	svc := &Service{}

	result, err := svc.MpLogin(context.Background(), "login-code")
	if err == nil {
		t.Fatal("expected missing dependency error")
	}
	if result != nil {
		t.Fatalf("expected nil result, got %#v", result)
	}
	if !errors.Is(err, errs.ErrInternal) {
		t.Fatalf("expected internal error, got %v", err)
	}
}

func TestBindPhoneMissingDependency(t *testing.T) {
	svc := &Service{}

	err := svc.BindPhone(context.Background(), 1001, "encrypted-data", "iv")
	if err == nil {
		t.Fatal("expected missing dependency error")
	}
	if !errors.Is(err, errs.ErrInternal) {
		t.Fatalf("expected internal error, got %v", err)
	}
}

func TestH5CallbackSuccessUsesOpenidH5(t *testing.T) {
	ctx := context.Background()
	snowflake.Init(1)
	pkgjwt.InitJwtConfig(pkgjwt.JwtConfig{
		JwtSecret:         "test-secret",
		Expiration:        time.Hour,
		RefreshExpiration: 24 * time.Hour,
	}, pkgjwt.JwtConfig{})

	openid := "h5-openid-001"
	unionid := "unionid-001"
	repo := &mockUserRepo{
		h5Found: &User{
			ID:       2002,
			OpenidH5: &openid,
			Unionid:  &unionid,
			Source:   "h5",
			Status:   "active",
		},
	}
	wx := mockWxLoginClient{
		oauthResp: &wxlogin.OAuthResp{
			OpenID:       openid,
			UnionID:      unionid,
			AccessToken:  "wx-access-token",
			RefreshToken: "wx-refresh-token",
			ExpiresIn:    7200,
		},
	}

	svc := NewService(repo, nil, nil, wx)
	result, err := svc.H5Callback(ctx, "oauth-code")
	if err != nil {
		t.Fatalf("H5Callback returned error: %v", err)
	}
	if result == nil {
		t.Fatal("expected login result, got nil")
	}
	if result.UserID != repo.h5Found.ID {
		t.Fatalf("expected result user id %d, got %d", repo.h5Found.ID, result.UserID)
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
	if repo.h5Upserted.Source != "h5" {
		t.Fatalf("expected source h5, got %q", repo.h5Upserted.Source)
	}

	claims, err := pkgjwt.Parse("test-secret", result.AccessToken)
	if err != nil {
		t.Fatalf("parse access token: %v", err)
	}
	if claims.Sub != repo.h5Found.ID {
		t.Fatalf("expected token subject %d, got %d", repo.h5Found.ID, claims.Sub)
	}
}
