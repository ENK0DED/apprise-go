package notify

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
	"os"

	"golang.org/x/crypto/argon2"
	"golang.org/x/crypto/chacha20poly1305"
	"golang.org/x/crypto/nacl/box"
)

// Simplepush v1 derives a 64-byte Argon2id master, then expands the message
// key with HKDF-SHA256's single output block. The fingerprint identifies that
// key without sending either the password or its salt-derived master.
func deriveSimplePushKey(password, salt string) ([]byte, string) {
	saltHash := sha256.Sum256([]byte(salt))
	master := argon2.IDKey([]byte(password), saltHash[:16], 3, 64*1024, 1, 64)
	mac := hmac.New(sha256.New, master)
	mac.Write([]byte("symmetric-key\x01"))
	key := mac.Sum(nil)
	fingerprint := sha256.Sum256(key)
	return key, base64.StdEncoding.EncodeToString(fingerprint[:8])
}

func sealSimplePush(data, key []byte) ([]byte, error) {
	aead, err := chacha20poly1305.NewX(key)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, chacha20poly1305.NonceSizeX)
	if override := os.Getenv("APPRISE_SIMPLEPUSH_TEST_NONCE"); override != "" {
		decoded, err := hex.DecodeString(override)
		if err != nil || len(decoded) != len(nonce) {
			return nil, fmt.Errorf("invalid Simplepush test nonce")
		}
		copy(nonce, decoded)
	} else if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}
	return aead.Seal(nonce, nonce, data, nil), nil
}

func openSimplePushOrganizationKey(seed, adminPublic, wrapped []byte) ([]byte, error) {
	if len(seed) != 32 || len(adminPublic) != 32 || len(wrapped) < 24+box.Overhead {
		return nil, fmt.Errorf("invalid Simplepush wrapped organization key")
	}
	// libsodium crypto_box_seed_keypair derives the private scalar from
	// SHA-512(seed), using its first 32 bytes.
	hash := sha512.Sum512(seed)
	var private, public [32]byte
	copy(private[:], hash[:32])
	copy(public[:], adminPublic)
	var nonce [24]byte
	copy(nonce[:], wrapped[:24])
	key, ok := box.Open(nil, wrapped[24:], &nonce, &public, &private)
	if !ok || len(key) != chacha20poly1305.KeySize {
		return nil, fmt.Errorf("could not open Simplepush organization key")
	}
	return key, nil
}
