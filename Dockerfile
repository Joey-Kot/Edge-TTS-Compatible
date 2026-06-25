# syntax=docker/dockerfile:1

FROM --platform=$BUILDPLATFORM golang:1.26.0-alpine AS build

ARG TARGETOS
ARG TARGETARCH
ARG TARGETVARIANT

WORKDIR /src
COPY go.mod go.sum* /src/
RUN --mount=type=cache,target=/go/pkg/mod go mod download
COPY cmd /src/cmd
COPY internal /src/internal

RUN --mount=type=cache,target=/root/.cache/go-build \
    set -eux; \
    goarm="${TARGETVARIANT#v}"; \
    if [ "${TARGETARCH}" != "arm" ]; then goarm=""; fi; \
    CGO_ENABLED=0 \
    GOOS="${TARGETOS}" \
    GOARCH="${TARGETARCH}" \
    GOARM="${goarm}" \
    go build -trimpath -ldflags="-s -w" -o /out/edge-tts-compatible ./cmd/server

FROM alpine:3.20

RUN apk add --no-cache ca-certificates

COPY --from=build /out/edge-tts-compatible /usr/local/bin/edge-tts-compatible
COPY docker-entrypoint.sh /usr/local/bin/docker-entrypoint.sh

USER 65532:65532
EXPOSE 8080

ENTRYPOINT ["docker-entrypoint.sh"]
