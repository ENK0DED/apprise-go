package notify

import (
	"testing"

	"github.com/unraid/apprise-go/internal/matrixolm"
)

// TestMatrixE2EERejectsUnsignedDeviceKeys is the security property: a
// homeserver that hands back a device whose keys are not signed by that device
// must not receive a room key. Without this check the server could name a
// device of its choosing and read everything.
func TestMatrixE2EERejectsUnsignedDeviceKeys(t *testing.T) {
	account, err := matrixolm.NewAccount()
	if err != nil {
		t.Fatalf("account: %v", err)
	}

	keys, err := account.DeviceKeys("@target:example.com", "TARGETDEVICE")
	if err != nil {
		t.Fatalf("device keys: %v", err)
	}
	if !matrixVerifyDeviceKeys(keys, "@target:example.com", "TARGETDEVICE") {
		t.Fatal("a device's own signature was rejected")
	}

	// Same keys, signature belonging to someone else.
	tampered := map[string]any{}
	for key, value := range keys {
		tampered[key] = value
	}
	tampered["signatures"] = map[string]any{
		"@target:example.com": map[string]any{
			"ed25519:TARGETDEVICE": "aW52YWxpZHNpZ25hdHVyZQ",
		},
	}
	if matrixVerifyDeviceKeys(tampered, "@target:example.com", "TARGETDEVICE") {
		t.Fatal("a forged device signature was accepted")
	}
}

func TestMatrixDeviceSignatureBindsClaimedIdentity(t *testing.T) {
	account, err := matrixolm.NewAccount()
	if err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{"user_id", "device_id"} {
		object := map[string]any{"user_id": "@target:example.com", "device_id": "TARGET", "keys": map[string]any{"ed25519:TARGET": account.SigningKey()}}
		object[field] = "other"
		canonical, err := matrixolm.CanonicalJSON(object)
		if err != nil {
			t.Fatal(err)
		}
		object["signatures"] = map[string]any{"@target:example.com": map[string]any{"ed25519:TARGET": account.Sign(canonical)}}
		if !matrixVerifySignedObject(object, "@target:example.com", "TARGET", account.SigningKey()) {
			t.Fatal("fixture signature is not valid")
		}
		if matrixVerifyDeviceKeys(object, "@target:example.com", "TARGET") {
			t.Fatalf("accepted signed key for another %s", field)
		}
	}
}
