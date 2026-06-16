package wxlogin

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"
)

// Code2SessionResp 小程序 code2session 响应。
type Code2SessionResp struct {
	OpenID     string `json:"openid"`
	UnionID    string `json:"unionid"`
	SessionKey string `json:"session_key"`
	ErrCode    int    `json:"errcode"`
	ErrMsg     string `json:"errmsg"`
}

// WxLoginClient 微信登录能力接口。
type WxLoginClient interface {
	// Code2Session 小程序登录，用 code 换取 openid + session_key。
	Code2Session(ctx context.Context, code string) (*Code2SessionResp, error)
	// DecryptUserData 解密微信加密数据（如手机号）。
	DecryptUserData(sessionKey, encryptedData, iv string) (map[string]any, error)
	// GetOAuthURL 生成公众号 OAuth2 授权 URL。
	GetOAuthURL(redirectURI, state string) string
	// OAuthCode2Token 公众号用 code 换 access_token + openid。
	//OAuthCode2Token(ctx context.Context, code string) (*OAuthResp, error)
}

// Client 微信 API 客户端
type Client struct {
	appID      string
	appSecret  string
	httpClient *http.Client
}

func NewClient(id, secret string) *Client {
	return &Client{
		appID:     id,
		appSecret: secret,
		httpClient: &http.Client{
			Timeout: 5 * time.Second,
		},
	}
}

func (c *Client) Code2Session(ctx context.Context, code string) (*Code2SessionResp, error) {
	apiURL := fmt.Sprintf(
		"https://api.weixin.qq.com/sns/jscode2session?appid=%s&secret=%s&js_code=%s&grant_type=authorization_code",
		c.appID, c.appSecret, url.QueryEscape(code),
	)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("wx code2session status: %d", resp.StatusCode)
	}

	var result Code2SessionResp
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode wx code2session response: %w", err)
	}

	return &result, nil
}
