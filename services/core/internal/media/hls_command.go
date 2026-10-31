package media

import (
	"errors"
	"fmt"
	"math"
	"path/filepath"
	"strconv"
	"strings"
)

var ErrInvalidHLSPlan = errors.New("invalid or unapproved HLS conversion")
var ErrHDRNeedsReview = errors.New("HDR video cannot be transcoded without verified tone mapping")

// HLSSpec specifies one local-source HLS job. The caller must verify file
// ownership, ticket authorization, worker isolation and private tmpfs storage.
type HLSSpec struct {
	InputPath       string
	PlaylistPath    string
	SegmentTemplate string
	VideoIndex      int
	AudioIndex      int // -1 means no audio track
	SeekSeconds     float64
	Plan            Plan
	HDRTransfer     string
}

// HLSArguments builds an argument array, never a shell command.
// It does not start FFmpeg or permit arbitrary network source URLs.
func HLSArguments(spec HLSSpec) ([]string, error) {
	if spec.InputPath == "" || !filepath.IsAbs(spec.InputPath) ||
		strings.Contains(spec.InputPath, "://") ||
		spec.PlaylistPath == "" || !filepath.IsAbs(spec.PlaylistPath) ||
		spec.SegmentTemplate == "" || !filepath.IsAbs(spec.SegmentTemplate) ||
		filepath.Ext(spec.PlaylistPath) != ".m3u8" ||
		!strings.HasSuffix(spec.SegmentTemplate, "%05d.ts") ||
		filepath.Dir(spec.PlaylistPath) != filepath.Dir(spec.SegmentTemplate) ||
		spec.VideoIndex < 0 || spec.VideoIndex > 255 ||
		spec.AudioIndex < -1 || spec.AudioIndex > 255 ||
		math.IsNaN(spec.SeekSeconds) || math.IsInf(spec.SeekSeconds, 0) ||
		spec.SeekSeconds < 0 || spec.SeekSeconds > 24*60*60 {
		return nil, ErrInvalidHLSPlan
	}
	if spec.Plan.Mode != Remux && spec.Plan.Mode != TranscodeAudio && spec.Plan.Mode != TranscodeVideo {
		return nil, ErrInvalidHLSPlan
	}
	if spec.Plan.Mode == Remux && (!spec.Plan.CopyVideo || !spec.Plan.CopyAudio) ||
		spec.Plan.Mode == TranscodeAudio && (!spec.Plan.CopyVideo || spec.Plan.CopyAudio) ||
		spec.Plan.Mode == TranscodeVideo && spec.Plan.CopyVideo {
		return nil, ErrInvalidHLSPlan
	}
	transfer := strings.ToLower(spec.HDRTransfer)
	if spec.Plan.Mode == TranscodeVideo && (transfer == "smpte2084" || transfer == "arib-std-b67") {
		return nil, ErrHDRNeedsReview
	}
	args := []string{
		"-hide_banner", "-loglevel", "error", "-nostdin",
		"-protocol_whitelist", "file,pipe",
		"-threads", "1",
	}
	if spec.SeekSeconds > 0 {
		args = append(args, "-ss", strconv.FormatFloat(spec.SeekSeconds, 'f', 3, 64))
	}
	args = append(args, "-i", spec.InputPath, "-map", fmt.Sprintf("0:%d", spec.VideoIndex))
	if spec.AudioIndex >= 0 {
		args = append(args, "-map", fmt.Sprintf("0:%d", spec.AudioIndex))
	} else {
		args = append(args, "-an")
	}
	args = append(args, "-sn", "-dn", "-threads", "1")
	if spec.Plan.CopyVideo {
		args = append(args, "-c:v", "copy")
	} else {
		args = append(args, "-c:v", "libx264", "-preset", "ultrafast", "-crf", "25",
			"-pix_fmt", "yuv420p", "-force_key_frames", "expr:gte(t,n_forced*3)")
	}
	if spec.AudioIndex >= 0 {
		if spec.Plan.CopyAudio {
			args = append(args, "-c:a", "copy")
		} else {
			args = append(args, "-c:a", "aac", "-b:a", "128k")
		}
	}
	args = append(args, "-f", "hls", "-hls_time", "3", "-hls_list_size", "6",
		"-hls_delete_threshold", "2", "-hls_flags", "delete_segments+temp_file",
		"-hls_segment_filename", spec.SegmentTemplate, "-y", spec.PlaylistPath)
	return args, nil
}
