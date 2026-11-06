package mail

import (
	"bufio"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/pem"
	"io"
	"math/big"
	"mime"
	"net"
	stdmail "net/mail"
	"strings"
	"sync"
	"testing"
	"time"
)

type smtpSink struct {
	listener  net.Listener
	tlsConfig *tls.Config
	messages  chan string
	errors    chan error
	mu        sync.Mutex
	count     int
	startTLS  bool
}

func testCertificate(t *testing.T, host string) (tls.Certificate, *x509.CertPool) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	template := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: host}, DNSNames: []string{host}, NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour), KeyUsage: x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment | x509.KeyUsageCertSign, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}, BasicConstraintsValid: true, IsCA: true}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})
	certificate, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		t.Fatal(err)
	}
	roots := x509.NewCertPool()
	if !roots.AppendCertsFromPEM(certPEM) {
		t.Fatal("test CA")
	}
	return certificate, roots
}
func newSMTPSink(t *testing.T, startTLS bool) (*smtpSink, *x509.CertPool) {
	t.Helper()
	certificate, roots := testCertificate(t, "smtp.inklore.test")
	cfg := &tls.Config{Certificates: []tls.Certificate{certificate}, MinVersion: tls.VersionTLS12}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	sink := &smtpSink{listener: listener, tlsConfig: cfg, messages: make(chan string, 64), errors: make(chan error, 16), startTLS: startTLS}
	t.Cleanup(func() { listener.Close() })
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			go sink.serve(conn)
		}
	}()
	return sink, roots
}
func (s *smtpSink) serve(conn net.Conn) {
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(5 * time.Second))
	secured := false
	if !s.startTLS {
		tlsConn := tls.Server(conn, s.tlsConfig)
		if tlsConn.Handshake() != nil {
			return
		}
		conn = tlsConn
		secured = true
	}
	reader := bufio.NewReader(conn)
	_, _ = io.WriteString(conn, "220 InkLore TLS test sink\r\n")
	authenticated := false
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			return
		}
		line = strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(line, "EHLO"), strings.HasPrefix(line, "HELO"):
			if secured {
				_, _ = io.WriteString(conn, "250-inklore.test\r\n250 AUTH PLAIN\r\n")
			} else {
				_, _ = io.WriteString(conn, "250-inklore.test\r\n250 STARTTLS\r\n")
			}
		case line == "STARTTLS" && !secured:
			_, _ = io.WriteString(conn, "220 Begin TLS\r\n")
			tlsConn := tls.Server(conn, s.tlsConfig)
			if tlsConn.Handshake() != nil {
				return
			}
			conn = tlsConn
			reader = bufio.NewReader(conn)
			secured = true
		case strings.HasPrefix(line, "AUTH PLAIN "):
			payload, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(line, "AUTH PLAIN "))
			if !secured || err != nil || string(payload) != "\x00sender\x00test-mail-secret" {
				_, _ = io.WriteString(conn, "535 Authentication rejected\r\n")
				continue
			}
			authenticated = true
			_, _ = io.WriteString(conn, "235 Authenticated\r\n")
		case strings.HasPrefix(line, "MAIL FROM:"), strings.HasPrefix(line, "RCPT TO:"):
			if !authenticated {
				_, _ = io.WriteString(conn, "530 Authentication required\r\n")
			} else {
				_, _ = io.WriteString(conn, "250 Accepted\r\n")
			}
		case line == "DATA" && authenticated:
			_, _ = io.WriteString(conn, "354 Send data\r\n")
			var data strings.Builder
			for {
				part, err := reader.ReadString('\n')
				if err != nil {
					return
				}
				if part == ".\r\n" {
					break
				}
				if strings.HasPrefix(part, "..") {
					part = part[1:]
				}
				data.WriteString(part)
			}
			s.mu.Lock()
			s.count++
			s.mu.Unlock()
			s.messages <- data.String()
			_, _ = io.WriteString(conn, "250 Queued\r\n")
		case line == "QUIT":
			_, _ = io.WriteString(conn, "221 Bye\r\n")
			return
		default:
			_, _ = io.WriteString(conn, "500 Unsupported command\r\n")
		}
	}
}
func (s *smtpSink) received() int { s.mu.Lock(); defer s.mu.Unlock(); return s.count }
func (s *smtpSink) message(t *testing.T) string {
	t.Helper()
	select {
	case message := <-s.messages:
		return message
	case <-time.After(3 * time.Second):
		t.Fatal("SMTP did not receive DATA")
		return ""
	}
}
func pinnedTransport(t *testing.T, o Options, listener net.Listener) *transport {
	t.Helper()
	tr := newTransport(o)
	tr.lookup = func(context.Context, string) ([]net.IPAddr, error) {
		return []net.IPAddr{{IP: net.ParseIP("8.8.8.8")}}, nil
	}
	tr.dial = func(ctx context.Context, network, address string) (net.Conn, error) {
		if !strings.HasPrefix(address, "8.8.8.8:") {
			t.Errorf("not DNS pinned: %s", address)
		}
		return (&net.Dialer{}).DialContext(ctx, network, listener.Addr().String())
	}
	return tr
}
func TestSMTPVerifiedTLSProtocols(t *testing.T) {
	for _, startTLS := range []bool{false, true} {
		name := "implicit"
		if startTLS {
			name = "starttls"
		}
		t.Run(name, func(t *testing.T) {
			sink, roots := newSMTPSink(t, startTLS)
			tr := pinnedTransport(t, Options{AllowedHosts: []string{"smtp.inklore.test"}, RootCAs: roots}, sink.listener)
			config := Settings{Provider: "smtp", SMTPHost: "smtp.inklore.test", SMTPPort: 465, SMTPSecurity: "implicit_tls", SMTPUsername: "sender", SenderEmail: "sender@inklore.test", SenderName: "InkLore"}
			if startTLS {
				config.SMTPPort = 587
				config.SMTPSecurity = "starttls"
			}
			message := Message{ID: "6138ae42-4980-414a-a22f-ea293851f8db", To: "recipient@inklore.test", Subject: "Code", Text: "verification code"}
			if err := tr.send(context.Background(), config, "test-mail-secret", message); err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(sink.message(t), "verification code") {
				t.Fatal("SMTP DATA omitted body")
			}
			tr.roots = nil
			if err := tr.send(context.Background(), config, "test-mail-secret", message); err == nil {
				t.Fatal("untrusted certificate accepted")
			}
		})
	}
}

func TestSMTPHeaderInjectionRejected(t *testing.T) {
	config := Settings{Provider: "smtp", SMTPHost: "smtp.inklore.test", SMTPPort: 465, SMTPSecurity: "implicit_tls", SMTPUsername: "sender", SenderEmail: "sender@inklore.test", SenderName: "InkLore"}
	message := Message{ID: "6138ae42-4980-414a-a22f-ea293851f8db", To: "recipient@inklore.test", Subject: "Code", Text: "verification code"}
	for _, test := range []struct {
		name   string
		change func(*Settings, *Message)
	}{
		{"sender-name", func(c *Settings, _ *Message) { c.SenderName = "InkLore\r\nBcc: other@inklore.test" }},
		{"sender-address", func(c *Settings, _ *Message) { c.SenderEmail += "\r\nBcc: other@inklore.test" }},
		{"recipient", func(_ *Settings, m *Message) { m.To += "\r\nBcc: other@inklore.test" }},
		{"subject", func(_ *Settings, m *Message) { m.Subject = "Code\r\nBcc: other@inklore.test" }},
		{"message-id", func(_ *Settings, m *Message) { m.ID += ">\r\nBcc: other@inklore.test" }},
		{"nul", func(_ *Settings, m *Message) { m.Subject = "Code\x00" }},
	} {
		for _, provider := range []string{"smtp"} {
			t.Run(provider+"/"+test.name, func(t *testing.T) {
				c, m := config, message
				c.Provider = provider
				test.change(&c, &m)
				tr := newTransport(Options{AllowedHosts: []string{"smtp.inklore.test"}})
				tr.lookup = func(context.Context, string) ([]net.IPAddr, error) {
					t.Fatal("invalid mail headers reached provider DNS")
					return nil, nil
				}
				if err := tr.send(context.Background(), c, "test-mail-secret", m); err == nil {
					t.Fatal("injected mail header was accepted")
				}
			})
		}
	}
}

func TestSMTPDataPreservesPlainTextBoundary(t *testing.T) {
	sink, roots := newSMTPSink(t, false)
	tr := pinnedTransport(t, Options{AllowedHosts: []string{"smtp.inklore.test"}, RootCAs: roots}, sink.listener)
	config := Settings{Provider: "smtp", SMTPHost: "smtp.inklore.test", SMTPPort: 465, SMTPSecurity: "implicit_tls", SMTPUsername: "sender", SenderEmail: "sender@inklore.test", SenderName: "InkLore 作者"}
	message := Message{ID: "6138ae42-4980-414a-a22f-ea293851f8db", To: "recipient@inklore.test", Subject: "验证消息", Text: "first line\n.\nBcc: body-only@inklore.test\n<p>plain text only</p>"}
	if err := tr.send(context.Background(), config, "test-mail-secret", message); err != nil {
		t.Fatal(err)
	}
	parsed, err := stdmail.ReadMessage(strings.NewReader(sink.message(t)))
	if err != nil {
		t.Fatal("SMTP DATA was not a valid MIME message")
	}
	if parsed.Header.Get("Bcc") != "" || parsed.Header.Get("Content-Type") != "text/plain; charset=UTF-8" || sink.received() != 1 {
		t.Fatal("body escaped into headers or another message")
	}
	subject, err := new(mime.WordDecoder).DecodeHeader(parsed.Header.Get("Subject"))
	if err != nil || subject != message.Subject {
		t.Fatal("encoded subject did not round-trip")
	}
	body, err := io.ReadAll(parsed.Body)
	if err != nil || string(body) != strings.ReplaceAll(message.Text, "\n", "\r\n")+"\r\n" {
		t.Fatal("SMTP dot-stuffing or plain-text body did not round-trip")
	}
}
func TestMailSSRFRejectsEveryPrivateAnswer(t *testing.T) {
	for _, address := range []string{"127.0.0.1", "169.254.169.254", "10.0.0.1", "::1", "::ffff:127.0.0.1", "64:ff9b::7f00:1", "2002:7f00:1::", "100.64.0.1", "192.0.2.1"} {
		t.Run(address, func(t *testing.T) {
			tr := newTransport(Options{AllowedHosts: []string{"smtp.inklore.test"}})
			tr.lookup = func(context.Context, string) ([]net.IPAddr, error) {
				return []net.IPAddr{{IP: net.ParseIP("8.8.8.8")}, {IP: net.ParseIP(address)}}, nil
			}
			if _, err := tr.address(context.Background(), "smtp.inklore.test", 465); err == nil {
				t.Fatal("private/rebinding destination accepted")
			}
		})
	}
	tr := newTransport(Options{})
	tr.lookup = func(context.Context, string) ([]net.IPAddr, error) {
		t.Fatal("unapproved host reached DNS")
		return nil, nil
	}
	if _, err := tr.address(context.Background(), "unknown.example", 465); err == nil {
		t.Fatal("missing allowlist accepted")
	}
}

func TestUnsupportedMailProviderFailsClosed(t *testing.T) {
	tr := newTransport(Options{AllowedHosts: []string{"smtp.inklore.test"}})
	err := tr.send(context.Background(), Settings{Provider: "resend", SenderEmail: "sender@inklore.test"}, "test-secret",
		Message{ID: "6138ae42-4980-414a-a22f-ea293851f8db", To: "recipient@inklore.test", Subject: "Code", Text: "code"})
	if err == nil {
		t.Fatal("unsupported provider was permitted")
	}
}
