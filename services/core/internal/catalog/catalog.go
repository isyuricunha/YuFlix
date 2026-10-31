// Package catalog consumes public Stremio addon catalog metadata without exposing addon URLs.
package catalog

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"regexp"
	"strings"
	"time"
)

var (
	ErrUnsafeURL = errors.New("invalid or unsafe public addon address")
	ErrUpstream  = errors.New("catalog provider unavailable")
	ErrInvalid   = errors.New("invalid catalog metadata")
)

// UpstreamHTTPError deliberately contains no URL, response body or headers:
// configured manifest paths and stream locations may embed credentials.
type UpstreamHTTPError struct{ StatusCode int }

func (e *UpstreamHTTPError) Error() string        { return fmt.Sprintf("upstream HTTP %d", e.StatusCode) }
func (e *UpstreamHTTPError) Is(target error) bool { return target == ErrUpstream }

var identifier = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_.-]{0,99}$`)
var cgnat = netip.PrefixFrom(netip.AddrFrom4([4]byte{100, 64, 0, 0}), 10)
var benchmark = netip.MustParsePrefix("198.18.0.0/15")

type Catalog struct {
	Type  string `json:"type"`
	ID    string `json:"id"`
	Name  string `json:"name"`
	Extra []struct {
		Name     string `json:"name"`
		Required bool   `json:"isRequired"`
	} `json:"extra,omitempty"`
}
type Manifest struct {
	ID        string            `json:"id"`
	Name      string            `json:"name"`
	Catalogs  []Catalog         `json:"catalogs"`
	Resources []json.RawMessage `json:"resources"`
}
type Preview struct {
	ID          string `json:"id"`
	Type        string `json:"type"`
	Name        string `json:"name"`
	Poster      string `json:"poster,omitempty"`
	Background  string `json:"background,omitempty"`
	Description string `json:"description,omitempty"`
	ReleaseInfo string `json:"releaseInfo,omitempty"`
}
type Detail struct {
	Preview
	Genres []string `json:"genres,omitempty"`
	Videos []struct {
		ID      string `json:"id"`
		Title   string `json:"title"`
		Season  int    `json:"season"`
		Episode int    `json:"episode"`
	} `json:"videos,omitempty"`
}
type Client struct {
	client *http.Client
	egress *EgressTransport
}

func AllowedIP(ip netip.Addr) bool {
	if !ip.IsValid() {
		return false
	}
	ip = ip.Unmap()
	if !ip.IsGlobalUnicast() || ip.IsPrivate() || ip.IsLoopback() || ip.IsLinkLocalUnicast() ||
		cgnat.Contains(ip) || benchmark.Contains(ip) {
		return false
	}
	if ip.Is4() {
		// Reject all non-public IPv4 ranges, even addresses marked global-unicast.
		b := ip.As4()
		if b[0] == 0 || b[0] == 10 || b[0] == 127 || b[0] >= 224 ||
			(b[0] == 100 && b[1] >= 64 && b[1] <= 127) ||
			(b[0] == 169 && b[1] == 254) ||
			(b[0] == 172 && b[1] >= 16 && b[1] <= 31) ||
			(b[0] == 192 && (b[1] == 168 || b[1] == 0 || b[1] == 88)) ||
			(b[0] == 198 && (b[1] == 18 || b[1] == 19)) ||
			(b[0] == 203 && b[1] == 0 && b[2] == 113) ||
			(b[0] == 198 && b[1] == 51 && b[2] == 100) ||
			(b[0] == 192 && b[1] == 0 && b[2] == 2) ||
			(b[0] == 192 && b[1] == 0 && b[2] == 0) ||
			(b[0] == 192 && b[1] == 0 && b[2] == 170) {
			return false
		}
	} else if ip.Is6() {
		// Reject non-routable transition mechanisms and special-use IPv6 ranges.
		if !netip.MustParsePrefix("2000::/3").Contains(ip) ||
			netip.MustParsePrefix("2002::/16").Contains(ip) {
			return false
		}
		if netip.MustParsePrefix("2001:db8::/32").Contains(ip) || netip.MustParsePrefix("2001::/23").Contains(ip) {
			return false
		}
	}
	return true
}

// A Stremio installation URL can include encoded addon configuration segments
// ahead of /manifest.json. Preserve escaped bytes exactly when forming resource
// URLs while refusing ambiguous separators, traversal and control characters.
func validManifestPath(escaped string) bool {
	if len(escaped) > 3800 || !strings.HasPrefix(escaped, "/") {
		return false
	}
	parts := strings.Split(strings.TrimPrefix(escaped, "/"), "/")
	if len(parts) < 1 || len(parts) > 12 || parts[len(parts)-1] != "manifest.json" {
		return false
	}
	for _, raw := range parts[:len(parts)-1] {
		if raw == "" || len(raw) > 2500 {
			return false
		}
		value, err := url.PathUnescape(raw)
		if err != nil || value == "" || value == "." || value == ".." ||
			strings.ContainsAny(value, "/\\") {
			return false
		}
		for _, r := range value {
			if r < 0x20 || r == 0x7f {
				return false
			}
		}
	}
	return true
}
func ValidateManifestURL(input string) (string, error) {
	if input == "" || len(input) > 4096 || strings.TrimSpace(input) != input || strings.ContainsAny(input, "\r\n\t\\") {
		return "", ErrUnsafeURL
	}
	u, err := url.Parse(input)
	if err != nil || u.Scheme != "https" || u.Host == "" || u.User != nil || u.Fragment != "" || u.RawQuery != "" || u.ForceQuery ||
		!validManifestPath(u.EscapedPath()) || u.Opaque != "" || u.Port() != "" {
		return "", ErrUnsafeURL
	}
	host := u.Hostname()
	if len(host) > 253 || host == "" || strings.HasSuffix(host, ".") || strings.Contains(host, "..") || strings.Contains(host, "_") ||
		strings.ToLower(host) != host || net.ParseIP(host) != nil || !strings.Contains(host, ".") ||
		strings.ContainsAny(host, "!@#:$%&/") {
		return "", ErrUnsafeURL
	}
	for _, part := range strings.Split(host, ".") {
		if part == "" || len(part) > 63 || part[0] == '-' || part[len(part)-1] == '-' {
			return "", ErrUnsafeURL
		}
		for _, ch := range part {
			if !((ch >= 'a' && ch <= 'z') || (ch >= '0' && ch <= '9') || ch == '-') {
				return "", ErrUnsafeURL
			}
		}
	}
	if u.String() != input {
		return "", ErrUnsafeURL
	}
	return input, nil
}
func safeImage(input string) string {
	if input == "" || len(input) > 650 {
		return ""
	}
	u, err := url.Parse(input)
	if err != nil || u.Scheme != "https" || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || u.Opaque != "" ||
		u.Port() != "" || net.ParseIP(u.Hostname()) != nil || strings.Contains(u.Hostname(), ".") == false {
		return ""
	}
	return input
}

// Validate redirect metadata before the pinned, connection-time DNS check.
func safeRedirect(u *url.URL) bool {
	if u == nil || u.Scheme != "https" || u.Host == "" || u.User != nil ||
		u.RawQuery != "" || u.Fragment != "" || u.Port() != "" || u.Opaque != "" ||
		u.RawPath != "" || len(u.String()) > 600 || !strings.HasSuffix(u.Path, ".json") ||
		strings.Contains(u.Path, "..") || strings.Contains(u.Path, "//") {
		return false
	}
	host := u.Hostname()
	if net.ParseIP(host) != nil || host == "" || strings.HasSuffix(host, ".") ||
		!strings.Contains(host, ".") || strings.ToLower(host) != host {
		return false
	}
	for _, part := range strings.Split(host, ".") {
		if part == "" || len(part) > 63 || part[0] == '-' || part[len(part)-1] == '-' {
			return false
		}
		for _, ch := range part {
			if !((ch >= 'a' && ch <= 'z') || (ch >= '0' && ch <= '9') || ch == '-') {
				return false
			}
		}
	}
	return true
}
func NewClient() *Client {
	dialer := &net.Dialer{Timeout: 3 * time.Second, KeepAlive: 15 * time.Second}
	resolver := net.DefaultResolver
	transport := &http.Transport{
		Proxy: nil, MaxIdleConns: 8, MaxIdleConnsPerHost: 2, IdleConnTimeout: 20 * time.Second,
		TLSHandshakeTimeout: 4 * time.Second, ResponseHeaderTimeout: 6 * time.Second,
		TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS12},
		DialContext: func(ctx context.Context, network, address string) (net.Conn, error) {
			host, port, err := net.SplitHostPort(address)
			if err != nil {
				return nil, ErrUnsafeURL
			}
			if net.ParseIP(host) != nil {
				return nil, ErrUnsafeURL
			}
			ips, err := resolver.LookupNetIP(ctx, "ip", host)
			if err != nil || len(ips) == 0 {
				return nil, ErrUpstream
			}
			for _, ip := range ips {
				if !AllowedIP(ip) {
					return nil, ErrUnsafeURL
				}
			}
			// Pin the actual connection to one of the checked addresses.
			var last error
			for _, ip := range ips {
				conn, err := dialer.DialContext(ctx, "tcp", net.JoinHostPort(ip.String(), port))
				if err == nil {
					return conn, nil
				}
				last = err
			}
			return nil, last
		},
	}
	return &Client{egress: NewEgressFromEnvironment(), client: &http.Client{Transport: transport, Timeout: 10 * time.Second, CheckRedirect: func(req *http.Request, previous []*http.Request) error {
		if len(previous) >= 3 || !safeRedirect(req.URL) {
			return http.ErrUseLastResponse
		}
		return nil
	}}}
}
func (c *Client) fetch(ctx context.Context, target string, max int64, dst any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return ErrInvalid
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "YuFlix/1.0 (Stremio catalog client)")
	res, err := c.client.Do(req)
	if err != nil {
		// Keep the error category while never returning the URL or any
		// signed provider response details to API clients or logs.
		if errors.Is(err, context.DeadlineExceeded) {
			return context.DeadlineExceeded
		}
		var nerr net.Error
		if errors.As(err, &nerr) && nerr.Timeout() {
			return context.DeadlineExceeded
		}
		return ErrUpstream
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return &UpstreamHTTPError{StatusCode: res.StatusCode}
	}
	if !strings.Contains(strings.ToLower(res.Header.Get("Content-Type")), "json") {
		return ErrInvalid
	}
	body, err := io.ReadAll(io.LimitReader(res.Body, max+1))
	if err != nil || int64(len(body)) > max {
		return ErrInvalid
	}
	if err := json.Unmarshal(body, dst); err != nil {
		return ErrInvalid
	}
	return nil
}
func normalizeCatalogs(cs []Catalog) []Catalog {
	results := make([]Catalog, 0, len(cs))
	seen := map[string]bool{}
	for _, c := range cs {
		if !identifier.MatchString(c.Type) || !identifier.MatchString(c.ID) || len(c.Name) > 120 || c.Name == "" {
			continue
		}
		if c.Type != "movie" && c.Type != "series" {
			continue
		} // TV and anime integrations follow after catalog proof.
		required := false
		for _, e := range c.Extra {
			if e.Required {
				required = true
			}
		}
		if required {
			continue
		}
		key := c.Type + ":" + c.ID
		if seen[key] {
			continue
		}
		seen[key] = true
		results = append(results, Catalog{Type: c.Type, ID: c.ID, Name: c.Name})
		if len(results) == 18 {
			break
		}
	}
	return results
}
func (c *Client) Manifest(ctx context.Context, manifestURL string) (Manifest, error) {
	v, err := ValidateManifestURL(manifestURL)
	if err != nil {
		return Manifest{}, err
	}
	var m Manifest
	if err = c.fetch(ctx, v, 128<<10, &m); err != nil {
		return Manifest{}, err
	}
	if m.Name == "" || len(m.Name) > 100 || len(m.ID) > 120 {
		return Manifest{}, ErrInvalid
	}
	m.Catalogs = normalizeCatalogs(m.Catalogs)
	if len(m.Catalogs) == 0 {
		return Manifest{}, errors.New("addon has no supported public movie/series catalogs")
	}
	m.Resources = nil
	return m, nil
}
func (c *Client) Catalog(ctx context.Context, manifestURL string, typ, id string) ([]Preview, error) {
	u, err := ValidateManifestURL(manifestURL)
	if err != nil || !identifier.MatchString(typ) || !identifier.MatchString(id) {
		return nil, ErrUnsafeURL
	}
	base := strings.TrimSuffix(u, "/manifest.json")
	var res struct {
		Metas []Preview `json:"metas"`
	}
	if err := c.fetch(ctx, fmt.Sprintf("%s/catalog/%s/%s.json", base, typ, id), 1300<<10, &res); err != nil {
		return nil, err
	}
	out := make([]Preview, 0, 25)
	for _, p := range res.Metas {
		if p.Type != typ || p.ID == "" || len(p.ID) > 160 || len(p.Name) > 200 || p.Name == "" {
			continue
		}
		p.Poster = safeImage(p.Poster)
		p.Background = safeImage(p.Background)
		if len(p.Description) > 500 {
			p.Description = p.Description[:500]
		}
		if len(p.ReleaseInfo) > 80 {
			p.ReleaseInfo = ""
		}
		out = append(out, p)
		if len(out) == 25 {
			break
		}
	}
	return out, nil
}

var mediaID = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9:._-]{0,159}$`)

func (c *Client) Meta(ctx context.Context, manifestURL, typ, id string) (Detail, error) {
	u, err := ValidateManifestURL(manifestURL)
	if err != nil || !identifier.MatchString(typ) || !mediaID.MatchString(id) {
		return Detail{}, ErrUnsafeURL
	}
	base := strings.TrimSuffix(u, "/manifest.json")
	var res struct {
		Meta Detail `json:"meta"`
	}
	if err := c.fetch(ctx, fmt.Sprintf("%s/meta/%s/%s.json", base, typ, url.PathEscape(id)), 2300<<10, &res); err != nil {
		return Detail{}, err
	}
	m := res.Meta
	if m.ID != id || m.Type != typ || m.Name == "" || len(m.Name) > 200 {
		return Detail{}, ErrInvalid
	}
	m.Poster = safeImage(m.Poster)
	m.Background = safeImage(m.Background)
	if len(m.Description) > 1400 {
		m.Description = m.Description[:1400]
	}
	if len(m.Videos) > 65 {
		m.Videos = m.Videos[:65]
	}
	if len(m.Genres) > 12 {
		m.Genres = m.Genres[:12]
	}
	return m, nil
}
func ValidMediaID(id string) bool { return mediaID.MatchString(id) }
