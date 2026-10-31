package catalog

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/url"
	"strings"
)

// StreamSource is a public HTTPS Stremio addon offering stream resources.
type StreamSource struct {
	Name string `json:"name"`
	ID   string `json:"id"`
}
type Candidate struct {
	Name  string
	Title string
	URL   string
}
type streamEntry struct {
	Name          string `json:"name"`
	Title         string `json:"title"`
	Description   string `json:"description"`
	URL           string `json:"url"`
	BehaviorHints struct {
		NotWebReady  bool            `json:"notWebReady"`
		ProxyHeaders json.RawMessage `json:"proxyHeaders"`
	} `json:"behaviorHints"`
}

func validateMediaURL(input string) bool {
	if input == "" || len(input) > 1800 || strings.TrimSpace(input) != input ||
		strings.ContainsAny(input, "\r\n\t\\") {
		return false
	}
	u, err := url.Parse(input)
	if err != nil || u.Scheme != "https" || u.Host == "" || u.Opaque != "" ||
		u.User != nil || u.Fragment != "" || u.RawFragment != "" || u.Port() != "" ||
		net.ParseIP(u.Hostname()) != nil || u.RawPath != "" || u.ForceQuery {
		return false
	}
	// No private, local or malformed domains even before DNS resolution.
	host := u.Hostname()
	if !strings.Contains(host, ".") || strings.HasSuffix(host, ".") || strings.ToLower(host) != host {
		return false
	}
	for _, part := range strings.Split(host, ".") {
		if part == "" || len(part) > 63 || part[0] == '-' || part[len(part)-1] == '-' {
			return false
		}
		for _, ch := range part {
			if !(ch >= 'a' && ch <= 'z' || ch >= '0' && ch <= '9' || ch == '-') {
				return false
			}
		}
	}
	// Explicitly recognized MP4 paths are candidates; opaque public HTTPS
	// paths also qualify for a *probe*, not for guaranteed playback. Many
	// Stremio addons use signed opaque stream identifiers without extensions.
	// HLS, MKV and other named formats remain excluded until supported.
	path := strings.ToLower(u.Path)
	if strings.HasSuffix(path, ".mp4") || strings.HasSuffix(path, ".m4v") {
		return true
	}
	leaf := path[strings.LastIndex(path, "/")+1:]
	return leaf != "" && len(leaf) <= 600 && !strings.Contains(leaf, ".")
}
func ValidateMediaURL(input string) bool { return validateMediaURL(input) }
func (c *Client) StreamManifest(ctx context.Context, manifestURL string) (StreamSource, error) {
	url, err := ValidateManifestURL(manifestURL)
	if err != nil {
		return StreamSource{}, err
	}
	var manifest struct {
		Name      string            `json:"name"`
		ID        string            `json:"id"`
		Resources []json.RawMessage `json:"resources"`
	}
	if err = c.fetch(ctx, url, 128<<10, &manifest); err != nil {
		return StreamSource{}, err
	}
	if manifest.Name == "" || len(manifest.Name) > 100 || manifest.ID == "" || len(manifest.ID) > 120 {
		return StreamSource{}, ErrInvalid
	}
	hasStream := false
	for _, raw := range manifest.Resources {
		var name string
		if json.Unmarshal(raw, &name) == nil && name == "stream" {
			hasStream = true
			break
		}
		var item struct {
			Name  string   `json:"name"`
			Types []string `json:"types"`
		}
		if json.Unmarshal(raw, &item) == nil && item.Name == "stream" {
			if len(item.Types) == 0 {
				hasStream = true
				break
			}
			for _, typ := range item.Types {
				if typ == "movie" || typ == "series" {
					hasStream = true
				}
			}
		}
	}
	if !hasStream {
		return StreamSource{}, errors.New("addon has no supported Stremio stream resource")
	}
	return StreamSource{Name: manifest.Name, ID: manifest.ID}, nil
}

// StreamReport distinguishes a reachable provider with unsupported streams
// from a provider whose HTTP request actually failed.
type StreamReport struct {
	Streams     []Candidate
	Received    int
	Unsupported int
}

func (c *Client) Streams(ctx context.Context, manifestURL, typ, videoID string) ([]Candidate, error) {
	report, err := c.StreamsDetailed(ctx, manifestURL, typ, videoID)
	return report.Streams, err
}
func (c *Client) StreamsDetailed(ctx context.Context, manifestURL, typ, videoID string) (StreamReport, error) {
	url, err := ValidateManifestURL(manifestURL)
	if err != nil || (typ != "movie" && typ != "series") || !ValidMediaID(videoID) {
		return StreamReport{}, ErrUnsafeURL
	}
	if c.egress != nil && c.egress.handles(url) {
		return c.egress.streams(ctx, url, typ, videoID)
	}
	base := strings.TrimSuffix(url, "/manifest.json")
	var payload struct {
		Streams []streamEntry `json:"streams"`
	}
	if err := c.fetch(ctx, base+"/stream/"+typ+"/"+videoID+".json", 512<<10, &payload); err != nil {
		return StreamReport{}, err
	}
	result := make([]Candidate, 0, 12)
	for _, st := range payload.Streams {
		if st.BehaviorHints.NotWebReady || len(st.BehaviorHints.ProxyHeaders) > 0 ||
			!validateMediaURL(st.URL) {
			continue
		}
		name := st.Name
		if name == "" {
			name = "Fonte MP4"
		}
		if len(name) > 100 {
			name = name[:100]
		}
		title := st.Title
		if title == "" {
			title = st.Description
		}
		if len(title) > 180 {
			title = title[:180]
		}
		result = append(result, Candidate{Name: name, Title: title, URL: st.URL})
		if len(result) >= 12 {
			break
		}
	}
	return StreamReport{Streams: result, Received: len(payload.Streams), Unsupported: len(payload.Streams) - len(result)}, nil
}
