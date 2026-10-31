FROM node:24-alpine AS builder
WORKDIR /src
RUN corepack enable
COPY package.json pnpm-workspace.yaml pnpm-lock.yaml ./
COPY apps/web/package.json apps/web/package.json
RUN pnpm install --frozen-lockfile
COPY apps/web apps/web
RUN pnpm build

FROM caddy:2-alpine
# This service binds only an unprivileged high port.
# Drop the unnecessary file capability so cap_drop: ALL can be enforced.
RUN apk add --no-cache libcap && setcap -r /usr/bin/caddy && apk del libcap \
    && addgroup -S -g 10001 yuflix && adduser -S -D -H -u 10001 -G yuflix yuflix
COPY deploy/Caddyfile /etc/caddy/Caddyfile
COPY --from=builder /src/apps/web/dist /srv
USER 10001:10001
EXPOSE 62471
