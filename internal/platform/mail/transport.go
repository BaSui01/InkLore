package mail

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net"
	stdmail "net/mail"
	"net/netip"
	"net/http"
	"net/smtp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"InkLore/internal/platform/scope"
)

// The dial and lookup functions are private test seams. Production always resolves,
// rejects every nonpublic answer, pins one approved address and verifies host TLS.
type transport struct {
	allowed map[string]bool
	timeout time.Duration
	roots   *x509.CertPool
	lookup  func(context.Context, string) ([]net.IPAddr, error)
	dial    func(context.Context, string, string) (net.Conn, error)
}

var deniedNetworks = []string{"0.0.0.0/8", "10.0.0.0/8", "100.64.0.0/10", "127.0.0.0/8", "169.254.0.0/16", "172.16.0.0/12", "192.0.0.0/24", "192.0.2.0/24", "192.168.0.0/16", "198.18.0.0/15", "198.51.100.0/24", "203.0.113.0/24", "224.0.0.0/4", "240.0.0.0/4", "::/96", "::ffff:0:0:0/96", "::1/128", "fc00::/7", "fe80::/10", "fec0::/10", "ff00::/8", "2001:db8::/32", "64:ff9b::/96", "64:ff9b:1::/48", "2001::/23", "2002::/16"}

func publicIP(ip net.IP) bool {
	value, ok := netip.AddrFromSlice(ip)
	if !ok {
		return false
	}
	value = value.Unmap()
	if !value.IsGlobalUnicast() || value.IsPrivate() || value.IsLoopback() || value.IsLinkLocalUnicast() {
		return false
	}
	for _, raw := range deniedNetworks {
		if netip.MustParsePrefix(raw).Contains(value) {
			return false
		}
	}
	return true
}
func newTransport(o Options) *transport {
	allowed := map[string]bool{}
	for _, host := range o.AllowedHosts {
		allowed[strings.ToLower(host)] = true
	}
	timeout := o.Timeout
	if timeout <= 0 || timeout > 30*time.Second {
		timeout = 10 * time.Second
	}
	dialer := &net.Dialer{Timeout: timeout}
	return &transport{allowed: allowed, timeout: timeout, roots: o.RootCAs, lookup: net.DefaultResolver.LookupIPAddr, dial: dialer.DialContext}
}
func (t *transport) pinnedAddress(ctx context.Context, host string, port int) (string, error) {
	if host == "" || net.ParseIP(host) != nil || strings.ContainsAny(host, "/@:\\ \t\r\n") {
		return "", errors.New("mail destination is invalid")
	}
	answers, err := t.lookup(ctx, host)
	if err != nil || len(answers) == 0 {
		return "", errors.New("mail destination resolution failed")
	}
	for _, answer := range answers {
		if !publicIP(answer.IP) {
			return "", errors.New("mail destination is not public")
		}
	}
	return net.JoinHostPort(answers[0].IP.String(), strconv.Itoa(port)), nil
}

func (t *transport) address(ctx context.Context, host string, port int) (string, error) {
	if !t.allowed[host] {
		return "", errors.New("mail destination is not allowed")
	}
	return t.pinnedAddress(ctx, host, port)
}
func (t *transport) send(ctx context.Context, c Settings, secret string, m Message) error {
	// Validate every envelope/header value again at the final boundary. MIME
	// encoded words do not encode ASCII CR/LF and are not header sanitizers.
	if !validAddress(c.SenderEmail) || !validAddress(m.To) || !scope.ValidID(m.ID) ||
		strings.ContainsAny(c.SenderName+m.Subject, "\r\n\x00") || !utf8.ValidString(c.SenderName+m.Subject+m.Text) {
		return errors.New("mail message headers are invalid")
	}
	bounded, cancel := context.WithTimeout(ctx, t.timeout)
	defer cancel()
	switch c.Provider {
	case "smtp":
		return t.sendSMTP(bounded, c, secret, m)
	case "resend":
		return t.sendResend(bounded, c, secret, m)
	default:
		return errors.New("mail provider unavailable")
	}
}
func (t *transport) sendSMTP(ctx context.Context, c Settings, secret string, m Message) error {
	address, err := t.address(ctx, c.SMTPHost, c.SMTPPort)
	if err != nil {
		return err
	}
	conn, err := t.dial(ctx, "tcp", address)
	if err != nil {
		return errors.New("SMTP connection failed")
	}
	defer conn.Close()
	rawConn := conn
	stop := context.AfterFunc(ctx, func() { _ = rawConn.Close() })
	defer stop()
	if deadline, ok := ctx.Deadline(); ok {
		if err = conn.SetDeadline(deadline); err != nil {
			return err
		}
	}
	tlsConfig := &tls.Config{MinVersion: tls.VersionTLS12, ServerName: c.SMTPHost, RootCAs: t.roots}
	if c.SMTPSecurity == "implicit_tls" {
		secured := tls.Client(conn, tlsConfig)
		if err = secured.HandshakeContext(ctx); err != nil {
			return errors.New("SMTP TLS verification failed")
		}
		conn = secured
	}
	client, err := smtp.NewClient(conn, c.SMTPHost)
	if err != nil {
		return errors.New("SMTP greeting failed")
	}
	defer client.Close()
	if c.SMTPSecurity == "starttls" {
		if ok, _ := client.Extension("STARTTLS"); !ok {
			return errors.New("SMTP requires STARTTLS")
		}
		if err = client.StartTLS(tlsConfig); err != nil {
			return errors.New("SMTP TLS verification failed")
		}
	}
	if state, ok := client.TLSConnectionState(); !ok || !state.HandshakeComplete {
		return errors.New("SMTP encryption required")
	}
	if err = client.Auth(smtp.PlainAuth("", c.SMTPUsername, secret, c.SMTPHost)); err != nil {
		return errors.New("SMTP authentication failed")
	}
	if err = client.Mail(c.SenderEmail); err != nil {
		return errors.New("SMTP sender rejected")
	}
	if err = client.Rcpt(m.To); err != nil {
		return errors.New("SMTP recipient rejected")
	}
	writer, err := client.Data()
	if err != nil {
		return errors.New("SMTP message refused")
	}
	from := (&stdmail.Address{Name: c.SenderName, Address: c.SenderEmail}).String()
	message := fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s\r\nDate: %s\r\nMessage-ID: <%s@%s>\r\nMIME-Version: 1.0\r\nContent-Type: text/plain; charset=UTF-8\r\nContent-Transfer-Encoding: 8bit\r\n\r\n%s\r\n", from, (&stdmail.Address{Address: m.To}).String(), mime.QEncoding.Encode("utf-8", m.Subject), time.Now().UTC().Format(time.RFC1123Z), m.ID, c.SenderEmail[strings.LastIndexByte(c.SenderEmail, '@')+1:], strings.ReplaceAll(strings.ReplaceAll(m.Text, "\r\n", "\n"), "\n", "\r\n"))
	if _, err = io.WriteString(writer, message); err != nil {
		return errors.New("SMTP message outcome unknown")
	}
	if err = writer.Close(); err != nil {
		return errors.New("SMTP message outcome unknown")
	}
	// DATA acknowledgement is the delivery handoff; a failed QUIT cannot undo it.

	_ = client.Quit()
	return nil
}

// Resend is a fixed HTTPS integration. The endpoint, TLS server name and
// destination port are constants; environment proxies and redirects are disabled.
// A single approved public IP is pinned for the request to prevent DNS rebinding.
func (t *transport) sendResend(ctx context.Context, c Settings, secret string, m Message) error {
	const host = "api.resend.com"
	if secret == "" || len(secret) > 8192 || strings.ContainsAny(secret, "\r\n\x00") {
		return errors.New("Resend credential is invalid")
	}
	pinned, err := t.pinnedAddress(ctx, host, 443)
	if err != nil {
		return err
	}
	from := (&stdmail.Address{Name: c.SenderName, Address: c.SenderEmail}).String()
	payload, err := json.Marshal(struct {
		From    string   `json:"from"`
		To      []string `json:"to"`
		Subject string   `json:"subject"`
		Text    string   `json:"text"`
	}{From: from, To: []string{m.To}, Subject: m.Subject, Text: m.Text})
	if err != nil {
		return errors.New("Resend request could not be encoded")
	}
	dialPinned := func(dialCtx context.Context, network, address string) (net.Conn, error) {
		requestedHost, port, splitErr := net.SplitHostPort(address)
		if splitErr != nil || !strings.EqualFold(requestedHost, host) || port != "443" {
			return nil, errors.New("Resend destination is fixed")
		}
		return t.dial(dialCtx, network, pinned)
	}
	transport := &http.Transport{
		Proxy:                 nil,
		DialContext:           dialPinned,
		TLSClientConfig:       &tls.Config{MinVersion: tls.VersionTLS12, ServerName: host, RootCAs: t.roots},
		ForceAttemptHTTP2:     true,
		DisableKeepAlives:     true,
		ResponseHeaderTimeout: t.timeout,
	}
	client := &http.Client{
		Transport: transport,
		Timeout:   t.timeout,
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	defer transport.CloseIdleConnections()
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.resend.com/emails", bytes.NewReader(payload))
	if err != nil {
		return errors.New("Resend request could not be created")
	}
	request.Header.Set("Authorization", "Bearer "+secret)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Idempotency-Key", m.ID)
	response, err := client.Do(request)
	if err != nil {
		return errors.New("Resend request failed")
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("Resend rejected email with HTTP %d", response.StatusCode)
	}
	var accepted struct {
		ID string `json:"id"`
	}
	decoder := json.NewDecoder(io.LimitReader(response.Body, 16*1024))
	if err = decoder.Decode(&accepted); err != nil || !scope.ValidID(accepted.ID) {
		return errors.New("Resend acceptance response is invalid")
	}
	var trailing any
	if err = decoder.Decode(&trailing); err != io.EOF {
		return errors.New("Resend acceptance response contains trailing data")
	}
	return nil
}
