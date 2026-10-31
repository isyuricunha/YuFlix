package media

import "testing"

func TestDecide(t *testing.T) {
	client := Capabilities{
		Containers: []string{"mp4", "fmp4", "mpegts"}, VideoCodecs: []string{"h264"},
		AudioCodecs: []string{"aac"}, CanUseHLS: true,
	}
	tests := []struct {
		name      string
		source    Probe
		client    Capabilities
		mode      Mode
		copyVideo bool
		copyAudio bool
	}{
		{"direct MP4", Probe{Container: "mp4", VideoCodec: "h264", AudioCodec: "aac", HasVideo: true}, client, DirectPlay, true, true},
		{"remux MKV", Probe{Container: "mkv", VideoCodec: "h264", AudioCodec: "aac", HasVideo: true}, client, Remux, true, true},
		{"audio only convert", Probe{Container: "mkv", VideoCodec: "h264", AudioCodec: "opus", HasVideo: true}, client, TranscodeAudio, true, false},
		{"video convert", Probe{Container: "mkv", VideoCodec: "hevc", AudioCodec: "aac", HasVideo: true}, client, TranscodeVideo, false, true},
		{"both convert", Probe{Container: "webm", VideoCodec: "vp9", AudioCodec: "opus", HasVideo: true}, client, TranscodeVideo, false, false},
		{"missing video", Probe{Container: "mp4", VideoCodec: "", HasVideo: false}, client, Unsupported, false, false},
		{"no HLS", Probe{Container: "mkv", VideoCodec: "h264", AudioCodec: "aac", HasVideo: true}, Capabilities{Containers: []string{"mp4"}, VideoCodecs: []string{"h264"}, AudioCodecs: []string{"aac"}}, Unsupported, false, false},
		{"no fallback", Probe{Container: "mkv", VideoCodec: "hevc", AudioCodec: "aac", HasVideo: true}, Capabilities{Containers: []string{"mp4", "fmp4"}, VideoCodecs: []string{"av1"}, AudioCodecs: []string{"aac"}, CanUseHLS: true}, Unsupported, false, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Decide(tt.source, tt.client)
			if got.Mode != tt.mode || got.CopyVideo != tt.copyVideo || got.CopyAudio != tt.copyAudio {
				t.Fatalf("got %+v; want mode %q copyV %v copyA %v", got, tt.mode, tt.copyVideo, tt.copyAudio)
			}
		})
	}
}
