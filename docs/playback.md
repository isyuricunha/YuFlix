# Playback acceptance criteria

These are **future** requirements, not claims that playback has been implemented.

1. A compatible MP4/H.264/AAC source uses Direct Play without spawning FFmpeg.
2. A container-only incompatibility uses remux rather than recoding compatible streams.
3. Audio incompatibility preserves video codec and transcodes only audio.
4. Video incompatibility transcodes only necessary tracks with CPU and memory limits.
5. Playback begins from valid early HLS segments; no full-file preconversion.
6. Seek to a distant point does not require converting the entire preceding timeline.
7. Cancel/timeout/abort terminates worker children and expires segments.
8. Source HTTP headers remain server-side, even after redirects or retry.
9. Authentication is mandatory and playback tickets are scoped and short lived.
10. Remote source resolution guards SSRF, redirects and DNS rebinding.
11. Track selection prefers pt-BR and does not discard other audio/subtitle options.
12. Playback is tested from beginning through end, including seek and restart in browser, Android, iOS and TV.
13. A degraded source does not poison healthy sources. Resource exhaustion fails gracefully.
14. Transcoding throughput and concurrent sessions are measured on the deployment hardware.
