package llm

import (
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	connectTimeout        = 10 * time.Second
	responseHeaderTimeout = 20 * time.Second
)

func newStreamingHTTPClient() *http.Client {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.DialContext = (&net.Dialer{
		Timeout:   connectTimeout,
		KeepAlive: 30 * time.Second,
	}).DialContext
	transport.TLSHandshakeTimeout = connectTimeout
	transport.ResponseHeaderTimeout = responseHeaderTimeout
	transport.ExpectContinueTimeout = 1 * time.Second
	transport.IdleConnTimeout = 90 * time.Second

	return &http.Client{Transport: transport}
}

func validateBaseURL(baseURL, defaultURL, fieldName string, requireTLSForRemote bool) (string, *url.URL, error) {
	normalized := normalizeBaseURL(baseURL, defaultURL)
	parsed, err := url.Parse(normalized)
	if err != nil {
		return "", nil, fmt.Errorf("invalid %s %q: %w", fieldName, baseURL, err)
	}
	if parsed.Hostname() == "" {
		return "", nil, fmt.Errorf("invalid %s %q: missing host", fieldName, baseURL)
	}

	switch parsed.Scheme {
	case "http", "https":
	default:
		return "", nil, fmt.Errorf("invalid %s %q: unsupported scheme %q", fieldName, baseURL, parsed.Scheme)
	}

	if requireTLSForRemote && parsed.Scheme != "https" && !isLocalEndpointHost(parsed.Hostname()) {
		return "", nil, fmt.Errorf("%s must use https unless it points to localhost or a private network address", fieldName)
	}

	return strings.TrimRight(parsed.String(), "/"), parsed, nil
}

func isLocalEndpointHost(host string) bool {
	host = strings.TrimSpace(strings.Trim(strings.ToLower(host), "[]"))
	if host == "" {
		return false
	}
	if host == "localhost" || strings.HasSuffix(host, ".local") {
		return true
	}

	ip := net.ParseIP(host)
	if ip == nil {
		return false
	}

	return ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast()
}
