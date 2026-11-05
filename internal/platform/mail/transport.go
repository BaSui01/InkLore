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
	"net/http"
	stdmail "net/mail"
	"net/netip"
	"net/smtp"
	"strconv"
	"strings"
	"time"
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
func (t *transport) address(ctx context.Context, host string, port int) (string, error) {
	if !t.allowed[host] || net.ParseIP(host) != nil || strings.ContainsAny(host, "/@:\\ \t\r\n") {
		return "", errors.New("mail destination is not allowed")
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
func (t *transport) send(ctx context.Context, c Settings, secret string, m Message) error {
	bounded, cancel := context.WithTimeout(ctx, t.timeout)
	defer cancel()
	switch c.Provider {
	case "smtp":
		return t.sendSMTP(bounded, c, secret, m)
	case "resend":
		return t.sendHTTPS(bounded, c, secret, m)
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
	message := fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s\r\nDate: %s\r\nMessage-ID: <%s@%s>\r\nMIME-Version: 1.0\r\nContent-Type: text/plain; charset=UTF-8\r\nContent-Transfer-Encoding: 8bit\r\n\r\n%s\r\n", from, (&stdmail.Address{Address: m.To}).String(), mime.QEncoding.Encode("utf-8", m.Subject), time.Now().UTC().Format(time.RFC1123Z), m.ID, strings.Split(c.SenderEmail, "@")[1], strings.ReplaceAll(strings.ReplaceAll(m.Text, "\r\n", "\n"), "\n", "\r\n"))
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
func (t *transport) sendHTTPS(ctx context.Context, c Settings, secret string, m Message) error {
	const host = "api.resend.com"
	address, err := t.address(ctx, host, 443)
	if err != nil {
		return err
	}
	body, err := json.Marshal(map[string]any{"from": (&stdmail.Address{Name: c.SenderName, Address: c.SenderEmail}).String(), "to": []string{m.To}, "subject": m.Subject, "text": m.Text})
	if err != nil {
		return err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://"+host+"/emails", bytes.NewReader(body))
	if err != nil {
		return err
	}
	request.Header.Set("Authorization", "Bearer "+secret)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Idempotency-Key", m.ID)
	tr := &http.Transport{TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS12, ServerName: host, RootCAs: t.roots}, DialContext: func(ctx context.Context, network, dest string) (net.Conn, error) {
		if dest != host+":443" {
			return nil, errors.New("unexpected mail destination")
		}
		return t.dial(ctx, network, address)
	}}
	defer tr.CloseIdleConnections()
	client := &http.Client{Transport: tr, Timeout: t.timeout, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	response, err := client.Do(request)
	if err != nil {
		return errors.New("HTTPS email delivery failed")
	}
	defer response.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(response.Body, 4097))
	if err != nil || len(raw) > 4096 || response.StatusCode < 200 || response.StatusCode >= 300 {
		return errors.New("HTTPS email provider rejected delivery")
	}
	var receipt struct {
		ID string `json:"id"`
	}
	if json.Unmarshal(raw, &receipt) != nil || len(receipt.ID) == 0 || len(receipt.ID) > 200 {
		return errors.New("HTTPS email receipt is invalid")
	}
	return nil
}
