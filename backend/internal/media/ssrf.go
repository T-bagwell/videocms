package media

import (
	"errors"
	"net"
	"net/url"
	"strings"
)

// SafeHTTPURL validates that raw is an http(s) URL and, unless allowLocal is
// set, that it does not resolve to private, loopback, link-local or multicast
// addresses (SSRF guard for user-controlled fetch endpoints).
func SafeHTTPURL(raw string, allowLocal bool) (string, error) {
	if strings.TrimSpace(raw) == "" {
		return "", errors.New("empty url")
	}
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return "", errors.New("invalid http(s) url")
	}
	if allowLocal {
		return u.String(), nil
	}
	host := strings.Trim(u.Host, "[]")
	ips, err := net.LookupIP(host)
	if err != nil {
		return "", err
	}
	for _, ip := range ips {
		if isUnsafeIP(ip) {
			return "", errors.New("url resolves to a private or loopback address")
		}
	}
	return u.String(), nil
}

func isUnsafeIP(ip net.IP) bool {
	return ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() ||
		ip.IsLinkLocalMulticast() || ip.IsMulticast() || ip.IsUnspecified()
}
