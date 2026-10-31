# PostgreSQL 17 and HTTPS activation

The public YuFlix page works without PostgreSQL. Login, sessions and administration are intentionally unavailable until the database schema is migrated AND a public HTTPS origin is configured.

## Dedicated database

Create a dedicated PostgreSQL 17 database and a role with the same name, using a PostgreSQL administrator:

    CREATE ROLE yuflix LOGIN;
    CREATE DATABASE yuflix OWNER yuflix;

From psql, assign its password interactively using the psql meta-command: backslash-password followed by yuflix. Do not put the password in shell history, code, logs or screenshots. Use an isolated database; never reuse an unrelated application's database or superuser.

Prefer TLS with sslmode=verify-full and certificate validation when connecting remotely.

## Private bootstrap settings

Copy .env.example to .env.production in the deployment workspace, populate it locally and restrict permissions to 0600. The file is ignored by Git.

    YUFLIX_DATABASE_URL=postgres://yuflix:REPLACE_ME@db.example.com:5432/yuflix?sslmode=verify-full
    YUFLIX_PUBLIC_ORIGIN=https://stream.example.com

Percent-encode reserved characters in URI passwords. The public origin must exactly match the HTTPS address used by the browser. The HTTPS reverse proxy must replace untrusted X-Forwarded headers, preserve the external Host and keep the Go Core isolated on the Docker network.

## Migrations and first administrator

Only after configuring and backing up any existing YuFlix database:

    docker compose run --rm core migrate
    docker compose run --rm -it core bootstrap-admin admin
    docker compose up -d --build --wait

The bootstrap CLI reads a 12 to 128 byte password from the terminal without echo and allows creating an admin only if there are zero users. No web registration or setup endpoint is exposed.

Migrations are explicit, transactional, checksummed and guarded by a PostgreSQL advisory lock.

## Account security

Each user has an individual password protected by Argon2id. Sessions use random 256-bit verifiers; only SHA-256 digests are stored. Secure, HttpOnly, SameSite=Strict host-only cookies expire after 12 hours. Logout revokes the server-side session. Only administrators may create users or edit shared settings. Watch progress is keyed to user identity, but progress APIs are not exposed yet.

Preferences can be stored under catalog.default_language and playback.prefer_direct, but neither is applied to playback until the catalog and Media Engine are implemented.

An optional PostgreSQL 17 integration test is named TestIntegrationPostgres17. It requires YUFLIX_TEST_DATABASE_URL pointing to a NEW, DISPOSABLE database. Never run it against production, because it migrates schema and creates test users.

## HTTP-only public preview

The initial preview may be accessed by HTTP. It must NOT be used for password input. Configure HTTPS and a valid origin before enabling account functions.
