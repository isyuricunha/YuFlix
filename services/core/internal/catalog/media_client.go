package catalog

import (
	"net/http"
	"time"
)

// NewMediaClient shares the public-address DNS pinning rules with the catalog
// client but never applies a whole-file timeout to a long-running video stream.
func NewMediaClient() *http.Client {
	t := NewClient().client.Transport.(*http.Transport).Clone()
	t.DisableCompression = true
	t.ResponseHeaderTimeout = 8 * time.Second
	return &http.Client{
		Transport: t,
		CheckRedirect: func(req *http.Request, previous []*http.Request) error {
			if len(previous) >= 3 || !ValidateMediaURL(req.URL.String()) {
				return http.ErrUseLastResponse
			}
			return nil
		},
	}
}
