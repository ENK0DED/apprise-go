package notify

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha1"
	"encoding/base64"
	"fmt"
	"net/url"
	"sort"
	"strings"
)

type oauthParam struct {
	key   string
	value string
}

func buildOAuth1Header(method, rawURL string, bodyParams url.Values, consumerKey, consumerSecret, token, tokenSecret string) (string, error) {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return "", err
	}

	nonce := oauthNonce()
	timestamp := oauthTimestamp()
	oauthParams := []oauthParam{
		{key: "oauth_nonce", value: nonce},
		{key: "oauth_timestamp", value: timestamp},
		{key: "oauth_version", value: "1.0"},
		{key: "oauth_signature_method", value: "HMAC-SHA1"},
		{key: "oauth_consumer_key", value: consumerKey},
		{key: "oauth_token", value: token},
	}

	signature, err := oauth1Signature(method, parsed, bodyParams, oauthParams, consumerSecret, tokenSecret)
	if err != nil {
		return "", err
	}

	headerParts := []string{
		fmt.Sprintf(`oauth_nonce="%s"`, oauthEscape(nonce)),
		fmt.Sprintf(`oauth_timestamp="%s"`, oauthEscape(timestamp)),
		`oauth_version="1.0"`,
		`oauth_signature_method="HMAC-SHA1"`,
		fmt.Sprintf(`oauth_consumer_key="%s"`, oauthEscape(consumerKey)),
		fmt.Sprintf(`oauth_token="%s"`, oauthEscape(token)),
		fmt.Sprintf(`oauth_signature="%s"`, oauthEscape(signature)),
	}

	return "OAuth " + strings.Join(headerParts, ", "), nil
}

func oauth1Signature(method string, parsed *url.URL, bodyParams url.Values, oauthParams []oauthParam, consumerSecret, tokenSecret string) (string, error) {
	params := make([]oauthParam, 0, len(oauthParams))
	params = append(params, oauthParams...)

	for key, values := range parsed.Query() {
		for _, value := range values {
			params = append(params, oauthParam{key: key, value: value})
		}
	}

	for key, values := range bodyParams {
		for _, value := range values {
			params = append(params, oauthParam{key: key, value: value})
		}
	}

	encodedParams := make([]oauthParam, 0, len(params))
	for _, param := range params {
		encodedParams = append(encodedParams, oauthParam{
			key:   oauthEscape(param.key),
			value: oauthEscape(param.value),
		})
	}

	sort.Slice(encodedParams, func(i, j int) bool {
		if encodedParams[i].key == encodedParams[j].key {
			return encodedParams[i].value < encodedParams[j].value
		}
		return encodedParams[i].key < encodedParams[j].key
	})

	var parameterString bytes.Buffer
	for i, param := range encodedParams {
		if i > 0 {
			parameterString.WriteByte('&')
		}
		parameterString.WriteString(param.key)
		parameterString.WriteByte('=')
		parameterString.WriteString(param.value)
	}

	baseURL := parsed.Scheme + "://" + parsed.Host + parsed.Path
	baseString := strings.ToUpper(method) + "&" + oauthEscape(baseURL) + "&" + oauthEscape(parameterString.String())

	signingKey := oauthEscape(consumerSecret) + "&" + oauthEscape(tokenSecret)
	mac := hmac.New(sha1.New, []byte(signingKey))
	_, _ = mac.Write([]byte(baseString))
	signature := base64.StdEncoding.EncodeToString(mac.Sum(nil))
	return signature, nil
}

func oauthEscape(value string) string {
	escaped := url.QueryEscape(value)
	escaped = strings.ReplaceAll(escaped, "+", "%20")
	escaped = strings.ReplaceAll(escaped, "%7E", "~")
	return escaped
}
