package notify

import "encoding/base64"

func basicAuthHeader(user, password string) string {
	token := user + ":" + password
	return "Basic " + base64.StdEncoding.EncodeToString([]byte(token))
}
