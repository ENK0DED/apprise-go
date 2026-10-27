package notify

import (
	"bytes"
	"crypto"
	"encoding/base64"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/textproto"
	"os"
	"path/filepath"
	"strings"

	"github.com/ProtonMail/go-crypto/openpgp"
	"github.com/ProtonMail/go-crypto/openpgp/armor"
	"github.com/ProtonMail/go-crypto/openpgp/packet"
)

type mailPGP struct {
	senderAddress                 string
	mode, publicPath, privatePath string
	wkd                           bool
	parsed                        *ParsedURL
	sender                        *openpgp.Entity
	recipients                    map[string]*openpgp.Entity
}

func newMailPGP(parsed *ParsedURL) (*mailPGP, error) {
	mode := strings.ToLower(strings.TrimSpace(parsed.Query["pgp"]))
	explicit := mode != ""
	switch {
	case mode == "" || mode == "none" || strings.HasPrefix("no", mode):
		mode = "no"
	case strings.HasPrefix("sign", mode):
		mode = "sign"
	case strings.HasPrefix("encrypt", mode):
		mode = "encrypt"
	default:
		return nil, fmt.Errorf("invalid pgp mode")
	}
	wkd := parseBoolWithDefault(parsed.Query["wkd"], false)
	if wkd && !explicit {
		mode = "encrypt"
	}
	if mode == "no" {
		return nil, nil
	}
	return &mailPGP{mode: mode, publicPath: parsed.Query["pgppub"], privatePath: parsed.Query["pgpprv"], wkd: wkd, parsed: parsed, recipients: map[string]*openpgp.Entity{}}, nil
}
func readMailPGPKey(path string, private bool) (openpgp.EntityList, error) {
	limit := int64(8000)
	if private {
		limit = 32000
	}
	var data []byte
	var err error
	if strings.HasPrefix(path, "https://") || strings.HasPrefix(path, "http://") {
		data, err = readHTTPAttachment(path, limit)
	} else {
		file, openErr := os.Open(expandStorePath(strings.TrimPrefix(path, "file://")))
		if openErr != nil {
			return nil, fmt.Errorf("could not open PGP key")
		}
		defer file.Close()
		data, err = io.ReadAll(io.LimitReader(file, limit+1))
	}
	if err != nil || int64(len(data)) > limit {
		return nil, fmt.Errorf("could not read bounded PGP key")
	}
	var keys openpgp.EntityList
	if bytes.Contains(data, []byte("-----BEGIN PGP")) {
		keys, err = openpgp.ReadArmoredKeyRing(bytes.NewReader(data))
	} else {
		keys, err = openpgp.ReadKeyRing(bytes.NewReader(data))
	}
	if err != nil || len(keys) == 0 {
		return nil, fmt.Errorf("invalid OpenPGP key")
	}
	return keys, nil
}
func mailPGPMatches(key *openpgp.Entity, email string) bool {
	for _, identity := range key.Identities {
		if strings.EqualFold(identity.UserId.Email, email) {
			return true
		}
	}
	return false
}
func (p *mailPGP) cacheDirectory() string {
	if disk, ok := StoreFor(p.parsed).(*diskStore); ok {
		return filepath.Dir(disk.path)
	}
	return ""
}
func (p *mailPGP) keyPath(email string, private bool) string {
	if private && p.privatePath != "" {
		return p.privatePath
	}
	if !private && p.publicPath != "" {
		return p.publicPath
	}
	dir := p.cacheDirectory()
	if dir == "" {
		return ""
	}
	suffix := "pub"
	full := "public"
	if private {
		suffix = "prv"
		full = "private"
	}
	email = strings.ToLower(email)
	local, _, _ := strings.Cut(email, "@")
	for _, name := range []string{email + "-" + suffix + ".asc", local + "-" + suffix + ".asc", "pgp-" + full + ".asc", "pgp-" + suffix + ".asc", full + ".asc", suffix + ".asc"} {
		if strings.ContainsAny(name, "/\\") {
			continue
		}
		path := filepath.Join(dir, name)
		if info, err := os.Stat(path); err == nil && info.Mode().IsRegular() {
			return path
		}
	}
	return ""
}
func (p *mailPGP) recipient(email string) (*openpgp.Entity, error) {
	originalEmail := email
	email = strings.ToLower(email)
	if key := p.recipients[email]; key != nil {
		return key, nil
	}
	path := p.keyPath(email, false)
	if path != "" {
		keys, err := readMailPGPKey(path, false)
		if err != nil {
			return nil, err
		}
		for _, key := range keys {
			if mailPGPMatches(key, email) {
				p.recipients[email] = key
				return key, nil
			}
		}
		return nil, fmt.Errorf("PGP public key does not identify recipient")
	}
	if strings.EqualFold(email, p.senderAddress) && p.publicPath == "" {
		key, err := p.signer(p.senderAddress)
		if err != nil {
			return nil, err
		}
		p.recipients[email] = key
		return key, nil
	}
	if p.wkd {
		keys, err := fetchWKDKeys(originalEmail, p.parsed)
		if err != nil {
			return nil, err
		}
		for _, key := range keys {
			if mailPGPMatches(key, email) {
				p.recipients[email] = key
				return key, nil
			}
		}
	}
	return nil, fmt.Errorf("no OpenPGP public key for recipient; supply pgppub or enable WKD")
}
func (p *mailPGP) signer(email string) (*openpgp.Entity, error) {
	if p.sender != nil {
		return p.sender, nil
	}
	path := p.keyPath(email, true)
	if path == "" {
		if p.privatePath == "" && p.publicPath == "" && p.cacheDirectory() != "" {
			key, err := p.generateSender(email)
			if err != nil {
				return nil, err
			}
			p.sender = key
			return key, nil
		}
		return nil, fmt.Errorf("no OpenPGP private key; supply pgpprv")
	}
	keys, err := readMailPGPKey(path, true)
	if err != nil {
		return nil, err
	}
	for _, key := range keys {
		if key.PrivateKey != nil && mailPGPMatches(key, email) {
			if key.PrivateKey.Encrypted {
				return nil, fmt.Errorf("PGP private key is locked")
			}
			p.sender = key
			return key, nil
		}
	}
	return nil, fmt.Errorf("PGP private key does not identify sender")
}
func pgpContainer(subtype string, params map[string]string, parts []struct {
	header textproto.MIMEHeader
	body   string
}) (string, string, error) {
	var out bytes.Buffer
	writer := multipart.NewWriter(&out)
	for _, part := range parts {
		target, err := writer.CreatePart(part.header)
		if err != nil {
			return "", "", err
		}
		if _, err = io.WriteString(target, part.body); err != nil {
			return "", "", err
		}
	}
	if err := writer.Close(); err != nil {
		return "", "", err
	}
	params["boundary"] = writer.Boundary()
	return mime.FormatMediaType("multipart/"+subtype, params), out.String(), nil
}
func (p *mailPGP) secure(contentType, transfer, body, recipient, sender, subject string) (string, string, string, string, error) {
	p.senderAddress = sender
	inner := "MIME-Version: 1.0\r\nContent-Type: " + contentType + "\r\n"
	if transfer != "" {
		inner += "Content-Transfer-Encoding: " + transfer + "\r\n"
	}
	inner += "\r\n" + body
	config := &packet.Config{DefaultHash: crypto.SHA256, DefaultCipher: packet.CipherAES256}
	autocrypt := ""
	if p.mode == "sign" {
		signer, err := p.signer(sender)
		if err != nil {
			return "", "", "", "", err
		}
		var signature bytes.Buffer
		if err = openpgp.ArmoredDetachSign(&signature, signer, strings.NewReader(inner), config); err != nil {
			return "", "", "", "", err
		}

		boundary := multipart.NewWriter(io.Discard).Boundary()
		contentType = mime.FormatMediaType("multipart/signed", map[string]string{"protocol": "application/pgp-signature", "micalg": "pgp-sha256", "boundary": boundary})
		body = "--" + boundary + "\r\n" + inner + "\r\n--" + boundary + "\r\nContent-Type: application/pgp-signature; name=\"signature.asc\"\r\nContent-Description: OpenPGP digital signature\r\nContent-Disposition: attachment; filename=\"signature.asc\"\r\n\r\n" + signature.String() + "\r\n--" + boundary + "--\r\n"

		inner = "MIME-Version: 1.0\r\nContent-Type: " + contentType + "\r\n\r\n" + body
		var public bytes.Buffer
		if err = signer.Serialize(&public); err != nil {
			return "", "", "", "", err
		}
		autocrypt = mailAutocryptHeader(sender, public.Bytes())
	}
	key, keyErr := p.recipient(recipient)
	if p.mode == "encrypt" && keyErr != nil {
		return "", "", "", "", keyErr
	}
	if keyErr == nil {
		if p.mode == "encrypt" {
			inner = "From: " + sender + "\r\nTo: " + recipient + "\r\nSubject: " + subject + "\r\n" + inner
		}
		var encrypted bytes.Buffer
		armored, err := armor.Encode(&encrypted, "PGP MESSAGE", nil)
		if err != nil {
			return "", "", "", "", err
		}
		plain, err := openpgp.Encrypt(armored, []*openpgp.Entity{key}, nil, &openpgp.FileHints{IsBinary: true}, config)
		if err != nil {
			return "", "", "", "", err
		}
		if _, err = io.WriteString(plain, inner); err != nil {
			return "", "", "", "", err
		}
		if err = plain.Close(); err != nil {
			return "", "", "", "", err
		}
		if err = armored.Close(); err != nil {
			return "", "", "", "", err
		}
		contentType, body, err = pgpContainer("encrypted", map[string]string{"protocol": "application/pgp-encrypted"}, []struct {
			header textproto.MIMEHeader
			body   string
		}{{textproto.MIMEHeader{"Content-Type": {"application/pgp-encrypted"}}, "Version: 1"}, {textproto.MIMEHeader{"Content-Type": {"application/octet-stream"}}, encrypted.String()}})
		if err != nil {
			return "", "", "", "", err
		}
	}
	if p.mode == "encrypt" {
		if signer, err := p.signer(sender); err == nil {
			var public bytes.Buffer
			if err = signer.Serialize(&public); err == nil {
				autocrypt = mailAutocryptHeader(sender, public.Bytes())
			}
		}
	}
	return contentType, "", body, autocrypt, nil
}

// Auto-generation is limited to the sender's persistent private directory.
// Atomic exclusive publication prevents two callers from replacing identities.
func (p *mailPGP) generateSender(email string) (*openpgp.Entity, error) {
	dir := p.cacheDirectory()
	if dir == "" {
		return nil, fmt.Errorf("PGP key generation requires persistent storage")
	}
	if strings.ContainsAny(email, "/\\") {
		return nil, fmt.Errorf("invalid PGP cache identity")
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, err
	}
	key, err := openpgp.NewEntity("", "", email, &packet.Config{RSABits: 2048, DefaultHash: crypto.SHA256, DefaultCipher: packet.CipherAES256})
	if err != nil {
		return nil, err
	}
	privatePath := filepath.Join(dir, strings.ToLower(email)+"-prv.asc")
	var private bytes.Buffer
	armored, err := armor.Encode(&private, openpgp.PrivateKeyType, nil)
	if err != nil {
		return nil, err
	}
	if err = key.SerializePrivate(armored, nil); err != nil {
		return nil, err
	}
	if err = armored.Close(); err != nil {
		return nil, err
	}
	if err = publishPGPKey(privatePath, private.Bytes()); err != nil {
		if !os.IsExist(err) {
			return nil, err
		}
		keys, readErr := readMailPGPKey(privatePath, true)
		if readErr != nil {
			return nil, readErr
		}
		key = keys[0]
	}
	var public bytes.Buffer
	armored, err = armor.Encode(&public, openpgp.PublicKeyType, nil)
	if err != nil {
		return nil, err
	}
	if err = key.Serialize(armored); err != nil {
		return nil, err
	}
	if err = armored.Close(); err != nil {
		return nil, err
	}
	err = publishPGPKey(filepath.Join(dir, strings.ToLower(email)+"-pub.asc"), public.Bytes())
	if err != nil && !os.IsExist(err) {
		return nil, err
	}
	return key, nil
}
func publishPGPKey(path string, data []byte) error {
	file, err := os.CreateTemp(filepath.Dir(path), ".pgp-key-*")
	if err != nil {
		return err
	}
	name := file.Name()
	defer os.Remove(name)
	if err = file.Chmod(0600); err != nil {
		_ = file.Close()
		return err
	}
	if _, err = file.Write(data); err != nil {
		_ = file.Close()
		return err
	}
	if err = file.Close(); err != nil {
		return err
	}
	return os.Link(name, path)
}

func mailAutocryptHeader(sender string, public []byte) string {
	prefix := "addr=" + sender + "; prefer-encrypt=mutual; keydata="
	data := base64.StdEncoding.EncodeToString(public)
	if len(prefix)+len(data) > 10*1024 {
		return ""
	}
	var lines []string
	for len(data) > 76 {
		lines = append(lines, data[:76])
		data = data[76:]
	}
	lines = append(lines, data)
	return prefix + strings.Join(lines, "\r\n ")
}
