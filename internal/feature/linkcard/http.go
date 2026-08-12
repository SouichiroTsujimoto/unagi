package linkcard

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	maxResponseBytes = 2 << 20 // 2 MiB (Amazon product pages are large)
	httpTimeout      = 10 * time.Second
	// Browser-like UA: Amazon returns a tiny interstitial for custom bot UAs.
	userAgent = "Mozilla/5.0 (compatible; unagi-linkcard/1.1; +https://github.com/SouichiroTsujimoto/unagi) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/122.0.0.0 Safari/537.36"
)

func newHTTPClient() *http.Client {
	dialer := &net.Dialer{Timeout: 5 * time.Second}
	transport := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: func(ctx context.Context, network, address string) (net.Conn, error) {
			host, port, err := net.SplitHostPort(address)
			if err != nil {
				return nil, err
			}
			ips, err := net.DefaultResolver.LookupIPAddr(ctx, host)
			if err != nil {
				return nil, err
			}
			var last error
			for _, ip := range ips {
				if isBlockedIP(ip.IP) {
					last = fmt.Errorf("blocked address")
					continue
				}
				conn, err := dialer.DialContext(ctx, network, net.JoinHostPort(ip.IP.String(), port))
				if err == nil {
					return conn, nil
				}
				last = err
			}
			if last == nil {
				last = fmt.Errorf("no usable address")
			}
			return nil, last
		},
		TLSHandshakeTimeout:   5 * time.Second,
		ResponseHeaderTimeout: 5 * time.Second,
		ForceAttemptHTTP2:     true,
	}
	return &http.Client{
		Timeout:   httpTimeout,
		Transport: transport,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 5 {
				return fmt.Errorf("too many redirects")
			}
			return validateFetchURL(req.URL)
		},
	}
}

func validateFetchURL(u *url.URL) error {
	if u == nil {
		return fmt.Errorf("empty url")
	}
	switch strings.ToLower(u.Scheme) {
	case "http", "https":
	default:
		return fmt.Errorf("unsupported scheme %q", u.Scheme)
	}
	host := strings.TrimSpace(u.Hostname())
	if host == "" {
		return fmt.Errorf("empty host")
	}
	if ip := net.ParseIP(host); ip != nil && isBlockedIP(ip) {
		return fmt.Errorf("blocked address")
	}
	return nil
}

func isBlockedIP(ip net.IP) bool {
	if ip == nil {
		return true
	}
	if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsMulticast() || ip.IsUnspecified() {
		return true
	}
	if ip4 := ip.To4(); ip4 != nil {
		// 169.254.0.0/16 already covered by link-local; CGNAT / benchmarking
		if ip4[0] == 100 && ip4[1] >= 64 && ip4[1] <= 127 {
			return true
		}
	}
	return false
}

func (c *Cards) get(ctx context.Context, rawURL string) (*http.Response, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, err
	}
	if err := validateFetchURL(u); err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept-Language", "ja-JP,ja;q=0.9,en-US;q=0.8,en;q=0.7")
	host := strings.ToLower(u.Hostname())
	switch {
	case host == "api.github.com":
		req.Header.Set("Accept", "application/vnd.github+json")
	case strings.Contains(host, "twitter.com") || host == "publish.twitter.com":
		req.Header.Set("Accept", "application/json")
	default:
		req.Header.Set("Accept", "text/html,application/xhtml+xml;q=0.9,application/json;q=0.8,*/*;q=0.7")
	}
	return c.http.Do(req)
}

func readLimited(r io.Reader) ([]byte, error) {
	return io.ReadAll(io.LimitReader(r, maxResponseBytes+1))
}

func (c *Cards) fetchBytes(ctx context.Context, rawURL string) ([]byte, string, error) {
	resp, err := c.get(ctx, rawURL)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, "", fmt.Errorf("status %d", resp.StatusCode)
	}
	body, err := readLimited(resp.Body)
	if err != nil {
		return nil, "", err
	}
	if len(body) > maxResponseBytes {
		return nil, "", fmt.Errorf("response too large")
	}
	finalURL := rawURL
	if resp.Request != nil && resp.Request.URL != nil {
		finalURL = resp.Request.URL.String()
	}
	return body, finalURL, nil
}
