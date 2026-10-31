package media

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

func testHLSSpec(root string, plan Plan) HLSSpec {
	return HLSSpec{
		InputPath: filepath.Join(root, "source.mkv"),
		PlaylistPath: filepath.Join(root, "index.m3u8"),
		SegmentTemplate: filepath.Join(root, "chunk-%05d.ts"),
		VideoIndex: 0, AudioIndex: 2, Plan: plan,
	}
}

func TestHLSArgumentsCopiesCompatibleVideoAndAudio(t *testing.T) {
	args, err := HLSArguments(testHLSSpec(t.TempDir(), Plan{
		Mode: Remux, CopyVideo: true, CopyAudio: true,
	}))
	if err != nil {
		t.Fatal(err)
	}
	line := strings.Join(args, " ")
	for _, flag := range []string{
		"-map 0:0", "-map 0:2", "-c:v copy", "-c:a copy",
		"-threads 1", "-hls_flags delete_segments+temp_file",
	} {
		if !strings.Contains(line, flag) {
			t.Errorf("missing FFmpeg argument %q", flag)
		}
	}
	if strings.Contains(line, "libx264") || strings.Contains(line, " -c:a aac ") {
		t.Fatal("remux must not reencode compatible tracks")
	}
}

func TestHLSArgumentsTranscodesOnlyIncompatibleTrack(t *testing.T) {
	audioArgs, err := HLSArguments(testHLSSpec(t.TempDir(), Plan{
		Mode: TranscodeAudio, CopyVideo: true, CopyAudio: false,
	}))
	if err != nil {
		t.Fatal(err)
	}
	s := strings.Join(audioArgs, " ")
	if !strings.Contains(s, "-c:v copy") || !strings.Contains(s, "-c:a aac") || strings.Contains(s, "libx264") {
		t.Fatal("audio-only conversion unexpectedly recodes video")
	}
	videoArgs, err := HLSArguments(testHLSSpec(t.TempDir(), Plan{
		Mode: TranscodeVideo, CopyVideo: false, CopyAudio: true,
	}))
	if err != nil {
		t.Fatal(err)
	}
	s = strings.Join(videoArgs, " ")
	if !strings.Contains(s, "-c:v libx264") || !strings.Contains(s, "-c:a copy") {
		t.Fatal("video-only conversion changed compatible audio codec")
	}
	if !slices.Contains(videoArgs, "ultrafast") {
		t.Fatal("CPU-constrained preset missing")
	}
}

func TestHLSArgumentsRejectsUnsafeInputsAndUnverifiedHDR(t *testing.T) {
	base := testHLSSpec(t.TempDir(), Plan{Mode: Remux, CopyVideo: true, CopyAudio: true})
	cases := []HLSSpec{
		func() HLSSpec { c := base; c.InputPath = "https://video.example.com/private"; return c }(),
		func() HLSSpec { c := base; c.VideoIndex = -1; return c }(),
		func() HLSSpec { c := base; c.SegmentTemplate = filepath.Join(t.TempDir(), "segment-%05d.ts"); return c }(),
		func() HLSSpec { c := base; c.Plan.Mode = DirectPlay; return c }(),
		func() HLSSpec { c := base; c.SeekSeconds = -1; return c }(),
	}
	for _, input := range cases {
		if _, err := HLSArguments(input); !errors.Is(err, ErrInvalidHLSPlan) {
			t.Fatalf("unsafe plan accepted: %+v (%v)", input, err)
		}
	}
	hdr := base
	hdr.Plan = Plan{Mode: TranscodeVideo, CopyVideo: false, CopyAudio: true}
	hdr.HDRTransfer = "smpte2084"
	if _, err := HLSArguments(hdr); !errors.Is(err, ErrHDRNeedsReview) {
		t.Fatalf("HDR transcoding must require explicit tone-mapping validation: %v", err)
	}
}

func TestHLSArgumentsCreateSegmentsFromSyntheticLocalMedia(t *testing.T) {
	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("FFmpeg not installed")
	}
	root := t.TempDir()
	source := filepath.Join(root, "source.mkv")
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	encode := exec.CommandContext(ctx, ffmpeg,
		"-hide_banner", "-loglevel", "error",
		"-f", "lavfi", "-i", "color=c=blue:s=320x180:r=12",
		"-f", "lavfi", "-i", "sine=frequency=440:sample_rate=44100",
		"-t", "1", "-threads", "1", "-c:v", "mpeg4", "-c:a", "aac", "-y", source)
	if err := encode.Run(); err != nil {
		t.Fatalf("failed to generate local source: %v", err)
	}
	spec := testHLSSpec(root, Plan{Mode: TranscodeVideo, CopyVideo: false, CopyAudio: true})
	spec.AudioIndex = 1
	args, err := HLSArguments(spec)
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.CommandContext(ctx, ffmpeg, args...)
	if err := cmd.Run(); err != nil {
		t.Fatalf("FFmpeg failed generating HLS: %v", err)
	}
	playlist, err := os.ReadFile(spec.PlaylistPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(playlist), "#EXTM3U") || !strings.Contains(string(playlist), ".ts") {
		t.Fatalf("invalid HLS playlist: %s", playlist)
	}
	segments, err := filepath.Glob(filepath.Join(root, "chunk-*.ts"))
	if err != nil || len(segments) == 0 {
		t.Fatalf("no HLS media segments produced: %v", err)
	}
}
