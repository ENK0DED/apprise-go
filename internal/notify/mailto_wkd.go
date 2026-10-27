package notify

import (
	"bytes"
	"context"
	"crypto/sha1"
	"crypto/tls"
	"encoding/base32"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strings"

	"github.com/ProtonMail/go-crypto/openpgp"
)

func wkdURLs(email string) ([]string, error) {
	local, domain, found := strings.Cut(strings.TrimSpace(email), "@")
	if !found || local == "" || !isValidHostname(domain) || strings.ContainsAny(domain, "_/@:\\") || net.ParseIP(domain) != nil {
		return nil, fmt.Errorf("invalid WKD address")
	}
	domain = strings.ToLower(domain)
	hash := sha1.Sum([]byte(strings.ToLower(local)))
	encoded := base32.NewEncoding("ybndrfg8ejkmcpqxot1uwisza345h769").WithPadding(base32.NoPadding).EncodeToString(hash[:])
	return []string{"https://openpgpkey." + domain + "/.well-known/openpgpkey/" + domain + "/hu/" + encoded + "?l=" + strings.ReplaceAll(strings.ReplaceAll(url.QueryEscape(local), "%2F", "/"), "+", "%20"), "https://" + domain + "/.well-known/openpgpkey/hu/" + encoded + "?l=" + strings.ReplaceAll(strings.ReplaceAll(url.QueryEscape(local), "%2F", "/"), "+", "%20")}, nil
}
func wkdDial(ctx context.Context, network, address string) (net.Conn, error) {
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return nil, err
	}
	ips, err := net.DefaultResolver.LookupIPAddr(ctx, host)
	if err != nil {
		return nil, err
	}
	if len(ips) == 0 {
		return nil, fmt.Errorf("WKD destination has no address")
	}
	for _, entry := range ips {
		ip := entry.IP
		if !wkdPublicIP(ip) {
			return nil, fmt.Errorf("WKD destination is not public")
		}
	}
	currentHTTPOptionsMu.RLock()
	timeout := currentHTTPOptions.connectTimeout
	currentHTTPOptionsMu.RUnlock()
	dialer := net.Dialer{Timeout: timeout}
	return dialer.DialContext(ctx, network, net.JoinHostPort(ips[0].IP.String(), port))
}
func fetchWKDKeys(email string, parsed *ParsedURL) (openpgp.EntityList, error) {
	urls, err := wkdURLs(email)
	if err != nil {
		return nil, err
	}
	currentHTTPOptionsMu.RLock()
	options := currentHTTPOptions
	currentHTTPOptionsMu.RUnlock()
	config := &tls.Config{MinVersion: tls.VersionTLS12}
	if parsed != nil {
		config.InsecureSkipVerify = !parseBoolWithDefault(parsed.Query["verify"], true)
	} // #nosec G402 -- explicit verify=no matches the provider URL contract
	if roots, ok, err := loadCertPoolFromEnv(); err != nil {
		return nil, err
	} else if ok {
		config.RootCAs = roots
	}
	transport := &http.Transport{DialContext: wkdDial, TLSClientConfig: config, ResponseHeaderTimeout: options.readTimeout}
	defer transport.CloseIdleConnections()
	client := &http.Client{Transport: transport, Timeout: options.connectTimeout + options.readTimeout, CheckRedirect: func(r *http.Request, via []*http.Request) error {
		if !options.followRedirect {
			return http.ErrUseLastResponse
		}
		if len(via) >= 10 || r.URL.Scheme != "https" || r.URL.User != nil {
			return fmt.Errorf("invalid WKD redirect")
		}
		return nil
	}}
	client = clientWithDeadline(client, currentNotificationDeadline())
	for _, endpoint := range urls {
		response, err := client.Get(endpoint)
		if err != nil {
			continue
		}
		data, readErr := io.ReadAll(io.LimitReader(response.Body, 8001))
		_ = response.Body.Close()
		if readErr != nil || response.StatusCode != 200 || len(data) > 8000 {
			continue
		}
		keys, err := openpgp.ReadKeyRing(bytes.NewReader(data))
		if err == nil && len(keys) > 0 {
			return keys, nil
		}
	}
	return nil, fmt.Errorf("WKD returned no usable recipient key")
}

func wkdPrefixes(values ...string) []netip.Prefix {
	result := make([]netip.Prefix, len(values))
	for i, value := range values {
		result[i] = netip.MustParsePrefix(value)
	}
	return result
}
func wkdInNetworks(address netip.Addr, networks []netip.Prefix) bool {
	for _, network := range networks {
		if network.Contains(address) {
			return true
		}
	}
	return false
}

var wkdNAT64 = wkdPrefixes("64:ff9b::/96", "64:ff9b:1::/48")

func wkdPublicIP(ip net.IP) bool {
	address, ok := netip.AddrFromSlice(ip)
	if !ok {
		return false
	}
	address = address.Unmap()
	if address.Is6() && wkdInNetworks(address, wkdNAT64) {
		bytes := address.As16()
		return wkdPublicIP(net.IP(bytes[12:]))
	}
	if !address.IsGlobalUnicast() || address.IsLoopback() || address.IsLinkLocalUnicast() || address.IsUnspecified() {
		return false
	}
	if address.Is4() {
		return (!wkdInNetworks(address, wkdPrivateV4) || wkdInNetworks(address, wkdExceptionsV4)) && !netip.MustParsePrefix("100.64.0.0/10").Contains(address)
	}
	return (!wkdInNetworks(address, wkdPrivateV6) || wkdInNetworks(address, wkdExceptionsV6)) && !wkdInNetworks(address, wkdReservedV6) && !netip.MustParsePrefix("fec0::/10").Contains(address)
}
