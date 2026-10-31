package media

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const maxProbeOutput = 1 << 20

var (
	ErrUnsafeLocalMedia = errors.New("media path is outside approved root")
	ErrInvalidProbeJSON = errors.New("invalid or excessive ffprobe metadata")
)

type MediaTrack struct {
	Index         int    `json:"index"`
	Kind          string `json:"kind"`
	Codec         string `json:"codec"`
	Language      string `json:"language,omitempty"`
	Title         string `json:"title,omitempty"`
	Default       bool   `json:"default"`
	Forced        bool   `json:"forced"`
	Channels      int    `json:"channels,omitempty"`
	ChannelLayout string `json:"channelLayout,omitempty"`
	Width         int    `json:"width,omitempty"`
	Height        int    `json:"height,omitempty"`
	ColorTransfer string `json:"colorTransfer,omitempty"`
}

type MediaTracks struct {
	Container string       `json:"container"`
	Duration  float64      `json:"durationSeconds,omitempty"`
	Tracks    []MediaTrack `json:"tracks"`
}

// ReadFFprobeJSON parses a bounded, externally produced ffprobe JSON payload.
// Filename, paths, arbitrary format tags, URL metadata, and extradata are discarded.
func ReadFFprobeJSON(raw []byte) (MediaTracks, error) {
	if len(raw) == 0 || len(raw) > maxProbeOutput {
		return MediaTracks{}, ErrInvalidProbeJSON
	}
	var payload struct {
		Format struct {
			FormatName string `json:"format_name"`
			Duration   string `json:"duration"`
		} `json:"format"`
		Streams []struct {
			Index         int    `json:"index"`
			CodecType     string `json:"codec_type"`
			CodecName     string `json:"codec_name"`
			Channels      int    `json:"channels"`
			ChannelLayout string `json:"channel_layout"`
			Width         int    `json:"width"`
			Height        int    `json:"height"`
			ColorTransfer string `json:"color_transfer"`
			Tags          struct {
				Language string `json:"language"`
				Title    string `json:"title"`
			} `json:"tags"`
			Disposition struct {
				Default int `json:"default"`
				Forced  int `json:"forced"`
				Picture int `json:"attached_pic"`
			} `json:"disposition"`
		} `json:"streams"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil || len(payload.Streams) > 64 {
		return MediaTracks{}, ErrInvalidProbeJSON
	}
	out := MediaTracks{Container: normalizeContainer(payload.Format.FormatName), Tracks: []MediaTrack{}}
	if seconds, err := strconv.ParseFloat(payload.Format.Duration, 64); err == nil && !math.IsInf(seconds, 0) && !math.IsNaN(seconds) && seconds >= 0 && seconds < 1e9 {
		out.Duration = seconds
	}
	seen := map[int]bool{}
	for _, s := range payload.Streams {
		if s.Index < 0 || s.Index > 255 || seen[s.Index] {
			return MediaTracks{}, ErrInvalidProbeJSON
		}
		seen[s.Index] = true
		if s.CodecType != "video" && s.CodecType != "audio" && s.CodecType != "subtitle" {
			continue
		}
		if s.Disposition.Picture != 0 {
			continue
		}
		if len(s.CodecName) > 40 || s.Channels < 0 || s.Channels > 32 || s.Width < 0 || s.Height < 0 || s.Width > 16384 || s.Height > 16384 {
			return MediaTracks{}, ErrInvalidProbeJSON
		}
		track := MediaTrack{
			Index: s.Index, Kind: s.CodecType, Codec: strings.ToLower(s.CodecName),
			Language: normalizeTrackLanguage(s.Tags.Language),
			Title:    trimTrackLabel(s.Tags.Title),
			Default:  s.Disposition.Default == 1, Forced: s.Disposition.Forced == 1,
			Channels: s.Channels, ChannelLayout: trimTrackLabel(s.ChannelLayout),
			Width: s.Width, Height: s.Height, ColorTransfer: trimTrackLabel(s.ColorTransfer),
		}
		out.Tracks = append(out.Tracks, track)
	}
	if len(out.Tracks) == 0 {
		return MediaTracks{}, ErrInvalidProbeJSON
	}
	return out, nil
}

func normalizeContainer(input string) string {
	switch {
	case strings.Contains(input, "mp4"), strings.Contains(input, "mov"):
		return "mp4"
	case strings.Contains(input, "matroska"):
		return "mkv"
	case strings.Contains(input, "webm"):
		return "webm"
	case strings.Contains(input, "mpegts"):
		return "mpegts"
	default:
		return "unknown"
	}
}

func normalizeTrackLanguage(input string) string {
	value := strings.ToLower(strings.ReplaceAll(strings.TrimSpace(input), "_", "-"))
	switch value {
	case "pt-br", "por-br", "pob", "pb":
		return "pt-BR"
	case "pt", "por", "pt-pt":
		return "pt"
	case "eng", "en", "en-us":
		return "en"
	case "jpn", "ja", "jp":
		return "ja"
	}
	if len(value) > 12 {
		return ""
	}
	for _, ch := range value {
		if !(ch >= 'a' && ch <= 'z') && ch != '-' {
			return ""
		}
	}
	return value
}

func trimTrackLabel(input string) string {
	value := strings.TrimSpace(input)
	var result []rune
	for _, ch := range value {
		if ch < 32 || ch == 127 {
			continue
		}
		result = append(result, ch)
		if len(result) >= 96 {
			break
		}
	}
	return string(result)
}

// PreferredAudio picks Brazilian Portuguese first, then Portuguese,
// then the file's default/original track. It does not silently replace tracks.
func (m MediaTracks) PreferredAudio() (MediaTrack, bool) {
	var best MediaTrack
	scoreBest := -1
	for _, track := range m.Tracks {
		if track.Kind != "audio" {
			continue
		}
		score := 0
		if track.Default {
			score += 40
		}
		switch track.Language {
		case "pt-BR":
			score += 120
		case "pt":
			score += 90
		case "en":
			score += 20
		case "ja":
			score += 10
		}
		if score > scoreBest {
			best, scoreBest = track, score
		}
	}
	return best, scoreBest >= 0
}

// PreferredSubtitle only opts in when there is a Portuguese subtitle track.
// Other languages stay available for explicit selection; no fabricated tracks.
func (m MediaTracks) PreferredSubtitle() (MediaTrack, bool) {
	var best MediaTrack
	scoreBest := -1
	for _, track := range m.Tracks {
		if track.Kind != "subtitle" {
			continue
		}
		score := -1
		switch track.Language {
		case "pt-BR":
			score = 120
		case "pt":
			score = 90
		}
		if track.Forced && score >= 0 {
			score += 10
		}
		if score > scoreBest {
			best, scoreBest = track, score
		}
	}
	return best, scoreBest >= 0
}

type boundedProbeBuffer struct {
	bytes.Buffer
}

func (b *boundedProbeBuffer) Write(p []byte) (int, error) {
	if b.Len()+len(p) > maxProbeOutput {
		return 0, ErrInvalidProbeJSON
	}
	return b.Buffer.Write(p)
}

// FFprobeFile only accepts regular, local files underneath an approved root.
// Symlinks are resolved first; URLs and remote protocols are never accepted.
func FFprobeFile(ctx context.Context, binary, root, file string) (MediaTracks, error) {
	if binary == "" || root == "" || file == "" || strings.Contains(file, "://") {
		return MediaTracks{}, ErrUnsafeLocalMedia
	}
	resolvedRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return MediaTracks{}, ErrUnsafeLocalMedia
	}
	resolvedFile, err := filepath.EvalSymlinks(file)
	if err != nil {
		return MediaTracks{}, ErrUnsafeLocalMedia
	}
	relative, err := filepath.Rel(resolvedRoot, resolvedFile)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) || relative == "." {
		return MediaTracks{}, ErrUnsafeLocalMedia
	}
	info, err := os.Stat(resolvedFile)
	if err != nil || !info.Mode().IsRegular() {
		return MediaTracks{}, ErrUnsafeLocalMedia
	}
	timeoutCtx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	cmd := exec.CommandContext(timeoutCtx, binary,
		"-v", "error", "-protocol_whitelist", "file,pipe",
		"-show_format", "-show_streams", "-of", "json", "-i", resolvedFile)
	cmd.Stderr = io.Discard
	var output boundedProbeBuffer
	cmd.Stdout = &output
	if err = cmd.Run(); err != nil {
		return MediaTracks{}, fmt.Errorf("media probe failed: %w", err)
	}
	return ReadFFprobeJSON(output.Bytes())
}
