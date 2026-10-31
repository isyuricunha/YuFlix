# PostgreSQL 17 and HTTP / HTTPS activation

The public YuFlix page works without PostgreSQL. Login, sessions and administration are unavailable until the schema is migrated and at least one approved origin is configured. Both HTTP and HTTPS are supported, but HTTP authentication requires an explicit administrator opt-in because HTTP is not encrypted.

## Dedicated database

Create a dedicated PostgreSQL 17 database and a role with the same name, using a PostgreSQL administrator:

    CREATE ROLE yuflix LOGIN;
    CREATE DATABASE yuflix OWNER yuflix;

From psql, assign its password interactively using the psql meta-command: backslash-password followed by yuflix. Do not put the password in shell history, code, logs or screenshots. Use an isolated database; never reuse an unrelated application's database or superuser.

Prefer TLS with sslmode=verify-full and certificate validation when connecting remotely.

## Private bootstrap settings

Copy .env.example to .env.production in the deployment workspace and restrict permissions to 0600. Never commit live credentials.

    YUFLIX_DATABASE_URL=postgres://yuflix:REPLACE_ME@db.example.com:5432/yuflix?sslmode=verify-full
    YUFLIX_ALLOWED_ORIGINS=https://stream.example.com,http://192.0.2.10:62471
    YUFLIX_ALLOW_INSECURE_HTTP=false

Use a full scheme, hostname/IP and optional port for each allowed origin. Only exactly listed origins may authenticate. The legacy single HTTPS YUFLIX_PUBLIC_ORIGIN remains supported.

For a trusted local network with HTTP-only clients, set YUFLIX_ALLOW_INSECURE_HTTP=true AND include the exact local HTTP origin in YUFLIX_ALLOWED_ORIGINS. Without both, HTTP authentication stays blocked. No certificate or public domain is required for this local configuration.

Both protocols can coexist. The server issues separate, non-interchangeable session cookies for each transport, and a database session is bound to the transport under which it was created.

**HTTP security:** HTTP sends credentials, session tokens and media traffic without encryption. Any network intermediary able to observe or modify traffic can steal a session or password. SameSite, HttpOnly, passwords hashed in the database and distinct cookie names do NOT protect network traffic. HTTP authentication should be limited to trusted local networks only. For remote or internet access, use HTTPS.

The HTTPS login cookie has the __Host- prefix, Secure, HttpOnly, SameSite=Strict and Path=/. The HTTP login cookie is separate, HttpOnly, SameSite=Strict and host-only but cannot use Secure or the __Host- prefix. Session tokens are random, digests are stored in PostgreSQL, and revocation works for both transports.

For HTTPS behind a reverse proxy, configure TLS at the gateway. The proxy must overwrite untrusted X-Forwarded-Proto, preserve the original Host and keep Core unreachable from untrusted networks. The packaged Compose sets trusted forwarded protocol handling only because its Core service is on an isolated network behind Caddy. Adapt a custom reverse proxy with a trusted proxy boundary rather than forwarding arbitrary client-supplied X-Forwarded-* headers.

Percent-encode reserved URI password characters and validate database certificates when accessing PostgreSQL remotely.

## Migrations and first administrator

Only after configuring and backing up any existing YuFlix database:

    docker compose run --rm core migrate
    docker compose run --rm -it core bootstrap-admin admin
    docker compose up -d --build --wait

The bootstrap CLI reads a 12 to 128 byte password from the terminal without echo and allows creating an admin only if there are zero users. No web registration or setup endpoint is exposed.

Migrations are explicit, transactional, checksummed and guarded by a PostgreSQL advisory lock.

## Account security

Each user has an individual password protected by Argon2id. Sessions use random 256-bit verifiers; only SHA-256 digests are stored. HTTPS and HTTP session cookies are transport-specific. All expire after 12 hours; only HTTPS cookies can be marked Secure. Logout revokes the server-side session. Only administrators may create users or edit shared settings. Watch progress is keyed to user identity, but progress APIs are not exposed yet.

Preferences can be stored under catalog.default_language and playback.prefer_direct, but neither is applied to playback until the catalog and Media Engine are implemented.

An optional PostgreSQL 17 integration test is named TestIntegrationPostgres17. It requires YUFLIX_TEST_DATABASE_URL pointing to a NEW, DISPOSABLE database. Never run it against production, because it migrates schema and creates test users.

## Browser preview

The initial preview works through HTTP even without a database. Once PostgreSQL migrations and the allowed origins are configured, login can work on HTTPS, opt-in HTTP, or both. The frontend warns when entering credentials through HTTP.

HTTP login is a conscious lower-security choice. Avoid enabling it on public networks.
