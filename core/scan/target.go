package scan

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strings"
	"time"
)

var errTargetBlocked = errors.New("target address is outside the allowed public network range")

var blockedPrefixes = mustPrefixes(
	"0.0.0.0/8",
	"100.64.0.0/10",
	"192.0.0.0/24",
	"192.0.2.0/24",
	"198.18.0.0/15",
	"198.51.100.0/24",
	"203.0.113.0/24",
	"240.0.0.0/4",
	"255.255.255.255/32",
	"64:ff9b::/96",
	"64:ff9b:1::/48",
	"2001:db8::/32",
)

// TargetError is a validation or network-policy failure with a stable code
// suitable for API responses and audit logs.
type TargetError struct {
	Code    string
	Message string
}

func (e *TargetError) Error() string { return e.Message }

func newTargetError(code, message string) error {
	return &TargetError{Code: code, Message: message}
}

func mustPrefixes(values ...string) []netip.Prefix {
	prefixes := make([]netip.Prefix, 0, len(values))
	for _, value := range values {
		prefix, err := netip.ParsePrefix(value)
		if err != nil {
			panic(err)
		}
		prefixes = append(prefixes, prefix)
	}
	return prefixes
}

// NormalizeTarget validates a user-supplied URL and returns its canonical form.
func NormalizeTarget(raw string) (string, error) {
	u, err := NormalizeTargetURL(raw)
	if err != nil {
		return "", err
	}
	return u.String(), nil
}

// NormalizeTargetURL validates a URL and resolves the host to enforce the
// public-network policy. The actual connection repeats the check to protect
// against DNS rebinding.
func NormalizeTargetURL(raw string) (*url.URL, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, newTargetError("URL_INVALID", "URL is required")
	}
	if !strings.Contains(raw, "://") {
		raw = "https://" + raw
	}
	u, err := url.Parse(raw)
	if err != nil {
		return nil, newTargetError("URL_INVALID", "invalid URL")
	}
	if err := validateTargetURL(u); err != nil {
		return nil, err
	}
	u.Fragment = ""
	u.Host = canonicalHost(u)
	return u, nil
}

func canonicalHost(u *url.URL) string {
	host := strings.ToLower(strings.TrimSuffix(u.Hostname(), "."))
	port := u.Port()
	if (u.Scheme == "http" && port == "80") || (u.Scheme == "https" && port == "443") {
		port = ""
	}
	if port == "" {
		if strings.Contains(host, ":") {
			return "[" + host + "]"
		}
		return host
	}
	return net.JoinHostPort(host, port)
}

func validateTargetURL(u *url.URL) error {
	if u.Scheme != "http" && u.Scheme != "https" {
		return newTargetError("URL_INVALID", "only http and https URLs are allowed")
	}
	if u.Hostname() == "" || u.User != nil {
		return newTargetError("URL_INVALID", "URL must contain a host and cannot include credentials")
	}
	if port := u.Port(); port != "" && port != "80" && port != "443" {
		return newTargetError("URL_INVALID", "only ports 80 and 443 are allowed")
	}
	return ValidateHost(u.Hostname())
}

// ValidateHost rejects localhost, private, link-local and other non-public
// targets. It resolves the hostname and checks every returned address.
func ValidateHost(host string) error {
	host = strings.ToLower(strings.TrimSuffix(strings.TrimSpace(host), "."))
	if host == "" {
		return newTargetError("URL_INVALID", "URL must contain a host")
	}
	if host == "localhost" || strings.HasSuffix(host, ".localhost") || host == "metadata.google.internal" || host == "instance-data" {
		return newTargetError("TARGET_BLOCKED", "local and cloud metadata targets are blocked")
	}
	var ips []net.IP
	if parsed := net.ParseIP(strings.Trim(host, "[]")); parsed != nil {
		ips = []net.IP{parsed}
	} else {
		resolved, err := net.LookupIP(host)
		if err != nil {
			return newTargetError("DNS_RESOLUTION_FAILED", "host cannot be resolved")
		}
		ips = resolved
	}
	if len(ips) == 0 {
		return newTargetError("DNS_RESOLUTION_FAILED", "host cannot be resolved")
	}
	for _, ip := range ips {
		if IsBlockedIP(ip) {
			return newTargetError("TARGET_BLOCKED", "private, local, and non-public IP targets are blocked")
		}
	}
	return nil
}

// IsBlockedIP reports whether an address is outside the allowed public range.
func IsBlockedIP(ip net.IP) bool {
	addr, ok := netip.AddrFromSlice(ip)
	if !ok {
		return true
	}
	addr = addr.Unmap()
	if !addr.IsValid() || !addr.IsGlobalUnicast() || addr.IsPrivate() || addr.IsLoopback() ||
		addr.IsLinkLocalUnicast() || addr.IsLinkLocalMulticast() || addr.IsMulticast() || addr.IsUnspecified() {
		return true
	}
	for _, prefix := range blockedPrefixes {
		if prefix.Contains(addr) {
			return true
		}
	}
	return false
}

func newSafeClient(maxRedirects int, timeout time.Duration) *http.Client {
	transport := &http.Transport{
		Proxy:                 nil, // ignore HTTP_PROXY, which could bypass target checks
		DialContext:           safeDialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          4,
		MaxIdleConnsPerHost:   2,
		IdleConnTimeout:       30 * time.Second,
		TLSHandshakeTimeout:   5 * time.Second,
		ResponseHeaderTimeout: 8 * time.Second,
		ExpectContinueTimeout: time.Second,
	}
	return &http.Client{
		Timeout:   timeout,
		Transport: transport,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= maxRedirects {
				return fmt.Errorf("too many redirects")
			}
			if err := validateTargetURL(req.URL); err != nil {
				return err
			}
			return nil
		},
	}
}

func safeDialContext(ctx context.Context, network, address string) (net.Conn, error) {
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return nil, fmt.Errorf("invalid target address")
	}
	if port != "80" && port != "443" {
		return nil, errTargetBlocked
	}
	var ips []net.IP
	if parsed := net.ParseIP(host); parsed != nil {
		ips = []net.IP{parsed}
	} else {
		resolved, err := net.DefaultResolver.LookupIP(ctx, "ip", host)
		if err != nil {
			return nil, newTargetError("DNS_RESOLUTION_FAILED", "host cannot be resolved")
		}
		ips = resolved
	}
	if len(ips) == 0 {
		return nil, newTargetError("DNS_RESOLUTION_FAILED", "host cannot be resolved")
	}
	// Reject the whole target if any answer is non-public. Otherwise a DNS
	// round-robin record could be used to reach an internal address.
	for _, ip := range ips {
		if IsBlockedIP(ip) {
			return nil, errTargetBlocked
		}
	}
	dialer := &net.Dialer{Timeout: 5 * time.Second, KeepAlive: 30 * time.Second}
	var lastErr error
	for _, ip := range ips {
		conn, err := dialer.DialContext(ctx, network, net.JoinHostPort(ip.String(), port))
		if err == nil {
			return conn, nil
		}
		lastErr = err
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("could not connect")
	}
	return nil, lastErr
}
