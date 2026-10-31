# YuFlix

**I Watch What I Want.**

YuFlix is an independent, self-hosted streaming platform for movies, series, anime, and IPTV. Built for shared sources with independent accounts, Portuguese (Brazil) first, and a media engine that prefers Direct Play over unnecessary transcoding.

> **Early foundation.** This repository currently provides a versioned HTTP API, a web shell, and a tested playback decision model. It does not yet stream or transcode media. Do not use it as a Jellyfin replacement.

## Technology

- Go 1.27.3, net/http, chi
- React, TypeScript, Vite
- YuTheme design system
- PostgreSQL 17 (planned persistence, no database migrations or connections yet)
- FFprobe and FFmpeg (planned media worker, no active transcoding yet)
- Docker Compose or native Go binary

## Development

Prerequisites: Go 1.27.3, Node.js 24 LTS or newer, pnpm 12.

    cd services/core
    go mod download
    go test ./...
    go run ./cmd/yuflix

In a second terminal:

    pnpm install --frozen-lockfile
    pnpm dev

The Vite development proxy forwards /api to the Go server. Open the local Vite address shown in the terminal.

Production build:

    pnpm build
    cd services/core && go build -o ../../bin/yuflix ./cmd/yuflix

## Docker Compose

    docker compose up -d --build

The web interface binds only to 127.0.0.1:62471 by default. Publish through your own trusted HTTPS reverse proxy after authorization and deployment review. No external secrets or database credentials are needed for the initial foundation.

Check:

    curl -fsS http://127.0.0.1:62471/healthz
    curl -fsS http://127.0.0.1:62471/api/v1/system/info

## Principles

- Never expose private source credentials or playback URLs in logs or browser-accessible APIs.
- Remote sources require SSRF protections, DNS-rebinding defenses, authorization, and safe redirect handling before they can be enabled.
- Never claim playback is working based only on HTTP status, FFprobe output, or a short clip.
- User preferences, progress, and history will be isolated; source configuration can be shared explicitly.
- Runtime settings belong in the authenticated admin panel where possible; environment variables are reserved for bootstrap and secrets.
- Keep older media servers and source aggregators untouched until full playback tests pass.

## Architecture and API

- [Architecture and milestone boundaries](docs/architecture.md)
- [Playback acceptance criteria](docs/playback.md)
- [OpenAPI v1 contract](api/openapi.yaml)

This repository intentionally contains no infrastructure-specific hostnames, IPs, credentials, or environment dumps.

## License

YuFlix is licensed under [AGPL-3.0-only](LICENSE). See [third-party notices](THIRD_PARTY_NOTICES.md) for dependencies.
