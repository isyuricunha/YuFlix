package media

import (
	"context"
	"errors"
	"io"
	"mime"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/isyuricunha/YuFlix/services/core/internal/catalog"
)

var ErrUnsafeStream = errors.New("stream is not a supported public HTTPS MP4")
var ErrNonSeekable = errors.New("upstream does not support byte ranges")
var ErrBadMedia = errors.New("upstream media is not confirmed MP4")
var validRange = regexp.MustCompile(`^bytes=([0-9]{1,18}-[0-9]{0,18}|-[0-9]{1,18})$`)

type Direct struct{ HTTP *http.Client }

func NewDirect() *Direct { return &Direct{HTTP: catalog.NewMediaClient()} }
func MimeMP4(contentType string) bool {
	mediaType, _, err := mime.ParseMediaType(contentType)
	return err == nil && (mediaType == "video/mp4" || mediaType == "application/mp4" || mediaType == "application/octet-stream")
}

// Probe confirms a bounded MP4 signature and successful HTTP 206 for Range.
// It is not a guarantee of complete playback or browser codec compatibility.
func (d *Direct) Probe(ctx context.Context, u string) error {
	if !catalog.ValidateMediaURL(u) {
		return ErrUnsafeStream
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return ErrUnsafeStream
	}
	req.Header.Set("Range", "bytes=0-1023")
	req.Header.Set("Accept-Encoding", "identity")
	resp, err := d.HTTP.Do(req)
	if err != nil {
		return ErrBadMedia
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusPartialContent {
		return ErrNonSeekable
	}
	if !strings.HasPrefix(resp.Header.Get("Content-Range"), "bytes 0-") {
		return ErrNonSeekable
	}
	if !MimeMP4(resp.Header.Get("Content-Type")) {
		return ErrBadMedia
	}
	var signature [12]byte
	if _, err = io.ReadFull(io.LimitReader(resp.Body, 32), signature[:]); err != nil {
		return ErrBadMedia
	}
	if string(signature[4:8]) != "ftyp" {
		return ErrBadMedia
	}
	return nil
}
func (d *Direct) Relay(w http.ResponseWriter, r *http.Request, u string) {
	if !catalog.ValidateMediaURL(u) {
		http.Error(w, "reprodução indisponível", http.StatusBadGateway)
		return
	}
	rng := r.Header.Get("Range")
	if rng != "" && !validRange.MatchString(rng) {
		http.Error(w, "intervalo inválido", http.StatusRequestedRangeNotSatisfiable)
		return
	}
	upstream, err := http.NewRequestWithContext(r.Context(), http.MethodGet, u, nil)
	if err != nil {
		http.Error(w, "reprodução indisponível", http.StatusBadGateway)
		return
	}
	upstream.Header.Set("Accept-Encoding", "identity")
	if rng != "" {
		upstream.Header.Set("Range", rng)
	}
	resp, err := d.HTTP.Do(upstream)
	if err != nil {
		http.Error(w, "fonte de vídeo indisponível", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()
	if (rng != "" && resp.StatusCode != 206) || (rng == "" && resp.StatusCode != 200 && resp.StatusCode != 206) {
		http.Error(w, "intervalo não suportado pela fonte", http.StatusBadGateway)
		return
	}
	if !MimeMP4(resp.Header.Get("Content-Type")) {
		http.Error(w, "formato incompatível", http.StatusUnsupportedMediaType)
		return
	}
	if rng != "" && !strings.HasPrefix(resp.Header.Get("Content-Range"), "bytes ") {
		http.Error(w, "Content-Range ausente", http.StatusBadGateway)
		return
	}
	// Never forward cookies, Location, Set-Cookie or external URLs.
	for _, header := range []string{"Content-Length", "Content-Range", "Last-Modified", "ETag"} {
		if v := resp.Header.Get(header); v != "" {
			w.Header().Set(header, v)
		}
	}
	w.Header().Set("Content-Type", "video/mp4")
	w.Header().Set("Accept-Ranges", "bytes")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(resp.StatusCode)
	_, _ = io.Copy(w, resp.Body) // bounded by client disconnect; never buffers a full video
}
func DirectCandidate(url string, notWebReady bool, proxyHeaders bool) bool {
	return !notWebReady && !proxyHeaders && strings.TrimSpace(url) != "" && catalog.ValidateMediaURL(url)
}
