package catalog

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"os"
	"strings"
	"time"
)

const egressPort = "62582"

// EgressTransport is an explicitly configured, authenticated Tailscale-only
// public-addon lookup route. It never proxies arbitrary URLs or media bytes.
type EgressTransport struct {
	endpoint string
	token    string
	hosts    map[string]struct{}
	client   *http.Client
}

func NewEgressFromEnvironment() *EgressTransport {
	endpoint := os.Getenv("YUFLIX_STREAM_EGRESS_URL")
	token := os.Getenv("YUFLIX_STREAM_EGRESS_TOKEN")
	hosts := os.Getenv("YUFLIX_STREAM_EGRESS_HOSTS")
	if endpoint == "" && token == "" && hosts == "" {
		return nil
	}
	e, err := NewEgressTransport(endpoint, token, hosts)
	if err != nil {
		panic("invalid YuFlix stream egress configuration")
	}
	return e
}
func NewEgressTransport(endpoint, token, hostnames string) (*EgressTransport, error) {
	u, err := url.Parse(endpoint)
	if err != nil || u.User != nil || u.RawQuery != "" || u.Fragment != "" ||
		u.Path != "" || u.Opaque != "" || u.Host == "" || len(token) < 32 || len(token) > 128 {
		return nil, ErrUnsafeURL
	}
	if u.Scheme != "http" || u.Port() != egressPort {
		return nil, ErrUnsafeURL
	}
	ip, err := netip.ParseAddr(u.Hostname())
	if err != nil || !cgnat.Contains(ip.Unmap()) {
		return nil, ErrUnsafeURL
	}
	hosts := map[string]struct{}{}
	for _, h := range strings.Split(hostnames, ",") {
		h = strings.TrimSpace(h)
		if h == "" {
			continue
		}
		if len(h) > 253 || h != strings.ToLower(h) || strings.ContainsAny(h, ":/@?") ||
			net.ParseIP(h) != nil || !strings.Contains(h, ".") {
			return nil, ErrUnsafeURL
		}
		hosts[h] = struct{}{}
	}
	if len(hosts) == 0 || len(hosts) > 12 {
		return nil, ErrUnsafeURL
	}
	return &EgressTransport{endpoint: u.String(), token: token, hosts: hosts,
		client: &http.Client{Timeout: 16 * time.Second, Transport: &http.Transport{Proxy: nil,
			MaxIdleConns: 4, MaxIdleConnsPerHost: 4, IdleConnTimeout: 20 * time.Second},
			CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
		}}, nil
}
func (e *EgressTransport) handles(manifest string) bool {
	if e == nil {
		return false
	}
	u, err := url.Parse(manifest)
	if err != nil {
		return false
	}
	_, ok := e.hosts[u.Hostname()]
	return ok
}

type egressRequest struct {
	ManifestURL string `json:"manifestUrl"`
	Type        string `json:"type"`
	ID          string `json:"id"`
}
type egressResponse struct {
	Report StreamReport `json:"report"`
	Status int          `json:"status,omitempty"`
	Reason string       `json:"reason,omitempty"`
}

func (e *EgressTransport) streams(ctx context.Context, manifestURL, typ, id string) (StreamReport, error) {
	if e == nil || !e.handles(manifestURL) {
		return StreamReport{}, ErrUnsafeURL
	}
	reqBytes, err := json.Marshal(egressRequest{ManifestURL: manifestURL, Type: typ, ID: id})
	if err != nil {
		return StreamReport{}, ErrInvalid
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, e.endpoint+"/v1/streams", bytes.NewReader(reqBytes))
	if err != nil {
		return StreamReport{}, ErrUpstream
	}
	req.Header.Set("Authorization", "Bearer "+e.token)
	req.Header.Set("Content-Type", "application/json")
	res, err := e.client.Do(req)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return StreamReport{}, context.DeadlineExceeded
		}
		return StreamReport{}, ErrUpstream
	}
	defer res.Body.Close()
	bytes, err := io.ReadAll(io.LimitReader(res.Body, 800<<10))
	if err != nil || len(bytes) >= 800<<10 {
		return StreamReport{}, ErrInvalid
	}
	var payload egressResponse
	if err = json.Unmarshal(bytes, &payload); err != nil {
		return StreamReport{}, ErrInvalid
	}
	if res.StatusCode != 200 {
		if payload.Status >= 400 && payload.Status <= 599 {
			return StreamReport{}, &UpstreamHTTPError{StatusCode: payload.Status}
		}
		return StreamReport{}, ErrUpstream
	}
	// Never trust relay JSON blindly: validate every returned media URL too.
	if len(payload.Report.Streams) > 12 || payload.Report.Received < 0 ||
		payload.Report.Unsupported < 0 {
		return StreamReport{}, ErrInvalid
	}
	for _, s := range payload.Report.Streams {
		if !validateMediaURL(s.URL) {
			return StreamReport{}, ErrUnsafeURL
		}
	}
	return payload.Report, nil
}
