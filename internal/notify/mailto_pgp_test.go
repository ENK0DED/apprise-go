package notify

import (
	"encoding/json"
	"net"
	"os"
	"reflect"
	"strings"
	"testing"
)

func TestWKDPublicIPPolicyMatchesUpstream(t *testing.T) {
	data, err := os.ReadFile("testdata/wkd_ip_policy.json")
	if err != nil {
		t.Fatal(err)
	}
	var rows []struct {
		IP     string
		Public bool
	}
	if err = json.Unmarshal(data, &rows); err != nil {
		t.Fatal(err)
	}
	for _, row := range rows {
		if got := wkdPublicIP(net.ParseIP(row.IP)); got != row.Public {
			t.Errorf("%s public=%v Python=%v", row.IP, got, row.Public)
		}
	}
}
func TestWKDURLsMatchUpstream(t *testing.T) {
	data, err := os.ReadFile("testdata/wkd_urls.json")
	if err != nil {
		t.Fatal(err)
	}
	var rows []struct {
		Email string
		URLs  []string
	}
	if err = json.Unmarshal(data, &rows); err != nil {
		t.Fatal(err)
	}
	for _, row := range rows {
		got, err := wkdURLs(row.Email)
		if !reflect.DeepEqual(got, row.URLs) || (err != nil) != (row.URLs == nil) {
			t.Errorf("%s URLs=%v err=%v Python=%v", row.Email, got, err, row.URLs)
		}
	}
}
func TestMailPGPMissingAndWrongKeysFailClosed(t *testing.T) {
	for _, raw := range []string{"mailto://example.com?from=sender@example.com&to=recipient@example.com&pgp=encrypt&store=no", "mailto://example.com?from=sender@example.com&to=recipient@example.com&pgp=sign&store=no", "mailto://example.com?from=sender@example.com&to=recipient@example.com&pgp=encrypt&pgppub=../testutil/fixtures/pgp_sender_public.asc&store=no"} {
		parsed, err := ParseURL(raw)
		if err != nil {
			t.Fatal(err)
		}
		target, err := NewMailtoTarget(parsed)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = target.buildMessages("private body", "title", nil); err == nil {
			t.Fatalf("requested PGP accepted unprotected content: %s", raw)
		}
	}
}
func TestMailPGPGeneratedIdentityPersists(t *testing.T) {
	storeMu.Lock()
	oldRoot, oldLength, oldSalt := storeRoot, storeUIDLength, storeSalt
	storeMu.Unlock()
	ConfigureStorage(t.TempDir(), 8, nil)
	defer ConfigureStorage(oldRoot, oldLength, oldSalt)
	parsed, err := ParseURL("mailto://example.com?from=sender@example.com&to=recipient@example.com&pgp=sign")
	if err != nil {
		t.Fatal(err)
	}
	first, err := NewMailtoTarget(parsed)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = first.buildMessages("body", "title", nil); err != nil {
		t.Fatal(err)
	}
	second, err := NewMailtoTarget(parsed)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = second.buildMessages("second", "title", nil); err != nil {
		t.Fatal(err)
	}
	if first.pgp.sender.PrimaryKey.KeyId != second.pgp.sender.PrimaryKey.KeyId {
		t.Fatal("sender identity changed")
	}
	path := first.pgp.keyPath("sender@example.com", true)
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0600 {
		t.Fatalf("private key mode %v", info.Mode())
	}
	data, err := os.ReadFile(path)
	if err != nil || !strings.Contains(string(data), "PGP PRIVATE KEY BLOCK") {
		t.Fatal("missing private key")
	}
}
