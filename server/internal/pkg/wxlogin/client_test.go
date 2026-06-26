package wxlogin

import (
	"crypto/aes"
	"crypto/cipher"
	"encoding/base64"
	"testing"
)

func TestDecryptUserDataSuccess(t *testing.T) {
	client := NewClient("wx-test-appid", "secret")
	sessionKey := []byte("0123456789abcdef")
	iv := []byte("abcdef9876543210")
	payload := []byte(`{"phoneNumber":"13800138000","purePhoneNumber":"13800138000","countryCode":"86","watermark":{"appid":"wx-test-appid","timestamp":1710000000}}`)

	data, err := client.DecryptUserData(
		base64.StdEncoding.EncodeToString(sessionKey),
		encryptTestUserData(t, sessionKey, iv, payload),
		base64.StdEncoding.EncodeToString(iv),
	)
	if err != nil {
		t.Fatalf("DecryptUserData returned error: %v", err)
	}
	if got := data["purePhoneNumber"]; got != "13800138000" {
		t.Fatalf("expected purePhoneNumber %q, got %#v", "13800138000", got)
	}
}

func TestDecryptUserDataRejectsAppIDMismatch(t *testing.T) {
	client := NewClient("wx-other-appid", "secret")
	sessionKey := []byte("0123456789abcdef")
	iv := []byte("abcdef9876543210")
	payload := []byte(`{"purePhoneNumber":"13800138000","watermark":{"appid":"wx-test-appid","timestamp":1710000000}}`)

	_, err := client.DecryptUserData(
		base64.StdEncoding.EncodeToString(sessionKey),
		encryptTestUserData(t, sessionKey, iv, payload),
		base64.StdEncoding.EncodeToString(iv),
	)
	if err == nil {
		t.Fatal("expected appid mismatch error")
	}
}

func encryptTestUserData(t *testing.T, key, iv, payload []byte) string {
	t.Helper()

	block, err := aes.NewCipher(key)
	if err != nil {
		t.Fatalf("new test cipher: %v", err)
	}
	plainText := pkcs7Pad(payload, aes.BlockSize)
	cipherText := make([]byte, len(plainText))
	cipher.NewCBCEncrypter(block, iv).CryptBlocks(cipherText, plainText)
	return base64.StdEncoding.EncodeToString(cipherText)
}

func pkcs7Pad(data []byte, blockSize int) []byte {
	padding := blockSize - len(data)%blockSize
	padded := make([]byte, len(data)+padding)
	copy(padded, data)
	for i := len(data); i < len(padded); i++ {
		padded[i] = byte(padding)
	}
	return padded
}
