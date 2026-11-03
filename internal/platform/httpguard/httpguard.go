package httpguard

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	defaultMaxRedirects     = 5
	defaultMaxResponseBytes = 16 << 20
)

type Policy struct {
	Resolver         Resolver
	Timeout          time.Duration
	MaxRedirects     int
	MaxResponseBytes int64
	// DisableProxy prevents user-controlled destinations from being resolved
	// and fetched by an environment proxy outside the guarded dialer.
	DisableProxy bool
}

type Resolver interface {
	LookupIP(ctx context.Context, host string) ([]net.IP, error)
}

type netResolver struct{}

func (netResolver) LookupIP(ctx context.Context, host string) ([]net.IP, error) {
	return net.DefaultResolver.LookupIP(ctx, "ip", host)
}

func ValidateURL(ctx context.Context, rawURL string, policy Policy) (*url.URL, error) {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return nil, fmt.Errorf("invalid url: %w", err)
	}
	if parsed.Scheme == "" || parsed.Host == "" {
		return nil, errors.New("url must include scheme and host")
	}
	if parsed.User != nil {
		return nil, errors.New("url userinfo is not allowed")
	}
	switch strings.ToLower(parsed.Scheme) {
	case "http", "https":
	default:
		return nil, fmt.Errorf("url scheme %q is not allowed", parsed.Scheme)
	}
	if err := ValidateHost(ctx, parsed.Hostname(), policy); err != nil {
		return nil, err
	}
	return parsed, nil
}

func ValidateHost(ctx context.Context, host string, policy Policy) error {
	host = strings.TrimSpace(strings.Trim(host, "[]"))
	if host == "" {
		return errors.New("host is required")
	}
	if ip := net.ParseIP(host); ip != nil {
		if IsBlockedIP(ip) {
			return fmt.Errorf("host resolves to blocked ip %s", ip.String())
		}
		return nil
	}

	resolver := policy.Resolver
	if resolver == nil {
		resolver = netResolver{}
	}
	ips, err := resolver.LookupIP(ctx, host)
	if err != nil {
		return fmt.Errorf("resolve host %q: %w", host, err)
	}
	if len(ips) == 0 {
		return fmt.Errorf("host %q resolved no addresses", host)
	}
	for _, ip := range ips {
		if IsBlockedIP(ip) {
			return fmt.Errorf("host %q resolves to blocked ip %s", host, ip.String())
		}
	}
	return nil
}

func IsBlockedIP(ip net.IP) bool {
	if ip == nil {
		return true
	}
	if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsMulticast() || ip.IsUnspecified() {
		return true
	}
	blocks := blockedIPBlocks()
	for _, block := range blocks {
		if block.Contains(ip) {
			return true
		}
	}
	return false
}

func NewClient(policy Policy) *http.Client {
	timeout := policy.Timeout
	if timeout <= 0 {
		timeout = 15 * time.Second
	}
	maxRedirects := policy.MaxRedirects
	if maxRedirects <= 0 {
		maxRedirects = defaultMaxRedirects
	}
	maxResponseBytes := policy.MaxResponseBytes
	if maxResponseBytes <= 0 {
		maxResponseBytes = defaultMaxResponseBytes
	}
	proxy := http.ProxyFromEnvironment
	if policy.DisableProxy {
		proxy = nil
	}
	transport := &http.Transport{
		Proxy:                 proxy,
		DialContext:           guardedDialContext(policy, 5*time.Second, 30*time.Second),
		TLSHandshakeTimeout:   5 * time.Second,
		ResponseHeaderTimeout: timeout,
		DisableKeepAlives:     true,
	}
	return &http.Client{
		Timeout:   timeout,
		Transport: limitResponseBodyTransport{base: transport, maxBytes: maxResponseBytes},
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= maxRedirects {
				return errors.New("stopped after too many redirects")
			}
			if req == nil || req.URL == nil {
				return errors.New("redirect target is empty")
			}
			_, err := ValidateURL(req.Context(), req.URL.String(), policy)
			return err
		},
	}
}

func guardedDialContext(policy Policy, timeout, keepAlive time.Duration) func(context.Context, string, string) (net.Conn, error) {
	dialer := &net.Dialer{Timeout: timeout, KeepAlive: keepAlive}
	return func(ctx context.Context, network, address string) (net.Conn, error) {
		host, port, err := net.SplitHostPort(address)
		if err != nil {
			return nil, err
		}
		ips, err := resolveHostForDial(ctx, host, policy)
		if err != nil {
			return nil, err
		}
		var lastErr error
		for _, ip := range ips {
			conn, err := dialer.DialContext(ctx, network, net.JoinHostPort(ip.String(), port))
			if err == nil {
				return conn, nil
			}
			lastErr = err
		}
		if lastErr != nil {
			return nil, lastErr
		}
		return nil, fmt.Errorf("host %q resolved no dialable addresses", host)
	}
}

func resolveHostForDial(ctx context.Context, host string, policy Policy) ([]net.IP, error) {
	host = strings.Trim(host, "[]")
	if ip := net.ParseIP(host); ip != nil {
		if IsBlockedIP(ip) {
			return nil, fmt.Errorf("host resolves to blocked ip %s", ip.String())
		}
		return []net.IP{ip}, nil
	}
	resolver := policy.Resolver
	if resolver == nil {
		resolver = netResolver{}
	}
	ips, err := resolver.LookupIP(ctx, host)
	if err != nil {
		return nil, fmt.Errorf("resolve host %q: %w", host, err)
	}
	if len(ips) == 0 {
		return nil, fmt.Errorf("host %q resolved no addresses", host)
	}
	for _, ip := range ips {
		if IsBlockedIP(ip) {
			return nil, fmt.Errorf("host %q resolves to blocked ip %s", host, ip.String())
		}
	}
	return ips, nil
}

func blockedIPBlocks() []*net.IPNet {
	cidrs := []string{
		"0.0.0.0/8",
		"10.0.0.0/8",
		"100.64.0.0/10",
		"127.0.0.0/8",
		"169.254.0.0/16",
		"172.16.0.0/12",
		"192.0.0.0/24",
		"192.168.0.0/16",
		"198.18.0.0/15",
		"224.0.0.0/4",
		"240.0.0.0/4",
		"::/128",
		"::1/128",
		"fc00::/7",
		"fe80::/10",
		"ff00::/8",
	}
	blocks := make([]*net.IPNet, 0, len(cidrs))
	for _, cidr := range cidrs {
		_, block, err := net.ParseCIDR(cidr)
		if err == nil {
			blocks = append(blocks, block)
		}
	}
	return blocks
}

type limitResponseBodyTransport struct {
	base     http.RoundTripper
	maxBytes int64
}

func (t limitResponseBodyTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	resp, err := t.base.RoundTrip(req)
	if err != nil || resp == nil || resp.Body == nil {
		return resp, err
	}
	resp.Body = &limitedReadCloser{Reader: io.LimitReader(resp.Body, t.maxBytes+1), closer: resp.Body, maxBytes: t.maxBytes}
	return resp, nil
}

type limitedReadCloser struct {
	io.Reader
	closer   io.Closer
	maxBytes int64
	read     int64
}

func (r *limitedReadCloser) Read(p []byte) (int, error) {
	n, err := r.Reader.Read(p)
	r.read += int64(n)
	if r.read > r.maxBytes {
		return n, fmt.Errorf("response body exceeds %d bytes", r.maxBytes)
	}
	return n, err
}

func (r *limitedReadCloser) Close() error {
	return r.closer.Close()
}
