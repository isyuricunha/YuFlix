// Package media contains deterministic playback decision logic.
// It intentionally performs no network access and does not run FFmpeg.
package media

import "strings"

type Mode string

const (
	Unsupported    Mode = "unsupported"
	DirectPlay     Mode = "direct_play"
	Remux          Mode = "remux"
	TranscodeAudio Mode = "transcode_audio"
	TranscodeVideo Mode = "transcode_video"
)

type Probe struct {
	Container  string
	VideoCodec string
	AudioCodec string
	HasVideo   bool
}

type Capabilities struct {
	Containers  []string
	VideoCodecs []string
	AudioCodecs []string
	CanUseHLS   bool
}

type Plan struct {
	Mode      Mode
	Reason    string
	CopyVideo bool
	CopyAudio bool
}

func contains(values []string, target string) bool {
	for _, value := range values {
		if strings.EqualFold(strings.TrimSpace(value), strings.TrimSpace(target)) {
			return true
		}
	}
	return false
}

// Decide is a planning hint, not proof of browser/device compatibility.
// Runtime probing, stream profiles, HDR, audio channels and subtitles
// must be checked before executing any plan.
func Decide(source Probe, client Capabilities) Plan {
	if !source.HasVideo || strings.TrimSpace(source.VideoCodec) == "" {
		return Plan{Mode: Unsupported, Reason: "missing video stream"}
	}
	videoOK := contains(client.VideoCodecs, source.VideoCodec)
	audioOK := source.AudioCodec == "" || contains(client.AudioCodecs, source.AudioCodec)
	if videoOK && audioOK && contains(client.Containers, source.Container) {
		return Plan{Mode: DirectPlay, Reason: "container and codecs supported", CopyVideo: true, CopyAudio: true}
	}
	if !client.CanUseHLS {
		return Plan{Mode: Unsupported, Reason: "no compatible direct source or HLS transport"}
	}
	if !contains(client.Containers, "mpegts") && !contains(client.Containers, "fmp4") {
		return Plan{Mode: Unsupported, Reason: "no supported segment container"}
	}
	if videoOK && audioOK {
		return Plan{Mode: Remux, Reason: "container incompatible", CopyVideo: true, CopyAudio: true}
	}
	if !contains(client.AudioCodecs, "aac") && source.AudioCodec != "" {
		return Plan{Mode: Unsupported, Reason: "no supported audio fallback codec"}
	}
	if videoOK {
		return Plan{Mode: TranscodeAudio, Reason: "audio codec incompatible", CopyVideo: true}
	}
	if !contains(client.VideoCodecs, "h264") {
		return Plan{Mode: Unsupported, Reason: "no supported video fallback codec"}
	}
	return Plan{Mode: TranscodeVideo, Reason: "video codec incompatible", CopyAudio: audioOK}
}
