// internal/adapters/moon/setup.go
//
// Setup-wizard integration for the Moon+ adapter.  Generates LAN URL +
// per-user credentials, returns step-by-step instructions for Moon+ Pro on
// Android, and provides a connection self-test (PROPFIND from Moon+'s
// expected path).

package moon

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"
)

// SetupBundle is everything the setup wizard needs to display to the user
// after first-time provisioning of the Moon+ adapter.
type SetupBundle struct {
	ServerURL    string // "http://192.168.1.10:8765/moon-webdav/"
	Username     string
	Password     string // plaintext - displayed once, then stored in secrets
	QRPayload    string // base64-encoded JSON for a QR-code in the wizard
	Instructions []string
	WritebackOK  bool   // false → setup warning ("Moon+ writeback disabled")
	Hint         string // shown when WritebackOK is false
}

// GenerateSetup picks a LAN IP, generates a random password, registers a
// new WebDAV user under `username`, and returns the bundle for the
// wizard to show to the user.  The password is also handed to the
// secrets.Store under the key "moon-webdav:<username>".
func (a *Adapter) GenerateSetup(username string) (*SetupBundle, error) {
	if username == "" {
		return nil, errors.New("moon: setup: username required")
	}
	if a.webdav == nil {
		return nil, errors.New("moon: setup: webdav server not initialised")
	}
	pw, err := generatePassword(24)
	if err != nil {
		return nil, fmt.Errorf("moon: setup: gen pw: %w", err)
	}
	if err := a.webdav.CreateUser(username, pw); err != nil {
		return nil, fmt.Errorf("moon: setup: create user: %w", err)
	}
	if a.secrets != nil {
		// Best-effort: secrets storage is optional in dev/test.
		_ = a.secrets.Set("moon-webdav:"+username, pw)
	}
	ip := pickLANIP()
	hostPort := joinHostPort(ip, a.webdav.BindAddr())
	url := fmt.Sprintf("http://%s%s", hostPort, a.webdav.URLPrefix())

	bundle := &SetupBundle{
		ServerURL:   url,
		Username:    username,
		Password:    pw,
		WritebackOK: IsWriterVerified(FormatV1Plain),
		Instructions: []string{
			"On Moon+ Reader Pro on your Android device:",
			"1. Open Moon+ Reader Pro.",
			"2. Tap the menu icon, then go to Settings.",
			"3. Open the Miscellaneous section.",
			"4. Tap \"Sync reading positions via Dropbox/WebDAV\".",
			"5. Choose WebDAV.",
			"6. Enter the server URL exactly as shown above.",
			"7. Enter the username and password shown above.",
			"8. Tap Test or Save — Moon+ will PROPFIND the server.",
			"9. Open any book and turn a page — ReadSync will record progress.",
		},
	}
	if !bundle.WritebackOK {
		bundle.Hint = "Moon+ writeback is disabled until a verified writer fixture is committed " +
			"under fixtures/moonplus/writers/. ReadSync will continue to read progress " +
			"FROM Moon+ (one-way) and will write back through Calibre / KOReader. " +
			"To enable two-way sync with Moon+, capture writer fixtures via the " +
			"in-app capture mode and re-run the wizard."
	}
	bundle.QRPayload = makeQRPayload(url, username, pw)
	return bundle, nil
}

// TestConnection performs the same PROPFIND that Moon+ Pro issues on first
// connect (depth=0 against the configured URL prefix) and returns nil if
// the server replies with a 207 Multi-Status.
func (a *Adapter) TestConnection(ctx context.Context, username, password string) error {
	if a.webdav == nil {
		return errors.New("moon: test: webdav not initialised")
	}
	hostPort := joinHostPort(pickLANIP(), a.webdav.BindAddr())
	target := fmt.Sprintf("http://%s%s", hostPort, a.webdav.URLPrefix())

	req, err := http.NewRequestWithContext(ctx, "PROPFIND", target,
		bytes.NewReader([]byte(`<?xml version="1.0"?><D:propfind xmlns:D="DAV:"><D:prop><D:resourcetype/></D:prop></D:propfind>`)))
	if err != nil {
		return err
	}
	req.Header.Set("Depth", "0")
	req.Header.Set("Content-Type", "application/xml")
	req.SetBasicAuth(username, password)

	cli := &http.Client{Timeout: 5 * time.Second}
	resp, err := cli.Do(req)
	if err != nil {
		return fmt.Errorf("moon: test: %w", err)
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body)
	if resp.StatusCode != http.StatusMultiStatus && resp.StatusCode != http.StatusOK {
		return fmt.Errorf("moon: test: unexpected status %s", resp.Status)
	}
	return nil
}

// pickLANIP returns the first non-loopback IPv4 address.  Falls back to
// "127.0.0.1" if none can be found (development-only).
func pickLANIP() string {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return "127.0.0.1"
	}
	for _, a := range addrs {
		ipnet, ok := a.(*net.IPNet)
		if !ok || ipnet.IP.IsLoopback() {
			continue
		}
		ip4 := ipnet.IP.To4()
		if ip4 == nil {
			continue
		}
		return ip4.String()
	}
	return "127.0.0.1"
}

// joinHostPort takes an IP and a "host:port" bind string and returns
// "ip:port" (replacing the host portion of bind with ip).
func joinHostPort(ip, bind string) string {
	_, port, err := net.SplitHostPort(bind)
	if err != nil {
		return ip
	}
	return net.JoinHostPort(ip, port)
}

// generatePassword returns a URL-safe random password of approximately n
// characters.  Uses crypto/rand for entropy.
func generatePassword(n int) (string, error) {
	if n <= 0 {
		n = 24
	}
	raw := make([]byte, n)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	s := base64.RawURLEncoding.EncodeToString(raw)
	if len(s) > n {
		s = s[:n]
	}
	return s, nil
}

// makeQRPayload returns a single line containing URL + credentials suitable
// for encoding into a QR code in the setup wizard.  Format:
//   "moon-webdav://<user>:<pass>@<host>:<port><prefix>"
// Mirrors the MQTT-style URI Moon+ users typically copy/paste between
// devices.
func makeQRPayload(url, user, pass string) string {
	// Strip "http://" prefix, prepend a scheme of our own.
	host := strings.TrimPrefix(url, "http://")
	host = strings.TrimPrefix(host, "https://")
	return fmt.Sprintf("moon-webdav://%s:%s@%s", user, pass, host)
}
