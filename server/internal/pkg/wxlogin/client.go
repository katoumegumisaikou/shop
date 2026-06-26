package wxlogin

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"encoding/base64"
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

// OAuthResp 是公众号 OAuth2 access_token 接口响应。
type OAuthResp struct {
	AccessToken  string `json:"access_token"`  // 公众号网页授权 access_token
	ExpiresIn    int    `json:"expires_in"`    // access_token 有效期，单位秒
	RefreshToken string `json:"refresh_token"` // 刷新 access_token 的 token
	Scope        string `json:"scope"`         // 授权范围：snsapi_base 或 snsapi_userinfo
	OpenID       string `json:"openid"`        // 用户在当前公众号下的唯一 ID
	UnionID      string `json:"unionid"`       // 用户在微信开放平台下的统一 ID，满足条件时返回

	ErrCode int    `json:"errcode"`
	ErrMsg  string `json:"errmsg"`
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
	OAuthCode2Token(ctx context.Context, code string) (*OAuthResp, error)
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

// Code2Session 调用微信 jscode2session 接口并处理错误码
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
	if result.OpenID == "" || result.SessionKey == "" {
		return nil, fmt.Errorf("wx code2session missing openid or session_key")
	}

	return &result, nil
}

// DecryptUserData 解密微信小程序加密数据，并校验数据归属的 appid。
func (c *Client) DecryptUserData(sessionKey, encryptedData, iv string) (map[string]any, error) {
	key, err := base64.StdEncoding.DecodeString(sessionKey)
	if err != nil {
		return nil, fmt.Errorf("decode wx session key: %w", err)
	}
	if len(key) != aes.BlockSize {
		return nil, fmt.Errorf("wx session key length: %d", len(key))
	}

	cipherText, err := base64.StdEncoding.DecodeString(encryptedData)
	if err != nil {
		return nil, fmt.Errorf("decode wx encrypted data: %w", err)
	}
	if len(cipherText) == 0 || len(cipherText)%aes.BlockSize != 0 {
		return nil, fmt.Errorf("wx encrypted data length: %d", len(cipherText))
	}

	ivBytes, err := base64.StdEncoding.DecodeString(iv)
	if err != nil {
		return nil, fmt.Errorf("decode wx iv: %w", err)
	}
	if len(ivBytes) != aes.BlockSize {
		return nil, fmt.Errorf("wx iv length: %d", len(ivBytes))
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("new wx aes cipher: %w", err)
	}

	plainText := make([]byte, len(cipherText))
	cipher.NewCBCDecrypter(block, ivBytes).CryptBlocks(plainText, cipherText)
	plainText, err = pkcs7Unpad(plainText, aes.BlockSize)
	if err != nil {
		return nil, err
	}

	var result map[string]any
	if err := json.Unmarshal(plainText, &result); err != nil {
		return nil, fmt.Errorf("decode wx decrypted data: %w", err)
	}
	if err := c.validateWatermark(result); err != nil {
		return nil, err
	}
	return result, nil
}

func pkcs7Unpad(data []byte, blockSize int) ([]byte, error) {
	if len(data) == 0 || len(data)%blockSize != 0 {
		return nil, fmt.Errorf("wx decrypted data length: %d", len(data))
	}
	padding := int(data[len(data)-1])
	if padding == 0 || padding > blockSize || padding > len(data) {
		return nil, fmt.Errorf("wx decrypted data padding: %d", padding)
	}
	for _, value := range data[len(data)-padding:] {
		if int(value) != padding {
			return nil, fmt.Errorf("wx decrypted data padding mismatch")
		}
	}
	return data[:len(data)-padding], nil
}

func (c *Client) validateWatermark(data map[string]any) error {
	rawWatermark, ok := data["watermark"]
	if !ok {
		return fmt.Errorf("wx decrypted data missing watermark")
	}
	watermark, ok := rawWatermark.(map[string]any)
	if !ok {
		return fmt.Errorf("wx decrypted data invalid watermark")
	}
	appID, ok := watermark["appid"].(string)
	if !ok || appID == "" {
		return fmt.Errorf("wx decrypted data missing appid")
	}
	if c.appID != "" && appID != c.appID {
		return fmt.Errorf("wx decrypted data appid mismatch")
	}
	return nil
}

// GetOAuthURL 生成公众号 OAuth2 授权跳转 URL。
//
// redirectURI 是微信授权完成后的回调地址，state 跳回的前端页面
func (c *Client) GetOAuthURL(redirectURI, state string) string {
	query := url.Values{}
	query.Set("appid", c.appID)
	query.Set("redirect_uri", redirectURI)
	query.Set("response_type", "code")
	query.Set("scope", "snsapi_userinfo")
	query.Set("state", state)

	return "https://open.weixin.qq.com/connect/oauth2/authorize?" + query.Encode() + "#wechat_redirect"
}

// OAuthCode2Token 用公众号网页授权 code 换取 access_token 和 openid。
//
// 这个 code 来自公众号 OAuth2 回调，不同于小程序 wx.login 的 code。
func (c *Client) OAuthCode2Token(ctx context.Context, code string) (*OAuthResp, error) {
	apiURL := fmt.Sprintf(
		"https://api.weixin.qq.com/sns/oauth2/access_token?appid=%s&secret=%s&code=%s&grant_type=authorization_code",
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
		return nil, fmt.Errorf("wx oauth2 access_token status: %d", resp.StatusCode)
	}

	var result OAuthResp
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode wx oauthcode2token response: %w", err)
	}
	if result.OpenID == "" || result.ErrCode != 0 {
		return nil, fmt.Errorf("wx oauth2 access_token failed: code=%d msg=%s", result.ErrCode, result.ErrMsg)
	}

	return &result, nil
}
