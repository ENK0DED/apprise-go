package notify

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha1"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
	"net/url"
	"strings"
)

const simplepushURL = "https://api.simplepush.io/send"

const simplepushBlockSize = aes.BlockSize

type SimplePushTarget struct {
	apiKey   string
	event    string
	user     string
	password string
	iv       []byte
	ivHex    string
	key      []byte
}

func NewSimplePushTarget(target *ParsedURL) (*SimplePushTarget, error) {
	apiKey := strings.TrimSpace(target.Host)
	if apiKey == "" || strings.ContainsAny(apiKey, " \t\n\r") {
		return nil, fmt.Errorf("missing api key")
	}

	event := strings.TrimSpace(target.Query["event"])

	return &SimplePushTarget{
		apiKey:   apiKey,
		event:    event,
		user:     target.User,
		password: target.Password,
	}, nil
}

func (s *SimplePushTarget) BuildRequest(body, title string, notifyType NotifyType) (RequestSpec, error) {
	payload := map[string]string{
		"key":   s.apiKey,
		"msg":   body,
		"title": title,
	}

	if s.user != "" && s.password != "" {
		encryptedBody, err := s.encrypt(body)
		if err != nil {
			return RequestSpec{}, err
		}
		encryptedTitle, err := s.encrypt(title)
		if err != nil {
			return RequestSpec{}, err
		}
		payload["msg"] = encryptedBody
		payload["title"] = encryptedTitle
		payload["encrypted"] = "true"
		payload["iv"] = s.ivHex
	}

	if s.event != "" {
		payload["event"] = s.event
	}

	values := url.Values{}
	for key, value := range payload {
		values.Set(key, value)
	}

	_ = notifyType

	return RequestSpec{
		Method: "POST",
		URL:    simplepushURL,
		Headers: map[string]string{
			"User-Agent":   "Apprise",
			"Accept":       "*/*",
			"Content-Type": "application/x-www-form-urlencoded",
		},
		Body: values.Encode(),
	}, nil
}

func (s *SimplePushTarget) Send(body, title string, notifyType NotifyType) error {
	spec, err := s.BuildRequest(body, title, notifyType)
	if err != nil {
		return err
	}

	return SendRequest(spec)
}

func (s *SimplePushTarget) encrypt(content string) (string, error) {
	if err := s.ensureEncryption(); err != nil {
		return "", err
	}

	padded := pkcs7Pad([]byte(content), simplepushBlockSize)
	block, err := aes.NewCipher(s.key)
	if err != nil {
		return "", err
	}

	ciphertext := make([]byte, len(padded))
	mode := cipher.NewCBCEncrypter(block, s.iv)
	mode.CryptBlocks(ciphertext, padded)

	return base64.URLEncoding.EncodeToString(ciphertext), nil
}

func (s *SimplePushTarget) ensureEncryption() error {
	if s.iv != nil && s.key != nil {
		return nil
	}

	iv := make([]byte, simplepushBlockSize)
	if _, err := io.ReadFull(rand.Reader, iv); err != nil {
		return err
	}

	hash := sha1.Sum([]byte(s.password + s.user))
	hexDigest := hex.EncodeToString(hash[:])
	keyBytes, err := hex.DecodeString(hexDigest[:32])
	if err != nil {
		return err
	}

	s.iv = iv
	s.ivHex = strings.ToUpper(hex.EncodeToString(iv))
	s.key = keyBytes
	return nil
}

func pkcs7Pad(input []byte, blockSize int) []byte {
	pad := blockSize - (len(input) % blockSize)
	if pad == 0 {
		pad = blockSize
	}
	padded := make([]byte, 0, len(input)+pad)
	padded = append(padded, input...)
	for i := 0; i < pad; i++ {
		padded = append(padded, byte(pad))
	}
	return padded
}
