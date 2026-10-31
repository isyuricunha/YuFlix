package media

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const trackFixture = `{
 "format":{"filename":"/private/keep-this-secret.mp4","format_name":"mov,mp4,m4a,3gp,3g2,mj2","duration":"127.50",
   "tags":{"description":"untrusted upstream private URL"}},
 "streams":[
 {"index":0,"codec_type":"video","codec_name":"h264","width":1920,"height":1080,
  "color_transfer":"smpte2084","disposition":{"default":1,"attached_pic":0}},
 {"index":1,"codec_type":"audio","codec_name":"aac","channels":2,
  "tags":{"language":"eng","title":"Original"},"disposition":{"default":1}},
 {"index":2,"codec_type":"audio","codec_name":"aac","channels":2,
  "tags":{"language":"pt-BR","title":"Dublado"},"disposition":{"default":0}},
 {"index":3,"codec_type":"subtitle","codec_name":"subrip",
  "tags":{"language":"por"},"disposition":{"default":0}},
 {"index":4,"codec_type":"subtitle","codec_name":"ass",
  "tags":{"language":"pob","title":"Português brasileiro"},"disposition":{"default":0}},
 {"index":5,"codec_type":"video","codec_name":"mjpeg",
  "disposition":{"attached_pic":1}}
 ]
}`

func TestReadFFprobeJSONTracksAndLanguage(t *testing.T) {
	got, err := ReadFFprobeJSON([]byte(trackFixture))
	if err != nil {
		t.Fatal(err)
	}
	if got.Container != "mp4" || got.Duration != 127.5 || len(got.Tracks) != 5 {
		t.Fatalf("parsed incorrect metadata: %+v", got)
	}
	if got.Tracks[0].ColorTransfer != "smpte2084" {
		t.Fatal("HDR transfer information missing")
	}
	audio, ok := got.PreferredAudio()
	if !ok || audio.Index != 2 || audio.Language != "pt-BR" {
		t.Fatalf("wrong audio track: %+v", audio)
	}
	subtitle, ok := got.PreferredSubtitle()
	if !ok || subtitle.Index != 4 || subtitle.Language != "pt-BR" {
		t.Fatalf("wrong subtitle track: %+v", subtitle)
	}
	if strings.Contains(strings.Join([]string{got.Container, audio.Title, subtitle.Title}, ";"), "private") {
		t.Fatal("private path leaked")
	}
}

func TestReadFFprobeJSONRejectsUnboundedAndCorruptData(t *testing.T) {
	for _, data := range [][]byte{nil, []byte("{"), []byte(`{"streams":[]}`), make([]byte, maxProbeOutput+1)} {
		if _, err := ReadFFprobeJSON(data); !errors.Is(err, ErrInvalidProbeJSON) {
			t.Fatalf("expected bounded probe rejection for %d bytes, got %v", len(data), err)
		}
	}
}

func TestPreferredTracksWithoutPortuguese(t *testing.T) {
	info := MediaTracks{Tracks: []MediaTrack{
		{Index: 1, Kind: "audio", Language: "ja", Default: true},
		{Index: 2, Kind: "audio", Language: "en"},
		{Index: 3, Kind: "subtitle", Language: "en"},
	}}
	audio, ok := info.PreferredAudio()
	if !ok || audio.Index != 1 {
		t.Fatal("default original audio not respected")
	}
	if _, ok := info.PreferredSubtitle(); ok {
		t.Fatal("subtitle should not be enabled without matching Portuguese")
	}
}

func TestFFprobeFileRejectsOutsideAndSymlinkEscape(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	secret := filepath.Join(outside, "secret.mp4")
	if err := os.WriteFile(secret, []byte("not a video"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := FFprobeFile(context.Background(), "ffprobe", root, secret); !errors.Is(err, ErrUnsafeLocalMedia) {
		t.Fatalf("outside root was not rejected: %v", err)
	}
	link := filepath.Join(root, "linked.mp4")
	if err := os.Symlink(secret, link); err != nil {
		t.Skip("symlink creation unavailable")
	}
	if _, err := FFprobeFile(context.Background(), "ffprobe", root, link); !errors.Is(err, ErrUnsafeLocalMedia) {
		t.Fatalf("symlink escape was not rejected: %v", err)
	}
	if _, err := FFprobeFile(context.Background(), "ffprobe", root, "https://example.com/media.mp4"); !errors.Is(err, ErrUnsafeLocalMedia) {
		t.Fatalf("remote URLs must never reach ffprobe: %v", err)
	}
}

func TestFFprobeFileRealLocalSyntheticMedia(t *testing.T) {
	ffprobe, err := exec.LookPath("ffprobe")
	if err != nil {
		t.Skip("ffprobe not installed in test runner")
	}
	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("ffmpeg not installed in test runner")
	}
	root := t.TempDir()
	video := filepath.Join(root, "original.mp4")
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, ffmpeg,
		"-hide_banner", "-loglevel", "error",
		"-f", "lavfi", "-i", "color=c=black:s=320x180:r=12",
		"-f", "lavfi", "-i", "sine=frequency=440:sample_rate=44100",
		"-t", "1", "-threads", "1",
		"-c:v", "mpeg4", "-c:a", "aac", "-movflags", "+faststart", "-y", video)
	if err := cmd.Run(); err != nil {
		t.Fatalf("cannot generate one-second fixture: %v", err)
	}
	got, err := FFprobeFile(ctx, ffprobe, root, video)
	if err != nil {
		t.Fatal(err)
	}
	var videoFound, audioFound bool
	for _, track := range got.Tracks {
		if track.Kind == "video" && track.Width == 320 {
			videoFound = true
		}
		if track.Kind == "audio" && track.Codec == "aac" {
			audioFound = true
		}
	}
	if got.Container != "mp4" || !videoFound || !audioFound {
		t.Fatalf("real media probe incomplete: %+v", got)
	}
}
