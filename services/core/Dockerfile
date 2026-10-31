FROM golang:1.27.3-alpine AS builder
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /yuflix ./cmd/yuflix

FROM alpine:3.23
RUN apk add --no-cache ca-certificates && addgroup -S app && adduser -S -G app app
COPY --from=builder /yuflix /usr/local/bin/yuflix
USER app
EXPOSE 62470
ENV YUFLIX_HTTP_ADDR=0.0.0.0:62470
ENTRYPOINT ["/usr/local/bin/yuflix"]
