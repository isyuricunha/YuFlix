package catalog

import (
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// A public HTTPS MP4 endpoint may redirect to a public HTTP media CDN.
// This is a compatibility path for explicitly selected video only, never
// for manifest or addon discovery. Every hop uses the pinned public-DNS dialer.
func SafeMediaRedirect(u *url.URL) bool {
	if u == nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" ||
		u.User != nil || u.Opaque != "" || u.Fragment != "" || u.RawFragment != "" ||
		u.ForceQuery || len(u.String()) > 2200 {
		return false
	}
	host := u.Hostname()
	if net.ParseIP(host) != nil || !strings.Contains(host, ".") ||
		strings.HasSuffix(host, ".") || strings.ToLower(host) != host ||
		strings.Contains(host, "..") || strings.Contains(host, "_") ||
		strings.HasSuffix(host, ".local") || strings.HasSuffix(host, ".localhost") ||
		strings.HasSuffix(host, ".internal") {
		return false
	}
	for _, part := range strings.Split(host, ".") {
		if part == "" || len(part) > 63 || part[0] == '-' || part[len(part)-1] == '-' {
			return false
		}
		for _, c := range part {
			if !(c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '-') {
				return false
			}
		}
	}
	if u.Port() != "" {
		n, err := strconv.Atoi(u.Port())
		if err != nil || n < 1 || n > 65535 {
			return false
		}
	}
	if len(u.EscapedPath()) > 1400 || strings.Contains(u.Path, "..") ||
		strings.ContainsAny(u.Path, "\\") {
		return false
	}
	return true
}

// NewMediaClient shares the public-address DNS pinning rules with the catalog
// client but never applies a whole-file timeout to a long-running video stream.
func NewMediaClient() *http.Client {
	t := NewClient().client.Transport.(*http.Transport).Clone()
	t.DisableCompression = true
	t.ResponseHeaderTimeout = 8 * time.Second
	return &http.Client{
		Transport: t,
		CheckRedirect: func(req *http.Request, previous []*http.Request) error {
			if len(previous) >= 3 || !SafeMediaRedirect(req.URL) {
				return http.ErrUseLastResponse
			}
			return nil
		},
	}
}
