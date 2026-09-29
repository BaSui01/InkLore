package infrastructure

import (
	"errors"
	"net"
	"net/url"
	"regexp"
	"strings"
)

var (
	envPattern = regexp.MustCompile("^[A-Z][A-Z0-9_]{0,127}$")

	forbiddenVars = map[string]bool{
		"PATH":              true,
		"PWD":               true,
		"HOME":              true,
		"SHELL":             true,
		"USER":              true,
		"LOGNAME":           true,
		"DATABASE_URL":      true,
		"POSTGRES_PASSWORD": true,
		"REDIS_URL":         true,
		"JWT_SECRET":        true,
		"LLM_SECRET_PEPPER": true,
		"OPENAI_API_KEY":    true,
		"DEEPSEEK_API_KEY":  true,
		"ANTHROPIC_API_KEY": true,
		"GEMINI_API_KEY":    true,
		"OLLAMA_HOST":       true,
	}
)

// DeploymentAllowlistOptions configures the deployment allowlist.
type DeploymentAllowlistOptions struct {
	AllowedHosts    []string
	AllowedEnvRefs  []string
	AllowedPrefixes []string
}

// DeploymentAllowlist implements d.MCPDeploymentAllowlistPort with fail-closed security.
type DeploymentAllowlist struct {
	allowedHosts    map[string]bool
	allowedEnvRefs  map[string]bool
	allowedPrefixes []string
}

// NewDeploymentAllowlist creates an allowlist port instance.
func NewDeploymentAllowlist(opts DeploymentAllowlistOptions) *DeploymentAllowlist {
	al := &DeploymentAllowlist{
		allowedHosts:    make(map[string]bool),
		allowedEnvRefs:  make(map[string]bool),
		allowedPrefixes: append([]string(nil), opts.AllowedPrefixes...),
	}
	for _, h := range opts.AllowedHosts {
		al.allowedHosts[strings.ToLower(strings.TrimSpace(h))] = true
	}
	for _, e := range opts.AllowedEnvRefs {
		al.allowedEnvRefs[strings.TrimSpace(e)] = true
	}
	if len(al.allowedPrefixes) == 0 {
		al.allowedPrefixes = []string{"INKLORE_MCP_"}
	}
	return al
}

// ValidateEndpoint ensures endpoint is HTTPS, valid host, non-private, and passes allowlist.
func (a *DeploymentAllowlist) ValidateEndpoint(endpoint string) error {
	if len(endpoint) > 2048 || !strings.HasPrefix(endpoint, "https://") {
		return errors.New("endpoint must start with https:// and be <= 2048 characters")
	}
	u, err := url.Parse(endpoint)
	if err != nil || u.Scheme != "https" || u.Host == "" || u.User != nil || u.Fragment != "" {
		return errors.New("invalid https endpoint format")
	}

	hostname := u.Hostname()
	if hostname == "" {
		return errors.New("missing endpoint hostname")
	}

	// Forbid localhost and metadata domains
	lowerHost := strings.ToLower(hostname)
	if lowerHost == "localhost" || strings.HasSuffix(lowerHost, ".local") || lowerHost == "metadata.google.internal" {
		return errors.New("localhost or internal metadata hostnames forbidden")
	}

	// If explicit allowed hosts list is configured, enforce membership
	if len(a.allowedHosts) > 0 && !a.allowedHosts[lowerHost] {
		return errors.New("endpoint host is not in deployment allowed hosts list")
	}

	// SSRF check on IP literal
	if ip := net.ParseIP(hostname); ip != nil {
		if isPrivateOrReservedIP(ip) {
			return errors.New("private, loopback, or metadata IP addresses forbidden")
		}
	}

	return nil
}

// ValidateEnvRef ensures the credential reference is well-formed, not in the blacklist,
// and matches deployment allowed prefixes/references.
func (a *DeploymentAllowlist) ValidateEnvRef(envRef string) error {
	if !envPattern.MatchString(envRef) {
		return errors.New("invalid env_ref identifier format")
	}
	if forbiddenVars[envRef] {
		return errors.New("env_ref references forbidden system environment variable")
	}
	if len(a.allowedEnvRefs) > 0 && a.allowedEnvRefs[envRef] {
		return nil
	}
	for _, p := range a.allowedPrefixes {
		if strings.HasPrefix(envRef, p) {
			return nil
		}
	}
	// Also allow general MCP environment references like MCP_*
	if strings.HasPrefix(envRef, "MCP_") {
		return nil
	}
	return errors.New("env_ref does not match allowed deployment prefix (e.g. INKLORE_MCP_)")
}

func isPrivateOrReservedIP(ip net.IP) bool {
	if ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsMulticast() || ip.IsUnspecified() {
		return true
	}
	if ip.IsPrivate() {
		return true
	}
	// AWS / GCP / Azure metadata service: 169.254.169.254
	if ip.Equal(net.ParseIP("169.254.169.254")) {
		return true
	}
	return false
}
